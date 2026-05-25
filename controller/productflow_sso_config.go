package controller

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	productFlowDefaultTokenName       = "Atelier"
	productFlowDefaultTicketTTL       = 60
	productFlowDefaultSessionTTL      = 14 * 24 * 60 * 60
	productFlowDefaultAdminSessionTTL = 60 * 60

	productFlowOptionBaseURL         = "productflow_sso.base_url"
	productFlowOptionSharedSecret    = "productflow_sso.shared_secret"
	productFlowOptionTokenName       = "productflow_sso.token_name"
	productFlowOptionTokenGroup      = "productflow_sso.token_group"
	productFlowOptionImageModel      = "productflow_sso.image_model"
	productFlowOptionTicketTTL       = "productflow_sso.ticket_ttl_seconds"
	productFlowOptionSessionTTL      = "productflow_sso.session_ttl_seconds"
	productFlowOptionAdminSessionTTL = "productflow_sso.admin_session_ttl_seconds"
	productFlowOptionEnabled         = "productflow_sso.enabled"
)

// errSSODisabled is a sentinel returned by validateForStart when the
// administrator has explicitly turned off the SSO bridge. The caller is
// expected to distinguish this from a configuration error so that the
// operator-facing 503 message stays unambiguous.
var errSSODisabled = errors.New("Atelier SSO is disabled")

type productFlowSSOConfig struct {
	Enabled                bool
	BaseURL                string
	SharedSecret           string
	TokenName              string
	TokenGroup             string
	ImageModel             string
	TicketTTLSeconds       int
	SessionTTLSeconds      int
	AdminSessionTTLSeconds int
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
	adminSessionTTL := getProductFlowOptionInt(
		productFlowOptionAdminSessionTTL,
		productFlowDefaultAdminSessionTTL,
	)
	if adminSessionTTL <= 0 {
		adminSessionTTL = productFlowDefaultAdminSessionTTL
	}
	cfg := productFlowSSOConfig{
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
		TokenGroup: getProductFlowOptionString(
			productFlowOptionTokenGroup,
			"",
		),
		ImageModel: getProductFlowOptionString(
			productFlowOptionImageModel,
			"",
		),
		TicketTTLSeconds:       ticketTTL,
		SessionTTLSeconds:      sessionTTL,
		AdminSessionTTLSeconds: adminSessionTTL,
	}
	cfg.applySingleImageModelDefault()
	return cfg
}

func snapshotProductFlowSSOOptionValues() map[string]string {
	keys := []string{
		productFlowOptionEnabled,
		productFlowOptionBaseURL,
		productFlowOptionSharedSecret,
		productFlowOptionTokenName,
		productFlowOptionTokenGroup,
		productFlowOptionImageModel,
		productFlowOptionTicketTTL,
		productFlowOptionSessionTTL,
		productFlowOptionAdminSessionTTL,
	}
	values := make(map[string]string, len(keys))
	common.OptionMapRWMutex.RLock()
	for _, key := range keys {
		values[key] = common.OptionMap[key]
	}
	common.OptionMapRWMutex.RUnlock()
	return values
}

func validateProductFlowSSOOptionValues(values map[string]string) error {
	cfg := productFlowSSOConfig{
		Enabled:    productFlowOptionBoolFromValues(values, productFlowOptionEnabled, true),
		TokenGroup: strings.TrimSpace(values[productFlowOptionTokenGroup]),
		ImageModel: strings.TrimSpace(values[productFlowOptionImageModel]),
	}
	return cfg.validateImageModel()
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

func productFlowOptionBoolFromValues(values map[string]string, optionKey string, fallback bool) bool {
	value, ok := values[optionKey]
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
		return errors.New("Atelier base URL is not configured")
	}
	parsed, err := url.ParseRequestURI(cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("Atelier base URL is invalid: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("Atelier base URL must be an absolute URL")
	}
	if err := cfg.validateForVerify(); err != nil {
		return err
	}
	return cfg.validateImageModel()
}

func (cfg productFlowSSOConfig) validateForVerify() error {
	if cfg.SharedSecret == "" {
		return errors.New("Atelier shared secret is not configured")
	}
	return nil
}

func (cfg productFlowSSOConfig) validateImageModel() error {
	if !cfg.Enabled || strings.TrimSpace(cfg.TokenGroup) == "" {
		return nil
	}
	models, err := model.GetGroupEnabledImageModels(cfg.TokenGroup)
	if err != nil {
		return fmt.Errorf("failed to load Atelier image models for token group: %w", err)
	}
	if len(models) == 0 {
		return fmt.Errorf("Atelier token group %q has no enabled image-generation models", cfg.TokenGroup)
	}
	if strings.TrimSpace(cfg.ImageModel) == "" {
		if len(models) == 1 {
			return nil
		}
		return fmt.Errorf("Atelier image model is required for token group %q", cfg.TokenGroup)
	}
	for _, modelName := range models {
		if modelName == cfg.ImageModel {
			return nil
		}
	}
	return fmt.Errorf("Atelier image model %q is not enabled for token group %q", cfg.ImageModel, cfg.TokenGroup)
}

func (cfg *productFlowSSOConfig) applySingleImageModelDefault() {
	if cfg == nil || !cfg.Enabled {
		return
	}
	if strings.TrimSpace(cfg.ImageModel) != "" || strings.TrimSpace(cfg.TokenGroup) == "" {
		return
	}
	models, err := model.GetGroupEnabledImageModels(cfg.TokenGroup)
	if err != nil || len(models) != 1 {
		return
	}
	cfg.ImageModel = models[0]
}

func WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup() {
	cfg := getProductFlowSSOConfig()
	if !cfg.Enabled {
		// Surface a single INFO when the admin has explicitly disabled SSO but
		// the rest of the config is still in place — operators benefit from
		// knowing the toggle, not the misconfiguration, is the reason
		// /api/productflow/sso/start returns 503.
		if cfg.BaseURL != "" && cfg.SharedSecret != "" {
			common.SysLog("Atelier SSO disabled (productflow_sso.enabled=false)")
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
