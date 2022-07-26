package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"
)

func SendProxyRequest(req *http.Request, proxy *httputil.ReverseProxy, bodyBytes []byte) (*http.Response, error) {
	// Copy and redirect request to EL endpoint
	proxyReq := req.Clone(req.Context())
	proxyReq.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	defer proxyReq.Body.Close()

	proxy.Director(proxyReq)

	resp, err := proxy.Transport.RoundTrip(proxyReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func isEngineRequest(method string) bool {
	return strings.HasPrefix(method, "engine_")
}
