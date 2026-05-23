package controller

import (
	"errors"
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
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid shared secret"})
		return
	}

	var req productFlowVerifyRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}

	claims, err := consumeProductFlowTicket(strings.TrimSpace(req.Ticket))
	if err != nil {
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
	c.Redirect(http.StatusFound, callbackURL)
	return nil
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
		ExpiresInSeconds: cfg.SessionTTLSeconds,
	}
}
