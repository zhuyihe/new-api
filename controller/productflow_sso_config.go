package controller

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	productFlowDefaultTokenName  = "ProductFlow"
	productFlowDefaultTicketTTL  = 60
	productFlowDefaultSessionTTL = 14 * 24 * 60 * 60

	productFlowOptionBaseURL          = "productflow_sso.base_url"
	productFlowOptionSharedSecret     = "productflow_sso.shared_secret"
	productFlowOptionTokenName        = "productflow_sso.token_name"
	productFlowOptionTokenModelLimits = "productflow_sso.token_model_limits"
	productFlowOptionTokenGroup       = "productflow_sso.token_group"
	productFlowOptionTicketTTL        = "productflow_sso.ticket_ttl_seconds"
	productFlowOptionSessionTTL       = "productflow_sso.session_ttl_seconds"
	productFlowOptionEnabled          = "productflow_sso.enabled"
)

// errSSODisabled is a sentinel returned by validateForStart when the
// administrator has explicitly turned off the SSO bridge. The caller is
// expected to distinguish this from a configuration error so that the
// operator-facing 503 message stays unambiguous.
var errSSODisabled = errors.New("ProductFlow SSO is disabled")

type productFlowSSOConfig struct {
	Enabled           bool
	BaseURL           string
	SharedSecret      string
	TokenName         string
	TokenModelLimits  string
	TokenGroup        string
	TicketTTLSeconds  int
	SessionTTLSeconds int
}

func getProductFlowSSOConfig() productFlowSSOConfig {
	tokenName := getProductFlowOptionString(
		productFlowOptionTokenName,
		productFlowDefaultTokenName,
	)
	if tokenName == "" {
		tokenName = productFlowDefaultTokenName
	}
	ticketTTL := getProductFlowOptionInt(
		productFlowOptionTicketTTL,
		productFlowDefaultTicketTTL,
	)
	if ticketTTL <= 0 {
		ticketTTL = productFlowDefaultTicketTTL
	}
	sessionTTL := getProductFlowOptionInt(
		productFlowOptionSessionTTL,
		productFlowDefaultSessionTTL,
	)
	if sessionTTL <= 0 {
		sessionTTL = productFlowDefaultSessionTTL
	}
	return productFlowSSOConfig{
		Enabled: getProductFlowOptionBool(productFlowOptionEnabled, true),
		BaseURL: getProductFlowOptionString(
			productFlowOptionBaseURL,
			"",
		),
		SharedSecret: getProductFlowOptionString(
			productFlowOptionSharedSecret,
			"",
		),
		TokenName: tokenName,
		TokenModelLimits: normalizeCSV(getProductFlowOptionString(
			productFlowOptionTokenModelLimits,
			"",
		)),
		TokenGroup: getProductFlowOptionString(
			productFlowOptionTokenGroup,
			"",
		),
		TicketTTLSeconds:  ticketTTL,
		SessionTTLSeconds: sessionTTL,
	}
}

func getProductFlowOptionString(optionKey string, fallback string) string {
	common.OptionMapRWMutex.RLock()
	value, ok := common.OptionMap[optionKey]
	common.OptionMapRWMutex.RUnlock()
	if ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}

func getProductFlowOptionInt(optionKey string, fallback int) int {
	common.OptionMapRWMutex.RLock()
	value, ok := common.OptionMap[optionKey]
	common.OptionMapRWMutex.RUnlock()
	if ok {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fallback
		}
		parsed, err := strconv.Atoi(trimmed)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func getProductFlowOptionBool(optionKey string, fallback bool) bool {
	common.OptionMapRWMutex.RLock()
	value, ok := common.OptionMap[optionKey]
	common.OptionMapRWMutex.RUnlock()
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

// isProductFlowBaseURLValid mirrors validateForStart's URL parse so the status
// endpoint and any other "would this configuration survive a real redirect?"
// check can stay in sync. A saved-but-malformed base URL must not be reported
// as `configured` (bug: stale dashboard kept showing "connected" after the
// admin pasted a typo).
func isProductFlowBaseURLValid(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != ""
}

func (cfg productFlowSSOConfig) validateForStart() error {
	if !cfg.Enabled {
		return errSSODisabled
	}
	if cfg.BaseURL == "" {
		return errors.New("ProductFlow base URL is not configured")
	}
	parsed, err := url.ParseRequestURI(cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("ProductFlow base URL is invalid: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("ProductFlow base URL must be an absolute URL")
	}
	return cfg.validateForVerify()
}

func (cfg productFlowSSOConfig) validateForVerify() error {
	if cfg.SharedSecret == "" {
		return errors.New("ProductFlow shared secret is not configured")
	}
	return nil
}

func WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup() {
	cfg := getProductFlowSSOConfig()
	if !cfg.Enabled {
		// Surface a single INFO when the admin has explicitly disabled SSO but
		// the rest of the config is still in place — operators benefit from
		// knowing the toggle, not the misconfiguration, is the reason
		// /api/productflow/sso/start returns 503.
		if cfg.BaseURL != "" && cfg.SharedSecret != "" {
			common.SysLog("ProductFlow SSO disabled (productflow_sso.enabled=false)")
		}
		return
	}
	if common.RedisEnabled {
		return
	}
	if err := cfg.validateForStart(); err != nil {
		return
	}
	common.SysLog(
		"WARN: productflow_sso is configured, but Redis is disabled; " +
			"ticket storage falls back to in-process memory and only " +
			"supports single-process deployments. See " +
			".trellis/spec/backend/productflow-sso.md#8-deployment-modes.",
	)
}

func normalizeCSV(value string) string {
	parts := strings.Split(value, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			normalized = append(normalized, item)
		}
	}
	return strings.Join(normalized, ",")
}
