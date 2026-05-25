package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type productFlowVerifyRequest struct {
	Ticket string `json:"ticket"`
}

type productFlowTicketClaims struct {
	UserID           string `json:"user_id"`
	Username         string `json:"username,omitempty"`
	Email            string `json:"email,omitempty"`
	Group            string `json:"group,omitempty"`
	Role             string `json:"role,omitempty"`
	Token            string `json:"token,omitempty"`
	TokenID          string `json:"token_id,omitempty"`
	TokenName        string `json:"token_name,omitempty"`
	ExpiresInSeconds int    `json:"expires_in,omitempty"`
}

func StartProductFlowSSO(c *gin.Context) {
	cfg := getProductFlowSSOConfig()
	if err := cfg.validateForStart(); err != nil {
		// Distinguish operator-disabled from misconfigured so the 503 body
		// stays unambiguous; both still map to ServiceUnavailable per the
		// SSO start contract.
		if errors.Is(err, errSSODisabled) {
			model.RecordLog(0, model.LogTypeSystem,
				"productflow_sso start failed: reason=disabled")
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Atelier SSO is disabled"})
			return
		}
		model.RecordLog(0, model.LogTypeSystem, fmt.Sprintf(
			"productflow_sso start failed: reason=%s", err.Error(),
		))
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": err.Error()})
		return
	}

	userID, err := currentBrowserSessionUserID(c)
	if err != nil {
		if errors.Is(err, errProductFlowSSOForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.Redirect(http.StatusFound, "/sign-in?redirect="+url.QueryEscape("/api/productflow/sso/start"))
		return
	}

	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Status != common.UserStatusEnabled {
		model.RecordLog(user.Id, model.LogTypeSystem,
			"productflow_sso start failed: reason=user_disabled")
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "user is disabled"})
		return
	}

	if err := redirectProductFlowUser(c, cfg, user); err != nil {
		common.ApiError(c, err)
	}
}

func VerifyProductFlowSSO(c *gin.Context) {
	cfg := getProductFlowSSOConfig()
	if err := cfg.validateForVerify(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": err.Error()})
		return
	}
	if !isValidProductFlowSecret(c, cfg.SharedSecret) {
		model.RecordLog(0, model.LogTypeSystem,
			"productflow_sso verify failed: reason=invalid_shared_secret")
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid shared secret"})
		return
	}

	var req productFlowVerifyRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}

	ticket := strings.TrimSpace(req.Ticket)
	claims, err := consumeProductFlowTicket(ticket)
	if err != nil {
		model.RecordLog(0, model.LogTypeSystem, fmt.Sprintf(
			"productflow_sso verify failed: ticket=%s reason=%s",
			hashProductFlowTicket(ticket), err.Error(),
		))
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiSuccess(c, claims)
}

func redirectProductFlowUser(c *gin.Context, cfg productFlowSSOConfig, user *model.User) error {
	token, err := getOrCreateProductFlowToken(user.Id, cfg)
	if err != nil {
		return err
	}

	ticket, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return err
	}

	claims := newProductFlowTicketClaims(user, token, cfg)
	ttl := time.Duration(cfg.TicketTTLSeconds) * time.Second
	if err := storeProductFlowTicket(ticket, claims, ttl); err != nil {
		return err
	}

	callbackURL, err := buildProductFlowCallbackURL(cfg.BaseURL, ticket)
	if err != nil {
		return err
	}
	// Record the lifecycle event with a one-way hash of the ticket so audit
	// trails can correlate start/verify pairs without leaking the bearer
	// credential. Keep the callback URL trimmed of any query string the
	// caller might have added so log scraping stays predictable.
	model.RecordLog(user.Id, model.LogTypeSystem, fmt.Sprintf(
		"productflow_sso start: ticket=%s callback=%s",
		hashProductFlowTicket(ticket), trimURLQuery(callbackURL),
	))
	c.Redirect(http.StatusFound, callbackURL)
	return nil
}

// hashProductFlowTicket returns a stable 16-char hex prefix of SHA-256(ticket)
// so audit logs can correlate start/verify events without persisting the
// bearer credential. Empty input maps to a fixed sentinel so log parsers can
// tell missing tickets apart.
func hashProductFlowTicket(ticket string) string {
	if ticket == "" {
		return "(empty)"
	}
	sum := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(sum[:])[:16]
}

// trimURLQuery strips the query string from a callback URL so audit log
// entries do not leak ticket values when the caller appended them as query
// parameters. The path and host remain intact for diagnostic value.
func trimURLQuery(rawURL string) string {
	if idx := strings.Index(rawURL, "?"); idx >= 0 {
		return rawURL[:idx]
	}
	return rawURL
}

func newProductFlowTicketClaims(user *model.User, token *model.Token, cfg productFlowSSOConfig) productFlowTicketClaims {
	return productFlowTicketClaims{
		UserID:           strconv.Itoa(user.Id),
		Username:         user.Username,
		Email:            user.Email,
		Group:            user.Group,
		Role:             strconv.Itoa(user.Role),
		Token:            "sk-" + token.Key,
		TokenID:          strconv.Itoa(token.Id),
		TokenName:        token.Name,
		ExpiresInSeconds: productFlowSessionTTLForRole(user.Role, cfg),
	}
}

func productFlowSessionTTLForRole(role int, cfg productFlowSSOConfig) int {
	if role >= common.RoleAdminUser {
		return cfg.AdminSessionTTLSeconds
	}
	return cfg.SessionTTLSeconds
}
