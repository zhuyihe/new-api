package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProductFlowStartResolvesCallbackAgainstBasePath(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	router := productFlowSSORouter()
	user := seedProductFlowUser(t, db)

	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, "https://db.example.com/admin/console/"))
	require.NoError(t, model.UpdateOption(productFlowOptionSharedSecret, "db-secret"))
	require.NoError(t, model.UpdateOption(productFlowOptionTokenName, "ProductFlow DB"))

	cookies := loginProductFlowSession(t, router, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	redirectURL := recorder.Header().Get("Location")
	require.True(t,
		strings.HasPrefix(redirectURL, "https://db.example.com/auth/new-api/callback?ticket="),
	)
	require.NotContains(t, redirectURL, "/admin/console/auth/new-api/callback")
	require.NotContains(t, redirectURL, "sk-")
}

func TestProductFlowStatusUsesCanonicalCallbackPreviewAndLastResult(t *testing.T) {
	prepareProductFlowSSOTest(t)

	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, "https://image.example.com/admin/console/"))
	require.NoError(t, model.UpdateOption(productFlowOptionSharedSecret, "test-secret"))

	result := productFlowTestResult{
		OK:            true,
		Category:      "connected",
		LatencyMs:     87,
		TestedAgainst: "draft",
		TestedAt:      1710000000,
		Message:       "Atelier 1.2.3",
	}
	encoded, err := common.Marshal(result)
	require.NoError(t, err)
	require.NoError(t, model.UpdateOption(productFlowOptionLastTestResult, string(encoded)))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/productflow/sso/status", nil)

	GetProductFlowSSOStatus(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool                         `json:"success"`
		Data    ProductFlowSSOStatusResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "https://image.example.com/auth/new-api/callback", response.Data.CallbackURLPreview)
	require.NotNil(t, response.Data.LastTestResult)
	require.True(t, response.Data.LastTestResult.OK)
	require.Equal(t, "draft", response.Data.LastTestResult.TestedAgainst)
	require.Equal(t, "Atelier 1.2.3", response.Data.LastTestResult.Message)
}
