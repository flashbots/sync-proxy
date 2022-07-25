package main

import (
	"bytes"
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
)

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
	numSuccessRequestsToBuilder := 0
	bodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		p.log.WithError(err).Error("failed to read request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.log.WithField("body", string(bodyBytes)).Debug("request received from beacon node")
	var mu sync.Mutex
	var responseHeader http.Header
	var responseBody io.ReadCloser
	// Call the builders
	var wg sync.WaitGroup
	for _, entry := range p.proxyEntries {
		wg.Add(1)
		go func(entry *ProxyEntry) {
			defer wg.Done()
			url := entry.BuilderURL
			proxy := entry.Proxy
			log.WithField("url", url.String()).Debug("sending request to builder")
			outreq := req.Clone(req.Context())
			outreq.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
			proxy.Director(outreq)
			resp, err := proxy.Transport.RoundTrip(outreq)
			if err != nil {
				log.WithError(err).WithField("url", url.String()).Error("error sending request to builder")
				return
			}
			defer outreq.Body.Close()
			log.WithField("url", url.String()).Debug("response received from builder")
			// write first successful response to the client
			if numSuccessRequestsToBuilder == 0 {
				responseHeader = resp.Header
				responseBody = resp.Body
			}
			mu.Lock()
			defer mu.Unlock()
			numSuccessRequestsToBuilder++
		}(entry)
	}

	// Wait for all requests to complete...
	wg.Wait()
	if numSuccessRequestsToBuilder != 0 {
		copyHeader(w.Header(), responseHeader)
		io.Copy(w, responseBody)
	} else {
		http.Error(w, errNoSuccessfulBuilderResponse.Error(), http.StatusBadGateway)
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
