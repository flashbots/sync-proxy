package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
)

func BuildProxyRequest(ctx context.Context, req *http.Request, proxy *httputil.ReverseProxy, bodyBytes []byte) *http.Request {
	// Copy and redirect request to EL endpoint. The context is detached from the
	// inbound request on purpose (client cancellation must not abort the forward)
	// but bounded by the configured timeout.
	proxyReq := req.Clone(ctx)
	appendHostToXForwardHeader(proxyReq.Header, req.URL.Host)
	proxyReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	proxy.Director(proxyReq)
	return proxyReq
}

// SendProxyRequest forwards the request to the entry's backend, bounded by the
// entry's timeout. On success the caller must close resp.Body and then call
// cancel once done reading it.
func SendProxyRequest(req *http.Request, entry *ProxyEntry, bodyBytes []byte) (*http.Response, context.CancelFunc, error) {
	ctx := context.Background()
	cancel := context.CancelFunc(func() {})
	if entry.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, entry.Timeout)
	}

	proxyReq := BuildProxyRequest(ctx, req, entry.Proxy, bodyBytes)
	resp, err := entry.Proxy.Transport.RoundTrip(proxyReq)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return resp, cancel, nil
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

func isEngineRequest(method string) bool {
	return strings.HasPrefix(method, "engine_")
}

func getRemoteHost(r *http.Request) string {
	var remoteHost string
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		remoteHost = xff
	} else {
		splitAddr := strings.Split(r.RemoteAddr, ":")
		if len(splitAddr) > 0 {
			remoteHost = splitAddr[0]
		}
	}
	return remoteHost
}

func extractStatus(method string, response []byte) (string, error) {
	var responseJSON JSONRPCResponse

	switch {
	case strings.HasPrefix(method, newPayload):
		responseJSON.Result = new(PayloadStatusV1)
	case strings.HasPrefix(method, fcU):
		responseJSON.Result = new(ForkChoiceResponse)
	default:
		return "", nil // not interested in other engine api calls
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

func getResponseBody(response BuilderResponse) []byte {
	if len(response.UncompressedBody) != 0 {
		return response.UncompressedBody
	}
	return response.Body
}
