package controller

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	productFlowDefaultTokenName  = "ProductFlow"
	productFlowDefaultTicketTTL  = 60
	productFlowDefaultSessionTTL = 14 * 24 * 60 * 60
)

type productFlowSSOConfig struct {
	BaseURL           string
	SharedSecret      string
	TokenName         string
	TokenModelLimits  string
	TokenGroup        string
	TicketTTLSeconds  int
	SessionTTLSeconds int
}

func getProductFlowSSOConfig() productFlowSSOConfig {
	tokenName := strings.TrimSpace(common.GetEnvOrDefaultString("PRODUCTFLOW_TOKEN_NAME", productFlowDefaultTokenName))
	if tokenName == "" {
		tokenName = productFlowDefaultTokenName
	}
	ticketTTL := common.GetEnvOrDefault("PRODUCTFLOW_SSO_TICKET_TTL_SECONDS", productFlowDefaultTicketTTL)
	if ticketTTL <= 0 {
		ticketTTL = productFlowDefaultTicketTTL
	}
	sessionTTL := common.GetEnvOrDefault("PRODUCTFLOW_SESSION_TTL_SECONDS", productFlowDefaultSessionTTL)
	if sessionTTL <= 0 {
		sessionTTL = productFlowDefaultSessionTTL
	}
	return productFlowSSOConfig{
		BaseURL:           strings.TrimSpace(common.GetEnvOrDefaultString("PRODUCTFLOW_BASE_URL", "")),
		SharedSecret:      strings.TrimSpace(common.GetEnvOrDefaultString("PRODUCTFLOW_SSO_SECRET", "")),
		TokenName:         tokenName,
		TokenModelLimits:  normalizeCSV(common.GetEnvOrDefaultString("PRODUCTFLOW_TOKEN_MODEL_LIMITS", "")),
		TokenGroup:        strings.TrimSpace(common.GetEnvOrDefaultString("PRODUCTFLOW_TOKEN_GROUP", "")),
		TicketTTLSeconds:  ticketTTL,
		SessionTTLSeconds: sessionTTL,
	}
}

func (cfg productFlowSSOConfig) validateForStart() error {
	if cfg.BaseURL == "" {
		return errors.New("PRODUCTFLOW_BASE_URL is not configured")
	}
	parsed, err := url.ParseRequestURI(cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("PRODUCTFLOW_BASE_URL is invalid: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("PRODUCTFLOW_BASE_URL must be an absolute URL")
	}
	return cfg.validateForVerify()
}

func (cfg productFlowSSOConfig) validateForVerify() error {
	if cfg.SharedSecret == "" {
		return errors.New("PRODUCTFLOW_SSO_SECRET is not configured")
	}
	return nil
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
