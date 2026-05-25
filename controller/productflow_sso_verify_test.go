package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProductFlowVerifyRejectsInvalidSecret(t *testing.T) {
	prepareProductFlowSSOTest(t)
	router := productFlowSSORouter()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/productflow/sso/verify", bytes.NewBufferString(`{"ticket":"missing"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer wrong-secret")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "invalid shared secret")
}

func TestProductFlowVerifyRequiresBearerAuthorization(t *testing.T) {
	prepareProductFlowSSOTest(t)
	router := productFlowSSORouter()

	tests := []struct {
		name       string
		auth       string
		fallback   string
		wantStatus int
	}{
		{name: "missing authorization", wantStatus: http.StatusUnauthorized},
		{name: "raw secret", auth: "test-secret", wantStatus: http.StatusUnauthorized},
		{name: "fallback header", fallback: "test-secret", wantStatus: http.StatusUnauthorized},
		{name: "valid bearer", auth: "Bearer test-secret", wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/productflow/sso/verify", bytes.NewBufferString(`{"ticket":"missing"}`))
			request.Header.Set("Content-Type", "application/json")
			if tt.auth != "" {
				request.Header.Set("Authorization", tt.auth)
			}
			if tt.fallback != "" {
				request.Header.Set("X-ProductFlow-SSO-Secret", tt.fallback)
			}
			router.ServeHTTP(recorder, request)

			require.Equal(t, tt.wantStatus, recorder.Code)
			if tt.auth != "Bearer test-secret" {
				require.Contains(t, recorder.Body.String(), "invalid shared secret")
			}
		})
	}
}

func TestProductFlowTicketCanOnlyBeVerifiedOnce(t *testing.T) {
	prepareProductFlowSSOTest(t)
	router := productFlowSSORouter()

	claims := productFlowTicketClaims{
		UserID:           "7",
		Username:         "alice",
		Token:            "sk-test-token",
		TokenID:          "9",
		TokenName:        "Atelier",
		ExpiresInSeconds: 3600,
	}
	require.NoError(t, storeProductFlowTicket("ticket-1", claims, time.Minute))

	body := []byte(`{"ticket":"ticket-1"}`)
	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/api/productflow/sso/verify", bytes.NewReader(body))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("Authorization", "Bearer test-secret")
	router.ServeHTTP(first, firstReq)

	require.Equal(t, http.StatusOK, first.Code)
	firstResponse := decodeProductFlowResponse(t, first)
	require.True(t, firstResponse.Success)
	require.Equal(t, claims.UserID, firstResponse.Data.UserID)
	require.Equal(t, claims.Token, firstResponse.Data.Token)

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/api/productflow/sso/verify", bytes.NewReader(body))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("Authorization", "Bearer test-secret")
	router.ServeHTTP(second, secondReq)

	require.Equal(t, http.StatusUnauthorized, second.Code)
	require.Contains(t, second.Body.String(), "ticket is invalid or expired")
}
