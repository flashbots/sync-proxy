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
	newPayloadPath       = "engine_newPayloadV1"
	forkchoicePath       = "engine_forkchoiceUpdatedV1"
	transitionConfigPath = "engine_exchangeTransitionConfigurationV1"

	// testLog is used to log information in the test methods
	testLog = logrus.WithField("testing", true)

	from = "10.0.0.0:1234"
)

type testBackend struct {
	proxyService *ProxyService
	builders     []*mockServer
	proxies      []*mockServer
}

// newTestBackend creates a new backend, initializes mock builders and return the instance
func newTestBackend(t *testing.T, numBuilders, numProxies int, builderTimeout, proxyTimeout time.Duration) *testBackend {
	backend := testBackend{
		builders: createMockServers(t, numBuilders),
		proxies:  createMockServers(t, numProxies),
	}

	builderUrls := getURLs(t, backend.builders)

	proxyUrls := getURLs(t, backend.proxies)

	opts := ProxyServiceOpts{
		Log:            testLog,
		ListenAddr:     "localhost:12345",
		Builders:       builderUrls,
		BuilderTimeout: builderTimeout,
		Proxies:        proxyUrls,
		ProxyTimeout:   proxyTimeout,
	}
	service, err := NewProxyService(opts)
	require.NoError(t, err)

	backend.proxyService = service
	return &backend
}

func createMockServers(t *testing.T, num int) []*mockServer {
	servers := make([]*mockServer, num)
	for i := 0; i < num; i++ {
		servers[i] = newMockServer(t)
		servers[i].Response = []byte(mockNewPayloadResponseValid)
	}
	return servers
}

// get urls from the mock servers
func getURLs(t *testing.T, servers []*mockServer) []*url.URL {
	urls := make([]*url.URL, len(servers))
	for i := 0; i < len(servers); i++ {
		url, err := url.Parse(servers[i].Server.URL)
		require.NoError(t, err)
		urls[i] = url
	}
	return urls
}

func (be *testBackend) request(t *testing.T, payload []byte, from string) *httptest.ResponseRecorder {
	var req *http.Request
	var err error

	if payload == nil {
		req, err = http.NewRequest(http.MethodGet, "/", nil)
	} else {
		req, err = http.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	}

	require.NoError(t, err)
	req.RemoteAddr = from
	rr := httptest.NewRecorder()
	be.proxyService.ServeHTTP(rr, req)
	return rr
}

func TestRequests(t *testing.T) {
	t.Run("test new payload request", func(t *testing.T) {
		backend := newTestBackend(t, 2, 0, time.Second, time.Second)

		backend.builders[0].Response = []byte(mockNewPayloadResponseValid)
		backend.builders[1].Response = []byte(mockNewPayloadResponseValid)

		rr := backend.request(t, []byte(mockNewPayloadRequest), from)
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
		backend := newTestBackend(t, 2, 0, time.Second, time.Second)

		backend.builders[0].Response = []byte(mockForkchoiceResponse)
		backend.builders[1].Response = []byte(mockForkchoiceResponse)

		rr := backend.request(t, []byte(mockForkchoiceRequest), from)
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
		backend := newTestBackend(t, 2, 0, time.Second, time.Second)

		backend.builders[0].Response = []byte(mockTransitionResponse)
		backend.builders[1].Response = []byte(mockTransitionResponse)

		rr := backend.request(t, []byte(mockTransitionRequest), from)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, 1, backend.builders[0].GetRequestCount(transitionConfigPath))
		require.Equal(t, 1, backend.builders[1].GetRequestCount(transitionConfigPath))

		var resp JSONRPCResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Equal(t, rr.Body.String(), mockTransitionResponse)
	})

	t.Run("service should send request to builders as well as other proxies", func(t *testing.T) {
		backend := newTestBackend(t, 2, 2, time.Second, time.Second)

		rr := backend.request(t, []byte(mockNewPayloadRequest), from)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, 1, backend.builders[0].GetRequestCount(newPayloadPath))
		require.Equal(t, 1, backend.builders[1].GetRequestCount(newPayloadPath))
	})

	t.Run("should filter requests not from engine or builder namespace", func(t *testing.T) {
		backend := newTestBackend(t, 1, 0, time.Second, time.Second)

		rr := backend.request(t, []byte(mockEthChainIDRequest), from)
		require.Equal(t, http.StatusOK, rr.Code)

		require.Equal(t, rr.Body.String(), "")
	})

	t.Run("should filter requests not from the best synced", func(t *testing.T) {
		backend := newTestBackend(t, 2, 2, time.Second, time.Second)

		rr := backend.request(t, []byte(mockForkchoiceRequest), "localhost:8080")
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		rr = backend.request(t, []byte(mockForkchoiceRequest), from)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		require.Equal(t, 1, backend.builders[0].GetRequestCount(forkchoicePath))
		require.Equal(t, 1, backend.builders[1].GetRequestCount(forkchoicePath))
	})

	t.Run("service should not filter new payload requests from any beacon node", func(t *testing.T) {
		backend := newTestBackend(t, 2, 2, time.Second, time.Second)

		rr := backend.request(t, []byte(mockNewPayloadRequest), "localhost:8080")
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		rr = backend.request(t, []byte(mockNewPayloadRequest), from)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		require.Equal(t, 2, backend.builders[0].GetRequestCount(newPayloadPath))
		require.Equal(t, 2, backend.builders[1].GetRequestCount(newPayloadPath))
	})

	t.Run("should return status ok for GET requests", func(t *testing.T) {
		backend := newTestBackend(t, 1, 0, time.Second, time.Second)

		rr := backend.request(t, nil, from)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})
}

func TestBuilders(t *testing.T) {
	t.Run("builders have different responses should return response of first builder", func(t *testing.T) {
		backend := newTestBackend(t, 2, 0, time.Second, time.Second)

		backend.builders[0].Response = []byte(mockNewPayloadResponseSyncing)
		backend.builders[1].Response = []byte(mockNewPayloadResponseValid)

		rr := backend.request(t, []byte(mockNewPayloadRequest), from)
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
		backend := newTestBackend(t, 2, 0, time.Second, time.Second)

		backend.builders[0].Response = []byte(mockForkchoiceResponse)
		backend.builders[1].Server.Close()

		rr := backend.request(t, []byte(mockForkchoiceRequest), from)
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
		backend := newTestBackend(t, 2, 0, time.Second, time.Second)

		backend.builders[1].Response = []byte(mockNewPayloadResponseSyncing)
		backend.builders[0].Server.Close()

		rr := backend.request(t, []byte(mockNewPayloadRequest), from)
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
		backend := newTestBackend(t, 1, 0, time.Second, time.Second)

		backend.builders[0].Server.Close()

		rr := backend.request(t, []byte(mockNewPayloadRequest), from)
		require.Equal(t, http.StatusBadGateway, rr.Code, rr.Body.String())
		require.Equal(t, 0, backend.builders[0].GetRequestCount(newPayloadPath))
	})
}

func TestUpdateBestBeaconNode(t *testing.T) {
	var data JSONRPCRequest
	json.Unmarshal([]byte(mockForkchoiceRequestWithPayloadAttributesV1), &data)

	data.Params[1].(*PayloadAttributes).Timestamp = 10
	higherTimestampFcu, err := json.Marshal(data)
	require.NoError(t, err)

	json.Unmarshal([]byte(mockForkchoiceRequestWithPayloadAttributesV2), &data)

	data.Params[1].(*PayloadAttributes).Timestamp = 1
	lowerTimestampFcu, err := json.Marshal(data)
	require.NoError(t, err)

	json.Unmarshal([]byte(mockForkchoiceRequest), &data)

	t.Run("should update address to sync if sync target address is not set", func(t *testing.T) {
		backend := newTestBackend(t, 1, 0, time.Second, time.Second)

		backend.request(t, []byte(mockNewPayloadRequest), from)
		require.NotNil(t, backend.proxyService.bestBeaconEntry)
	})

	t.Run("should update address to sync if higher current timestamp is received", func(t *testing.T) {
		backend := newTestBackend(t, 1, 0, time.Second, time.Second)

		backend.request(t, lowerTimestampFcu, from)
		require.NotNil(t, backend.proxyService.bestBeaconEntry)
		require.Equal(t, backend.proxyService.bestBeaconEntry.Timestamp, uint64(1))

		backend.request(t, higherTimestampFcu, from)
		require.NotNil(t, backend.proxyService.bestBeaconEntry)
		require.Equal(t, uint64(10), backend.proxyService.bestBeaconEntry.Timestamp)
	})

	t.Run("should not update address to sync if timestamp received is not higher than previously received", func(t *testing.T) {
		backend := newTestBackend(t, 1, 0, time.Second, time.Second)

		backend.request(t, higherTimestampFcu, from)
		require.NotNil(t, backend.proxyService.bestBeaconEntry)
		require.Equal(t, backend.proxyService.bestBeaconEntry.Timestamp, uint64(10))

		backend.request(t, higherTimestampFcu, from)
		require.NotNil(t, backend.proxyService.bestBeaconEntry)
		require.Equal(t, backend.proxyService.bestBeaconEntry.Timestamp, uint64(10))
	})
}
