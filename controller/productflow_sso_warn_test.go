package controller

import (
	"bytes"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	prepareProductFlowSSOTest(t)

	oldWriter := gin.DefaultWriter
	var buf bytes.Buffer
	gin.DefaultWriter = &buf
	t.Cleanup(func() {
		gin.DefaultWriter = oldWriter
		common.RedisEnabled = oldRedisEnabled
	})

	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, "https://image.example.com"))
	require.NoError(t, model.UpdateOption(productFlowOptionSharedSecret, "test-secret"))
	common.RedisEnabled = false

	WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup()

	output := buf.String()
	require.Contains(t, output, "WARN: productflow_sso is configured")
	require.Contains(t, output, "single-process deployments")
	require.Equal(t, 1, strings.Count(output, "WARN: productflow_sso is configured"))
}

func TestWarnIfProductFlowSSOTicketFallbackIsRiskyOnStartupSkipsSafeModes(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	prepareProductFlowSSOTest(t)

	oldWriter := gin.DefaultWriter
	var buf bytes.Buffer
	gin.DefaultWriter = &buf
	t.Cleanup(func() {
		gin.DefaultWriter = oldWriter
		common.RedisEnabled = oldRedisEnabled
	})

	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, "https://image.example.com"))
	require.NoError(t, model.UpdateOption(productFlowOptionSharedSecret, "test-secret"))
	common.RedisEnabled = true
	WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup()
	require.Empty(t, buf.String())

	buf.Reset()
	common.RedisEnabled = false
	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, ""))
	require.NoError(t, model.UpdateOption(productFlowOptionSharedSecret, "test-secret"))
	WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup()
	require.Empty(t, buf.String())
}

func TestWarnIfProductFlowSSOTicketFallbackEmitsDisabledInfo(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	prepareProductFlowSSOTest(t)

	oldWriter := gin.DefaultWriter
	var buf bytes.Buffer
	gin.DefaultWriter = &buf
	t.Cleanup(func() {
		gin.DefaultWriter = oldWriter
		common.RedisEnabled = oldRedisEnabled
	})

	// Fully configured but explicitly disabled: surface the disabled INFO so
	// operators know the 503 is by intent, not by misconfiguration.
	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, "https://image.example.com"))
	require.NoError(t, model.UpdateOption(productFlowOptionSharedSecret, "test-secret"))
	require.NoError(t, model.UpdateOption(productFlowOptionEnabled, "false"))
	common.RedisEnabled = false

	WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup()

	output := buf.String()
	require.Contains(t, output, "Atelier SSO disabled")
	require.NotContains(t, output, "WARN: productflow_sso is configured")
}

func TestWarnIfProductFlowSSOTicketFallbackStaysQuietWhenDisabledAndUnconfigured(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	prepareProductFlowSSOTest(t)

	oldWriter := gin.DefaultWriter
	var buf bytes.Buffer
	gin.DefaultWriter = &buf
	t.Cleanup(func() {
		gin.DefaultWriter = oldWriter
		common.RedisEnabled = oldRedisEnabled
	})

	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, ""))
	require.NoError(t, model.UpdateOption(productFlowOptionSharedSecret, ""))
	require.NoError(t, model.UpdateOption(productFlowOptionEnabled, "false"))
	common.RedisEnabled = false

	WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup()

	require.Empty(t, buf.String(),
		"disabled+unconfigured should stay quiet to avoid log noise")
}
