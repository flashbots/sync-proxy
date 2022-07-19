package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

// mockBuilder is used to fake a builder's behavior.
type mockBuilder struct {
	// Used to panic if impossible error happens
	t *testing.T

	ProxyEntry ProxyEntry

	// Used to count each engine made to the builder, either if it fails or not, for each method
	mu           sync.Mutex
	requestCount map[string]int

	// Responses placeholders that can be overridden
	Response []byte

	// Server section
	Server        *httptest.Server
	ResponseDelay time.Duration
}

// newMockBuilder creates a mocked builder
func newMockBuilder(t *testing.T) *mockBuilder {
	builder := &mockBuilder{t: t, requestCount: make(map[string]int)}

	// Initialize server
	builder.Server = httptest.NewServer(builder.getRouter())

	url, err := url.Parse(builder.Server.URL)
	require.NoError(t, err)
	builder.ProxyEntry = ProxyEntry{BuilderURL: url, Proxy: httputil.NewSingleHostReverseProxy(url)}
	require.NoError(t, err)

	return builder
}

// getRouter registers the backend, apply the test middleware and returns the router
func (m *mockBuilder) getRouter() http.Handler {
	// Create router.
	r := mux.NewRouter()

	// Register handlers
	r.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(m.Response)
	})).Methods(http.MethodPost)

	return m.newTestMiddleware(r)
}

// newTestMiddleware creates a middleware which increases the Request counter and creates a fake delay for the response
func (m *mockBuilder) newTestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Request counter
			m.mu.Lock()

			bodyBytes, err := ioutil.ReadAll(r.Body)
			require.NoError(m.t, err)

			var req JSONRPCRequest
			err = json.Unmarshal(bodyBytes, &req)
			require.NoError(m.t, err)
			m.requestCount[req.Method]++

			r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
			
			m.mu.Unlock()

			// Artificial Delay
			if m.ResponseDelay > 0 {
				time.Sleep(m.ResponseDelay)
			}

			next.ServeHTTP(w, r)
		},
	)
}

// GetRequestCount returns the number of requests made to an api method
func (m *mockBuilder) GetRequestCount(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requestCount[method]
}