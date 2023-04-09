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
		builder := newMockServer(t)

		builder.Response = []byte(mockNewPayloadResponseValid)

		req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(mockNewPayloadRequest)))
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		builder.getRouter().ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		require.Equal(t, 1, builder.requestCount["engine_newPayloadV1"])
	})
}
