package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)


var (
	newPayloadPath = "engine_newPayloadV1"
	forkchoicePath = "engine_forkchoiceUpdatedV1"
	transitionConfigPath = "engine_exchangeTransitionConfigurationV1"

	// testLog is used to log information in the test methods
	testLog = logrus.WithField("testing", true)
)

type testBackend struct {
	proxy    *ProxyService
	builders []*mockBuilder
}

// newTestBackend creates a new backend, initializes mock builders and return the instance
func newTestBackend(t *testing.T, numBuilders int, builderTimeout time.Duration) *testBackend {
	backend := testBackend{
		builders: make([]*mockBuilder, numBuilders),
	}

	builders := make([]*url.URL, numBuilders)
	for i := 0; i < numBuilders; i++ {
		// Create a mock builder
		backend.builders[i] = newMockBuilder(t)
		backend.builders[i].Response = []byte(mockNewPayloadResponseValid)
		url, err := url.Parse(backend.builders[i].Server.URL)
		require.NoError(t, err)
		builders[i] = url
	}

	opts := ProxyServiceOpts{
		Log:            testLog,
		ListenAddr:     "localhost:12345",
		Builders:       builders,
		BuilderTimeout: builderTimeout,
	}
	service, err := NewProxyService(opts)
	require.NoError(t, err)

	backend.proxy = service
	return &backend
}

func (be *testBackend) request(t *testing.T, method string, payload []byte) *httptest.ResponseRecorder {
	var req *http.Request
	var err error

	req, err = http.NewRequest(method, "/", bytes.NewReader(payload))
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	be.proxy.ServeHTTP(rr, req)
	return rr
}

func TestRequests(t *testing.T) {
	t.Run("test new payload request", func(t *testing.T) {
		backend := newTestBackend(t, 2, time.Second)

		backend.builders[0].Response = []byte(mockNewPayloadResponseValid)
		backend.builders[1].Response = []byte(mockNewPayloadResponseValid)

		rr := backend.request(t, http.MethodPost, []byte(mockNewPayloadRequest))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, 1, backend.builders[0].GetRequestCount(newPayloadPath))
		require.Equal(t, 1, backend.builders[1].GetRequestCount(newPayloadPath))

		var resp JSONRPCResponse
		resp.Result = new(PayloadStatusV1)
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Equal(t, rr.Body.String(), mockNewPayloadResponseValid)
	})

	t.Run("test forkchoice updated request", func(t *testing.T) {
		backend := newTestBackend(t, 2, time.Second)

		backend.builders[0].Response = []byte(mockForkchoiceResponse)
		backend.builders[1].Response = []byte(mockForkchoiceResponse)

		rr := backend.request(t, http.MethodPost, []byte(mockForkchoiceRequest))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, 1, backend.builders[0].GetRequestCount(forkchoicePath))
		require.Equal(t, 1, backend.builders[1].GetRequestCount(forkchoicePath))

		var resp JSONRPCResponse
		resp.Result = new(ForkChoiceResponse)
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Equal(t, rr.Body.String(), mockForkchoiceResponse)
	})

	t.Run("test engine request", func(t *testing.T) {
		backend := newTestBackend(t, 2, time.Second)

		backend.builders[0].Response = []byte(mockTransitionResponse)
		backend.builders[1].Response = []byte(mockTransitionResponse)

		rr := backend.request(t, http.MethodPost, []byte(mockTransitionRequest))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, 1, backend.builders[0].GetRequestCount(transitionConfigPath))
		require.Equal(t, 1, backend.builders[1].GetRequestCount(transitionConfigPath))

		var resp JSONRPCResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Equal(t, rr.Body.String(), mockTransitionResponse)
	})
}

func TestBuilders(t *testing.T) {
	t.Run("builders have different responses should return response of first builder", func(t *testing.T) {
		backend := newTestBackend(t, 2, time.Second)

		backend.builders[0].Response = []byte(mockNewPayloadResponseSyncing)
		backend.builders[1].Response = []byte(mockNewPayloadResponseValid)

		rr := backend.request(t, http.MethodPost, []byte(mockNewPayloadRequest))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, 1, backend.builders[0].GetRequestCount(newPayloadPath))
		require.Equal(t, 1, backend.builders[1].GetRequestCount(newPayloadPath))

		var resp JSONRPCResponse
		resp.Result = new(PayloadStatusV1)
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Equal(t, rr.Body.String(), mockNewPayloadResponseSyncing)
	})

	t.Run("only first builder online should return response of first builder", func(t *testing.T) {
		backend := newTestBackend(t, 2, time.Second)

		backend.builders[0].Response = []byte(mockForkchoiceResponse)
		backend.builders[1].Server.Close()

		rr := backend.request(t, http.MethodPost, []byte(mockForkchoiceRequest))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, 1, backend.builders[0].GetRequestCount(forkchoicePath))
		require.Equal(t, 0, backend.builders[1].GetRequestCount(forkchoicePath))

		var resp JSONRPCResponse
		resp.Result = new(ForkChoiceResponse)
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Equal(t, rr.Body.String(), mockForkchoiceResponse)
	})

	t.Run("if first builder is offline proxy should fallback to another builder", func(t *testing.T) {
		backend := newTestBackend(t, 2, time.Second)

		backend.builders[1].Response = []byte(mockNewPayloadResponseSyncing)
		backend.builders[0].Server.Close()

		rr := backend.request(t, http.MethodPost, []byte(mockNewPayloadRequest))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, 0, backend.builders[0].GetRequestCount(newPayloadPath))
		require.Equal(t, 1, backend.builders[1].GetRequestCount(newPayloadPath))

		var resp JSONRPCResponse
		resp.Result = new(PayloadStatusV1)
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Equal(t, rr.Body.String(), mockNewPayloadResponseSyncing)
	})

	t.Run("all builders are down", func(t *testing.T) {
		backend := newTestBackend(t, 1, time.Second)

		backend.builders[0].Server.Close()

		rr := backend.request(t, http.MethodPost, []byte(mockNewPayloadRequest))
		require.Equal(t, http.StatusBadGateway, rr.Code, rr.Body.String())
		require.Equal(t, 0, backend.builders[0].GetRequestCount(newPayloadPath))
	})
}
