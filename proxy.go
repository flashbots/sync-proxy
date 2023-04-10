package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

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
	Timestamp uint64
}

// ProxyServiceOpts contains options for the ProxyService
type ProxyServiceOpts struct {
	ListenAddr        string
	Builders          []*url.URL
	BuilderTimeout    time.Duration
	Proxies           []*url.URL
	ProxyTimeout      time.Duration
	BeaconEntryExpiry time.Duration
	Log               *logrus.Entry
}

// ProxyService is a service that proxies requests from beacon node to builders
type ProxyService struct {
	listenAddr       string
	srv              *http.Server
	builderEntries   []*ProxyEntry
	proxyEntries     []*ProxyEntry
	bestBeaconEntry  *BeaconEntry
	beaconExpiryTime time.Duration

	log   *logrus.Entry
	timer *time.Timer
	mu    sync.Mutex
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
		listenAddr:       opts.ListenAddr,
		builderEntries:   builderEntries,
		proxyEntries:     proxyEntries,
		log:              opts.Log,
		timer:            time.NewTimer(opts.BeaconEntryExpiry),
		beaconExpiryTime: opts.BeaconEntryExpiry,
	}, nil
}

// StartHTTPServer starts the HTTP server for the proxy service
func (p *ProxyService) StartHTTPServer() error {
	if p.srv != nil {
		return errServerAlreadyRunning
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background task to make sure we don't get stuck
	// with a beacon node that stop sending requests
	go func() {
		for {
			select {
			case <-ctx.Done():
				p.timer.Stop()
				return
			case <-p.timer.C:
				p.mu.Lock()
				p.bestBeaconEntry = nil
				p.mu.Unlock()
			}
		}
	}()

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

func (p *ProxyService) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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

	remoteHost := getRemoteHost(req)
	requestJSON, err := p.checkBeaconRequest(bodyBytes, remoteHost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if p.shouldFilterRequest(remoteHost, requestJSON.Method) {
		p.log.WithField("remoteHost", remoteHost).Debug("request filtered from beacon node proxy is not synced to")
		w.WriteHeader(http.StatusOK)
		return
	}

	// return if request is cancelled or timed out
	err = req.Context().Err()
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	builderResponse, err := p.callBuilders(req, requestJSON, bodyBytes)
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

func (p *ProxyService) checkBeaconRequest(bodyBytes []byte, remoteHost string) (JSONRPCRequest, error) {
	var requestJSON JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &requestJSON); err != nil {
		p.log.WithError(err).Error("failed to decode request body json")
		return requestJSON, err
	}

	p.log.WithFields(logrus.Fields{
		"method": requestJSON.Method,
		"id":     requestJSON.ID,
	}).Debug("request received from beacon node")

	p.updateBestBeaconEntry(requestJSON, remoteHost)
	return requestJSON, nil
}

func (p *ProxyService) shouldFilterRequest(remoteHost, method string) bool {
	if !isEngineRequest(method) {
		return true
	}

	if !strings.HasPrefix(method, newPayload) && !p.isFromBestBeaconEntry(remoteHost) {
		return true
	}

	return false
}

func (p *ProxyService) isFromBestBeaconEntry(remoteHost string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bestBeaconEntry != nil && p.bestBeaconEntry.Addr == remoteHost
}

// updates for which the proxy / beacon should sync to
func (p *ProxyService) updateBestBeaconEntry(request JSONRPCRequest, requestAddr string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.bestBeaconEntry == nil {
		log.WithFields(logrus.Fields{
			"newAddr": requestAddr,
		}).Info("request received from beacon node")
		p.bestBeaconEntry = &BeaconEntry{Addr: requestAddr, Timestamp: 0}
	}

	// update to compare differences in timestamp
	var timestamp uint64
	if strings.HasPrefix(request.Method, fcU) {
		switch v := request.Params[1].(type) {
		case *PayloadAttributes:
			timestamp = v.Timestamp
		}
	} else if strings.HasPrefix(request.Method, newPayload) {
		switch v := request.Params[0].(type) {
		case *ExecutionPayload:
			timestamp = v.Timestamp
		}
	}

	if p.bestBeaconEntry.Timestamp < timestamp {
		log.WithFields(logrus.Fields{
			"oldTimestamp": p.bestBeaconEntry.Timestamp,
			"oldAddr":      p.bestBeaconEntry.Addr,
			"newTimestamp": timestamp,
			"newAddr":      requestAddr,
		}).Info(fmt.Sprintf("new timestamp from %s request received from beacon node", request.Method))
		p.bestBeaconEntry = &BeaconEntry{Timestamp: timestamp, Addr: requestAddr}
	}

	// reset expiry time for the current best entry
	if requestAddr == p.bestBeaconEntry.Addr {
		p.timer.Reset(p.beaconExpiryTime)
	}
}

func (p *ProxyService) maybeLogReponseDifferences(method string, primaryResponse BuilderResponse, responses []BuilderResponse) {
	expectedStatus, err := extractStatus(method, getResponseBody(primaryResponse))
	if err != nil {
		p.log.WithError(err).WithFields(logrus.Fields{
			"method": method,
			"url":    primaryResponse.URL.String(),
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
