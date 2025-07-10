package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

var (
	errServerAlreadyRunning        = errors.New("server already running")
	errNoBuilders                  = errors.New("no builders specified")
	errNoSuccessfulBuilderResponse = errors.New("no successful builder response")

	newPayload = "engine_newPayload"
	fcU        = "engine_forkchoiceUpdated"
)

type BuilderResponse struct {
	Header           http.Header
	Body             []byte
	UncompressedBody []byte
	URL              *url.URL
	StatusCode       int
}

// ProxyEntry is an entry consisting of a URL and a proxy
type ProxyEntry struct {
	URL   *url.URL
	Proxy *httputil.ReverseProxy
}

// BeaconEntry consists of a URL from a beacon client and latest timestamp recorded
type BeaconEntry struct {
	Addr      string
	UpdatedAt uint64
}

// ProxyServiceOpts contains options for the ProxyService
type ProxyServiceOpts struct {
	ListenAddr     string
	Builders       []*url.URL
	BuilderTimeout time.Duration
	Proxies        []*url.URL
	ProxyTimeout   time.Duration
	Log            *logrus.Entry
}

// ProxyService is a service that proxies requests from beacon node to builders
type ProxyService struct {
	listenAddr     string
	srv            *http.Server
	builderEntries []*ProxyEntry
	proxyEntries   []*ProxyEntry

	stateManager *StateManager

	log *logrus.Entry
	mu  sync.Mutex
}

// NewProxyService creates a new ProxyService
func NewProxyService(opts ProxyServiceOpts) (*ProxyService, error) {
	if len(opts.Builders) == 0 {
		return nil, errNoBuilders
	}

	var builderEntries []*ProxyEntry
	for _, builder := range opts.Builders {
		entry := buildProxyEntry(builder, opts.BuilderTimeout)
		builderEntries = append(builderEntries, &entry)
	}

	var proxyEntries []*ProxyEntry
	for _, proxy := range opts.Proxies {
		entry := buildProxyEntry(proxy, opts.ProxyTimeout)
		proxyEntries = append(proxyEntries, &entry)
	}

	return &ProxyService{
		listenAddr:     opts.ListenAddr,
		builderEntries: builderEntries,
		proxyEntries:   proxyEntries,
		log:            opts.Log,
	}, nil
}

// StartHTTPServer starts the HTTP server for the proxy service
func (p *ProxyService) StartHTTPServer() error {
	if p.srv != nil {
		return errServerAlreadyRunning
	}

	p.srv = &http.Server{
		Addr:    p.listenAddr,
		Handler: http.HandlerFunc(p.ServeHTTP),
	}

	err := p.srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func getLogFieldsFromRequest(req *http.Request) logrus.Fields {
	logFields := logrus.Fields{"remoteHost": getRemoteHost(req)}

	token := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	jwtID := getIDClaim(token)
	if jwtID != "" {
		logFields["clID"] = jwtID
	}

	return logFields
}

// getIDClaim extracts the "id" claim from JWT token
// Ignores signatures and does not validate the token.
// Returns empty string if no token, invalid token, or no "id" claim
func getIDClaim(tokenStr string) string {
	token, _, err := jwt.NewParser().ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return ""
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if id, exists := claims["id"]; exists {
			if idStr, ok := id.(string); ok {
				return idStr
			}
		}
	}

	return ""
}

func (p *ProxyService) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// return OK for all GET requests, used for debug
	if req.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		return
	}

	bodyBytes, err := io.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		p.log.WithError(err).Error("failed to read request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log := p.log.WithFields(getLogFieldsFromRequest(req))
	remoteHost := getRemoteHost(req)
	requestJSON, err := p.getBeaconRequest(log, bodyBytes)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// initialize manager only when Forkchoice_update arrived
	if p.stateManager == nil && (requestJSON.Params.SlotStage == FCUOpen || requestJSON.Params.SlotStage == FCUClose) {
		p.stateManager = InitializeStateManager(requestJSON.Params.SlotStage, &BeaconEntry{
			Addr:      remoteHost,
			UpdatedAt: uint64(time.Now().Unix()),
		})
		log.WithFields(logrus.Fields{
			"updated_at": p.stateManager.entry.UpdatedAt,
		}).Infoln("State manager initialized")
	}

	meta, _ := json.Marshal(requestJSON.Params)

	if p.stateManager != nil {
		log.Infoln("Current state manager stage:", p.stateManager.stage.String(), "selected host", remoteHost)
	}
	if p.shouldFilterRequest(remoteHost, requestJSON.Method, requestJSON.Params) {
		log.Debug("request filtered from beacon node proxy is not synced to")
		log.WithFields(logrus.Fields{
			"id":            requestJSON.ID,
			"method":        requestJSON.Method,
			"request_stage": requestJSON.Params.SlotStage.String(),
			"host":          remoteHost,
			"meta":          string(meta)}).Infoln("Filter request")
		w.WriteHeader(http.StatusOK)
		return
	}

	log.WithFields(logrus.Fields{
		"id":            requestJSON.ID,
		"method":        requestJSON.Method,
		"request_stage": requestJSON.Params.SlotStage.String(),
		"host":          remoteHost,
		"meta":          string(meta),
	}).Infoln("Forwarding request")

	// return if request is cancelled or timed out
	err = req.Context().Err()
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	now := time.Now()
	builderResponse, err := p.callBuilders(req, requestJSON, bodyBytes)
	p.log.Infoln("Call builders request latency_ms", time.Since(now).Milliseconds())
	p.callProxies(req, bodyBytes)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	copyHeader(w.Header(), builderResponse.Header)
	w.WriteHeader(builderResponse.StatusCode)
	io.Copy(w, io.NopCloser(bytes.NewBuffer(builderResponse.Body)))
}

func (p *ProxyService) callBuilders(req *http.Request, requestJSON JSONRPCRequest, bodyBytes []byte) (BuilderResponse, error) {
	numSuccessRequestsToBuilder := 0
	var mu sync.Mutex

	var responses []BuilderResponse
	var primaryReponse BuilderResponse

	// Call the builders
	var wg sync.WaitGroup
	for _, entry := range p.builderEntries {
		wg.Add(1)
		go func(entry *ProxyEntry) {
			defer wg.Done()
			url := entry.URL
			proxy := entry.Proxy
			resp, err := SendProxyRequest(req, proxy, bodyBytes)
			if err != nil {
				log.WithError(err).WithField("url", url.String()).Error("error sending request to builder")
				return
			}

			reader := resp.Body
			responseBytes, err := io.ReadAll(reader)
			if err != nil {
				p.log.WithError(err).Error("failed to read response body")
				return
			}
			defer resp.Body.Close()

			var uncompressedResponseBytes []byte
			if !resp.Uncompressed && resp.Header.Get("Content-Encoding") == "gzip" {
				reader, err = gzip.NewReader(io.NopCloser(bytes.NewBuffer(responseBytes)))
				if err != nil {
					p.log.WithError(err).Error("failed to decompress response body")
					return
				}
				uncompressedResponseBytes, err = io.ReadAll(reader)
				if err != nil {
					p.log.WithError(err).Error("failed to read decompressed response body")
					return
				}
			}

			mu.Lock()
			defer mu.Unlock()

			builderResponse := BuilderResponse{Header: resp.Header, Body: responseBytes, UncompressedBody: uncompressedResponseBytes, URL: url, StatusCode: resp.StatusCode}
			responses = append(responses, builderResponse)

			p.log.WithFields(logrus.Fields{
				"method":   requestJSON.Method,
				"id":       requestJSON.ID,
				"response": string(getResponseBody(builderResponse)),
				"url":      url.String(),
			}).Debug("response received from builder")

			// Use response from first EL endpoint specificed and fallback if response not found
			if numSuccessRequestsToBuilder == 0 {
				primaryReponse = builderResponse
			}
			if url.String() == p.builderEntries[0].URL.String() {
				primaryReponse = builderResponse
			}

			numSuccessRequestsToBuilder++
		}(entry)
	}

	// Wait for all requests to complete...
	wg.Wait()

	if numSuccessRequestsToBuilder == 0 {
		return primaryReponse, errNoSuccessfulBuilderResponse
	}

	if isEngineRequest(requestJSON.Method) {
		p.maybeLogReponseDifferences(requestJSON.Method, primaryReponse, responses)
	}

	return primaryReponse, nil
}

func (p *ProxyService) callProxies(req *http.Request, bodyBytes []byte) {
	// call other proxies to forward requests from other beacon nodes
	for _, entry := range p.proxyEntries {
		go func(entry *ProxyEntry) {
			_, err := SendProxyRequest(req, entry.Proxy, bodyBytes)
			if err != nil {
				log.WithError(err).WithField("url", entry.URL.String()).Error("error sending request to proxy")
				return
			}
		}(entry)
	}
}

func (p *ProxyService) getBeaconRequest(log *logrus.Entry, bodyBytes []byte) (JSONRPCRequest, error) {
	var requestJSON JSONRPCRequest
	var batchRequestJSON []JSONRPCRequest
	err := json.Unmarshal(bodyBytes, &requestJSON)

	if err != nil {
		log.WithError(err).Warn("failed to decode request body json, trying to decode as batch request")
		// may be batch request
		if err := json.Unmarshal(bodyBytes, &batchRequestJSON); err != nil {
			log.WithError(err).Error("failed to decode request body json as batch request")
			return requestJSON, err
		}
		// not interested in batch requests
		return requestJSON, nil
	}

	log.WithFields(logrus.Fields{
		"method": requestJSON.Method,
		"id":     requestJSON.ID,
	}).Debug("request received from beacon node")

	return requestJSON, nil
}

func (p *ProxyService) shouldFilterRequest(remoteHost, method string, params PayloadParams) bool {
	if !isEngineRequest(method) {
		return true
	}

	// safetely forward all other requests from any CL
	if !strings.Contains(method, fcU) {
		return false
	}

	stage := p.stateManager.CurrentSlotStage()
	entry := p.stateManager.Entry()

	// accept host which arrives first and forward
	if stage == FCUOpen && params.SlotStage == FCUOpen {
		p.stateManager.NextSlotStage()

		prevHost := entry.Addr
		entry.Addr = remoteHost
		entry.UpdatedAt = uint64(time.Now().Unix())

		p.log.WithFields(logrus.Fields{
			"updated_at": entry.UpdatedAt,
			"prev_host":  prevHost,
			"host":       entry.Addr,
		}).Infoln("Update CL entry. Forwarding Forkchoice_update: open_stage")

		return false
	}

	// we should wait the same host entry, which was first before, for the forkchoice_update with payloadAttributes
	if stage == FCUClose && params.SlotStage == FCUClose {
		// don't forward for different CL
		if entry.Addr != remoteHost {
			return true
		}

		// forward request and start from forkchoice_update open_stage
		p.stateManager.NextSlotStage()
		entry.UpdatedAt = uint64(time.Now().Unix())

		p.log.WithFields(logrus.Fields{
			"updated_at": entry.UpdatedAt,
			"host":       entry.Addr,
		}).Infoln("Forwarding Forkchoice_update: close_stage")

		return false
	}

	return true
}

func (p *ProxyService) maybeLogReponseDifferences(method string, primaryResponse BuilderResponse, responses []BuilderResponse) {
	expectedStatus, err := extractStatus(method, getResponseBody(primaryResponse))
	if err != nil {
		p.log.WithError(err).WithFields(logrus.Fields{
			"method": method,
			"url":    primaryResponse.URL.String(),
			"resp":   string(getResponseBody(primaryResponse)),
		}).Error("error reading status from primary EL response")
	}

	if expectedStatus == "" {
		return
	}

	for _, response := range responses {
		if response.URL.String() == primaryResponse.URL.String() {
			continue
		}

		status, err := extractStatus(method, getResponseBody(response))
		if err != nil {
			p.log.WithError(err).WithFields(logrus.Fields{
				"method": method,
				"url":    primaryResponse.URL.String(),
				"resp":   string(getResponseBody(response)),
			}).Error("error reading status from EL response")
		}

		if status != expectedStatus {
			p.log.WithFields(logrus.Fields{
				"primaryStatus":   expectedStatus,
				"secondaryStatus": status,
				"primaryUrl":      primaryResponse.URL.String(),
				"secondaryUrl":    response.URL.String(),
			}).Info("found difference in EL responses")
		}
	}
}

func buildProxyEntry(proxyURL *url.URL, timeout time.Duration) ProxyEntry {
	proxy := httputil.NewSingleHostReverseProxy(proxyURL)
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: timeout,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return ProxyEntry{Proxy: proxy, URL: proxyURL}
}
