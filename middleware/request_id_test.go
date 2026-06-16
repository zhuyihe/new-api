package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAtelierRequestId(t *testing.T) {
	require.Equal(t, "atr_abc-123", normalizeAtelierRequestId(" atr_abc-123 "))
	require.Empty(t, normalizeAtelierRequestId(""))
	require.Empty(t, normalizeAtelierRequestId("abc/123"))
	require.Empty(t, normalizeAtelierRequestId("中文"))
	require.Empty(t, normalizeAtelierRequestId("atr_"+string(make([]byte, 65))))
}

func TestRequestIdStoresValidAtelierRequestId(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestId())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(common.AtelierRequestIdKey))
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set(common.AtelierRequestIdKey, "atr_valid-123")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "atr_valid-123", recorder.Body.String())
	require.NotEmpty(t, recorder.Header().Get(common.RequestIdKey))
}

func TestRequestIdIgnoresInvalidAtelierRequestId(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestId())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(common.AtelierRequestIdKey))
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set(common.AtelierRequestIdKey, "atr invalid")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, recorder.Body.String())
}
