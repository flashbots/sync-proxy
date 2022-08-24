package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
)

func BuildProxyRequest(req *http.Request, proxy *httputil.ReverseProxy, bodyBytes []byte) *http.Request {
	// Copy and redirect request to EL endpoint
	proxyReq := req.Clone(req.Context())
	appendHostToXForwardHeader(proxyReq.Header, req.URL.Host)
	proxyReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	proxy.Director(proxyReq)
	return proxyReq
}
func SendProxyRequest(req *http.Request, proxy *httputil.ReverseProxy, bodyBytes []byte) (*http.Response, error) {
	proxyReq := BuildProxyRequest(req, proxy, bodyBytes)
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

func appendHostToXForwardHeader(header http.Header, host string) {
	// X-Forwarded-For information to indicate it is forwarded from a BN
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

// func isRequestFromProxy(req *http.Request) bool {
// 	// check if request is from a proxy url
// 	forwardedFrom := req.Header["X-Forwarded-For"]
// 	return len(forwardedFrom) != 0
// }

func isEngineRequest(method string) bool {
	return strings.HasPrefix(method, "engine_")
}
