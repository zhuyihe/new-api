package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
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
	UserID           string   `json:"user_id"`
	Username         string   `json:"username,omitempty"`
	Email            string   `json:"email,omitempty"`
	Group            string   `json:"group,omitempty"`
	Role             string   `json:"role,omitempty"`
	Token            string   `json:"token,omitempty"`
	TokenID          string   `json:"token_id,omitempty"`
	TokenName        string   `json:"token_name,omitempty"`
	TokenGroup       string   `json:"token_group,omitempty"`
	ImageModel       string   `json:"image_model,omitempty"`
	ImageModels      []string `json:"image_models,omitempty"`
	TextModel        string   `json:"text_model,omitempty"`
	TextModels       []string `json:"text_models,omitempty"`
	ExpiresInSeconds int      `json:"expires_in,omitempty"`
}

const productFlowSSOStartUnavailableTemplate = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    :root { color-scheme: light dark; }
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f6f7f9; color: #18181b; }
    main { width: min(92vw, 520px); box-sizing: border-box; border: 1px solid #e4e4e7; border-radius: 8px; background: #fff; padding: 28px; box-shadow: 0 18px 50px rgba(15, 23, 42, .08); }
    h1 { margin: 0 0 10px; font-size: 22px; line-height: 1.25; }
    p { margin: 0; color: #52525b; line-height: 1.65; }
    .detail { margin-top: 16px; border: 1px solid #fde68a; border-radius: 6px; background: #fffbeb; color: #92400e; padding: 10px 12px; word-break: break-word; }
    .button { display: inline-flex; margin-top: 18px; align-items: center; justify-content: center; min-height: 38px; border-radius: 6px; background: #18181b; color: #fff; padding: 0 14px; text-decoration: none; font-size: 14px; font-weight: 600; }
    @media (prefers-color-scheme: dark) {
      body { background: #09090b; color: #fafafa; }
      main { background: #18181b; border-color: #27272a; box-shadow: none; }
      p { color: #d4d4d8; }
      .detail { background: #2f2608; border-color: #854d0e; color: #fde68a; }
      .button { background: #fafafa; color: #18181b; }
    }
  </style>
</head>
<body>
  <main>
    <h1>%s</h1>
    <p>%s</p>
    %s
  </main>
</body>
</html>`

func StartProductFlowSSO(c *gin.Context) {
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

	cfg := getProductFlowSSOConfig()
	if err := cfg.validateForStart(); err != nil {
		// Validate after browser auth so unauthenticated users are always sent
		// through the normal sign-in flow and public requests cannot probe SSO
		// operator configuration.
		reason := err.Error()
		if errors.Is(err, errSSODisabled) {
			reason = "disabled"
		}
		model.RecordLog(user.Id, model.LogTypeSystem, fmt.Sprintf(
			"productflow_sso start failed: reason=%s", reason,
		))
		respondProductFlowSSOStartUnavailable(c, user, err)
		return
	}

	if err := redirectProductFlowUser(c, cfg, user); err != nil {
		common.ApiError(c, err)
	}
}

func respondProductFlowSSOStartUnavailable(c *gin.Context, user *model.User, err error) {
	message := productFlowSSOStartUnavailableMessage(user, err)
	if productFlowSSOWantsJSON(c) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": message})
		return
	}
	c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8",
		[]byte(renderProductFlowSSOStartUnavailableHTML(user, err, message)))
}

func productFlowSSOWantsJSON(c *gin.Context) bool {
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}

func productFlowSSOStartUnavailableMessage(user *model.User, err error) string {
	if errors.Is(err, errSSODisabled) {
		return "Atelier SSO is disabled"
	}
	if user != nil && user.Role >= common.RoleRootUser {
		return err.Error()
	}
	return "Atelier SSO is not ready. Please contact an administrator."
}

func renderProductFlowSSOStartUnavailableHTML(user *model.User, err error, message string) string {
	title := "Atelier SSO 还没准备好"
	intro := "当前无法打开 Atelier，请联系管理员检查 New API 的 Atelier SSO 配置。"
	if errors.Is(err, errSSODisabled) {
		title = "Atelier SSO 已停用"
		intro = "当前无法打开 Atelier，请联系管理员重新启用 Atelier SSO。"
	}

	var adminAction string
	if user != nil && user.Role >= common.RoleRootUser {
		adminAction = fmt.Sprintf(
			`<p class="detail">配置问题：%s</p><a class="button" href="/system-settings/operations/atelier-sso">打开 Atelier SSO 设置</a>`,
			html.EscapeString(message),
		)
	}

	return fmt.Sprintf(productFlowSSOStartUnavailableTemplate,
		html.EscapeString(title),
		html.EscapeString(title),
		html.EscapeString(intro),
		adminAction,
	)
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
	textModels := productFlowTextModelsForClaims(token.Group)
	textModel := ""
	if len(textModels) > 0 {
		textModel = textModels[0]
	}
	return productFlowTicketClaims{
		UserID:           strconv.Itoa(user.Id),
		Username:         user.Username,
		Email:            user.Email,
		Group:            user.Group,
		Role:             strconv.Itoa(user.Role),
		Token:            "sk-" + token.Key,
		TokenID:          strconv.Itoa(token.Id),
		TokenName:        token.Name,
		TokenGroup:       token.Group,
		ImageModel:       cfg.ImageModel,
		ImageModels:      productFlowImageModelsForClaims(token.Group, cfg.ImageModel),
		TextModel:        textModel,
		TextModels:       textModels,
		ExpiresInSeconds: productFlowSessionTTLForRole(user.Role, cfg),
	}
}

func productFlowImageModelsForClaims(group string, selectedModel string) []string {
	models, err := model.GetGroupEnabledImageModels(group)
	if err != nil {
		models = nil
	}
	selectedModel = strings.TrimSpace(selectedModel)
	if selectedModel == "" {
		return models
	}
	ordered := []string{selectedModel}
	for _, modelName := range models {
		if modelName != selectedModel {
			ordered = append(ordered, modelName)
		}
	}
	return ordered
}

func productFlowTextModelsForClaims(group string) []string {
	models, err := model.GetGroupEnabledTextModels(group)
	if err != nil || len(models) == 0 {
		return nil
	}
	return models
}

func productFlowSessionTTLForRole(role int, cfg productFlowSSOConfig) int {
	if role >= common.RoleAdminUser {
		return cfg.AdminSessionTTLSeconds
	}
	return cfg.SessionTTLSeconds
}
