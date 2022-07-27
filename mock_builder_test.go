package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_mockBuilder(t *testing.T) {
	t.Run("test payload", func(t *testing.T) {
		builder := newMockBuilder(t)

		builder.Response = []byte(mockNewPayloadResponseValid)

		req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(`{"jsonrpc":"2.0","method":"engine_newPayload","params":["0x01"],"id":67}`)))
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		builder.getRouter().ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		require.Equal(t, builder.requestCount["engine_newPayload"], 1)
	})
}
