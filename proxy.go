package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
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
	fcU = "engine_forkchoiceUpdatedV1"
)

type ProxyResponse struct {
	Header http.Header
	Body   []byte
	URL		*url.URL
}

// ProxyEntry is an entry consisting of a builder URL and a proxy
type ProxyEntry struct {
	BuilderURL *url.URL
	Proxy      *httputil.ReverseProxy
}

// ProxyServiceOpts contains options for the ProxyService
type ProxyServiceOpts struct {
	ListenAddr     string
	Builders       []*url.URL
	BuilderTimeout time.Duration
	Log            *logrus.Entry
}

// ProxyService is a service that proxies requests from beacon node to builders
type ProxyService struct {
	listenAddr   string
	srv          *http.Server
	proxyEntries []*ProxyEntry
	log          *logrus.Entry
}

// NewProxyService creates a new ProxyService
func NewProxyService(opts ProxyServiceOpts) (*ProxyService, error) {
	if len(opts.Builders) == 0 {
		return nil, errNoBuilders
	}

	var proxyEntries []*ProxyEntry
	for _, builder := range opts.Builders {
		proxy := httputil.NewSingleHostReverseProxy(builder)
		proxy.Transport = http.DefaultTransport
		proxyEntries = append(proxyEntries, &ProxyEntry{
			BuilderURL: builder,
			Proxy:      proxy,
		})
	}

	return &ProxyService{
		listenAddr:   opts.ListenAddr,
		proxyEntries: proxyEntries,
		log:          opts.Log,
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

	var responses []ProxyResponse
	var primaryReponse ProxyResponse

	// Call the builders
	var wg sync.WaitGroup
	for _, entry := range p.proxyEntries {
		wg.Add(1)
		go func(entry *ProxyEntry) {
			defer wg.Done()
			url := entry.BuilderURL
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
			
			responses = append(responses, ProxyResponse{Header: resp.Header, Body: responseBytes, URL: url})

			p.log.WithFields(logrus.Fields{
				"method": requestJSON.Method,
				"id":     requestJSON.ID,
				"response": string(responseBytes),
				"url":    url.String(),
			}).Debug("response received from builder")

			// Use response from first EL endpoint specificed and fallback if response not found
			if (numSuccessRequestsToBuilder == 0) {
				primaryReponse = ProxyResponse{Header: resp.Header, Body: responseBytes, URL: url}
			}
			if (url.String() == p.proxyEntries[0].BuilderURL.String()) {
				primaryReponse = ProxyResponse{Header: resp.Header, Body: responseBytes, URL: url}
			}

			numSuccessRequestsToBuilder++
		}(entry)
	}

	// Wait for all requests to complete...
	wg.Wait()

	if numSuccessRequestsToBuilder != 0 {
		if isEngineRequest(requestJSON.Method) {
			p.maybeLogReponseDifferences(requestJSON.Method, primaryReponse, responses)
		}
		copyHeader(w.Header(), primaryReponse.Header)
		io.Copy(w, ioutil.NopCloser(bytes.NewBuffer(primaryReponse.Body)))
	} else {
		http.Error(w, errNoSuccessfulBuilderResponse.Error(), http.StatusBadGateway)
	}
}

func (p *ProxyService) maybeLogReponseDifferences(method string, primaryResponse ProxyResponse, responses []ProxyResponse) {
	expectedStatus, err := extractStatus(method, primaryResponse.Body)
	if err != nil {
		p.log.WithError(err).WithFields(logrus.Fields{
			"method": method,
			"url": primaryResponse.URL.String(),
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
				"url": primaryResponse.URL.String(),
			}).Error("error reading status from EL response")
		}

		if (status != expectedStatus) {
			p.log.WithFields(logrus.Fields{
				"primaryStatus": expectedStatus,
				"secondaryStatus": status,
				"primaryUrl": primaryResponse.URL.String(),
				"secondaryUrl": response.URL.String(),
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
