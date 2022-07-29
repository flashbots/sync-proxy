package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	errServerAlreadyRunning        = errors.New("server already running")
	errNoBuilders                  = errors.New("no builders specified")
	errNoSuccessfulBuilderResponse = errors.New("no successful builder response")

	newPayload = "engine_newPayloadV1"
	fcU        = "engine_forkchoiceUpdatedV1"
)

type BuilderResponse struct {
	Header     http.Header
	Body       []byte
	URL        *url.URL
	StatusCode int
}

// ProxyEntry is an entry consisting of a URL and a proxy
type ProxyEntry struct {
	URL   *url.URL
	Proxy *httputil.ReverseProxy
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
	log            *logrus.Entry
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

func (p *ProxyService) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// ignore responses forwarded from other proxies unless
	// there is an issue with the beacon node request
	if isRequestFromProxy(req) {
		p.log.Debug("request received from another proxy")
		w.WriteHeader(http.StatusOK)
		return
	}

	bodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		p.log.WithError(err).Error("failed to read request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var requestJSON JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &requestJSON); err != nil {
		p.log.WithError(err).Error("failed to decode request body json")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.log.WithFields(logrus.Fields{
		"method": requestJSON.Method,
		"id":     requestJSON.ID,
	}).Info("request received from beacon node")

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

			responseBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				p.log.WithError(err).Error("failed to read response body")
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			defer mu.Unlock()

			responses = append(responses, BuilderResponse{Header: resp.Header, Body: responseBytes, URL: url, StatusCode: resp.StatusCode})

			p.log.WithFields(logrus.Fields{
				"method":   requestJSON.Method,
				"id":       requestJSON.ID,
				"response": string(responseBytes),
				"url":      url.String(),
			}).Debug("response received from builder")

			// Use response from first EL endpoint specificed and fallback if response not found
			if numSuccessRequestsToBuilder == 0 {
				primaryReponse = BuilderResponse{Header: resp.Header, Body: responseBytes, URL: url, StatusCode: resp.StatusCode}
			}
			if url.String() == p.builderEntries[0].URL.String() {
				primaryReponse = BuilderResponse{Header: resp.Header, Body: responseBytes, URL: url, StatusCode: resp.StatusCode}
			}

			numSuccessRequestsToBuilder++
		}(entry)
	}

	// call other proxies to forward requests from other beacon nodes
	for _, entry := range p.proxyEntries {
		wg.Add(1)
		go func(entry *ProxyEntry) {
			defer wg.Done()
			_, err := SendProxyRequest(req, entry.Proxy, bodyBytes)
			if err != nil {
				log.WithError(err).WithField("url", entry.URL.String()).Error("error sending request to proxy")
				return
			}
		}(entry)
	}

	// Wait for all requests to complete...
	wg.Wait()

	if numSuccessRequestsToBuilder != 0 {
		if isEngineRequest(requestJSON.Method) {
			p.maybeLogReponseDifferences(requestJSON.Method, primaryReponse, responses)
		}
		copyHeader(w.Header(), primaryReponse.Header)
		w.WriteHeader(primaryReponse.StatusCode)
		io.Copy(w, ioutil.NopCloser(bytes.NewBuffer(primaryReponse.Body)))
	} else {
		http.Error(w, errNoSuccessfulBuilderResponse.Error(), http.StatusBadGateway)
	}
}

func (p *ProxyService) maybeLogReponseDifferences(method string, primaryResponse BuilderResponse, responses []BuilderResponse) {
	expectedStatus, err := extractStatus(method, primaryResponse.Body)
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

		status, err := extractStatus(method, response.Body)
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

func extractStatus(method string, response []byte) (string, error) {
	var responseJSON JSONRPCResponse

	switch method {
	case newPayload:
		responseJSON.Result = new(PayloadStatusV1)
	case fcU:
		responseJSON.Result = new(ForkChoiceResponse)
	default:
	}

	if err := json.Unmarshal(response, &responseJSON); err != nil {
		return "", err
	}

	switch v := responseJSON.Result.(type) {
	case *ForkChoiceResponse:
		return v.PayloadStatus.Status, nil
	case *PayloadStatusV1:
		return v.Status, nil
	default:
		return "", nil // not interested in other engine api calls
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
