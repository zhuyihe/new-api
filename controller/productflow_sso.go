package controller

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	productFlowDefaultTokenName  = "ProductFlow"
	productFlowDefaultTicketTTL  = 60
	productFlowDefaultSessionTTL = 14 * 24 * 60 * 60
	productFlowTicketKeyPrefix   = "productflow:sso:ticket:"
)

var (
	errProductFlowSSONotLoggedIn = errors.New("not logged in")
	errProductFlowSSOForbidden   = errors.New("user is not allowed to start ProductFlow SSO")
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

type productFlowMemoryTicket struct {
	Claims    productFlowTicketClaims
	ExpiresAt time.Time
}

var productFlowMemoryTickets = struct {
	sync.Mutex
	items map[string]productFlowMemoryTicket
}{items: map[string]productFlowMemoryTicket{}}

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

	token, err := getOrCreateProductFlowToken(user.Id, cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ticket, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	claims := productFlowTicketClaims{
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
	if err := storeProductFlowTicket(ticket, claims, time.Duration(cfg.TicketTTLSeconds)*time.Second); err != nil {
		common.ApiError(c, err)
		return
	}

	callbackURL, err := buildProductFlowCallbackURL(cfg.BaseURL, ticket)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Redirect(http.StatusFound, callbackURL)
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

func currentBrowserSessionUserID(c *gin.Context) (int, error) {
	session := sessions.Default(c)
	id, ok := session.Get("id").(int)
	if !ok || id <= 0 {
		return 0, errProductFlowSSONotLoggedIn
	}
	status, ok := session.Get("status").(int)
	if !ok || status != common.UserStatusEnabled {
		return 0, errProductFlowSSOForbidden
	}
	role, ok := session.Get("role").(int)
	if !ok || role < common.RoleCommonUser {
		return 0, errProductFlowSSOForbidden
	}
	return id, nil
}

func getOrCreateProductFlowToken(userID int, cfg productFlowSSOConfig) (*model.Token, error) {
	var token model.Token
	err := model.DB.Where("user_id = ? AND name = ?", userID, cfg.TokenName).Order("id desc").Limit(1).Find(&token).Error
	if err != nil {
		return nil, err
	}
	if token.Id == 0 {
		key, keyErr := common.GenerateKey()
		if keyErr != nil {
			return nil, keyErr
		}
		token = model.Token{
			UserId:             userID,
			Name:               cfg.TokenName,
			Key:                key,
			Status:             common.TokenStatusEnabled,
			CreatedTime:        common.GetTimestamp(),
			AccessedTime:       common.GetTimestamp(),
			ExpiredTime:        -1,
			RemainQuota:        0,
			UnlimitedQuota:     true,
			ModelLimitsEnabled: cfg.TokenModelLimits != "",
			ModelLimits:        cfg.TokenModelLimits,
			Group:              cfg.TokenGroup,
			CrossGroupRetry:    false,
		}
		if err := token.Insert(); err != nil {
			return nil, err
		}
		return &token, nil
	}

	token.Status = common.TokenStatusEnabled
	token.ExpiredTime = -1
	token.UnlimitedQuota = true
	token.ModelLimitsEnabled = cfg.TokenModelLimits != ""
	token.ModelLimits = cfg.TokenModelLimits
	token.Group = cfg.TokenGroup
	token.CrossGroupRetry = false
	if err := token.Update(); err != nil {
		return nil, err
	}
	return &token, nil
}

func buildProductFlowCallbackURL(baseURL string, ticket string) (string, error) {
	callback, err := url.Parse(common.BuildURL(strings.TrimRight(baseURL, "/")+"/", "/auth/new-api/callback"))
	if err != nil {
		return "", err
	}
	q := callback.Query()
	q.Set("ticket", ticket)
	callback.RawQuery = q.Encode()
	return callback.String(), nil
}

func storeProductFlowTicket(ticket string, claims productFlowTicketClaims, ttl time.Duration) error {
	if ticket == "" {
		return errors.New("empty ticket")
	}
	if common.RedisEnabled && common.RDB != nil {
		payload, err := common.Marshal(claims)
		if err != nil {
			return err
		}
		// Ticket payload contains token material; avoid RedisSet debug logging.
		return common.RDB.Set(context.Background(), productFlowTicketKeyPrefix+ticket, string(payload), ttl).Err()
	}
	productFlowMemoryTickets.Lock()
	defer productFlowMemoryTickets.Unlock()
	pruneExpiredProductFlowTicketsLocked(time.Now())
	productFlowMemoryTickets.items[ticket] = productFlowMemoryTicket{
		Claims:    claims,
		ExpiresAt: time.Now().Add(ttl),
	}
	return nil
}

func consumeProductFlowTicket(ticket string) (productFlowTicketClaims, error) {
	if ticket == "" {
		return productFlowTicketClaims{}, errors.New("missing ticket")
	}
	if common.RedisEnabled && common.RDB != nil {
		return consumeProductFlowTicketFromRedis(ticket)
	}
	return consumeProductFlowTicketFromMemory(ticket)
}

func consumeProductFlowTicketFromRedis(ticket string) (productFlowTicketClaims, error) {
	const script = `local value = redis.call("GET", KEYS[1]); if value then redis.call("DEL", KEYS[1]); end; return value`
	result, err := common.RDB.Eval(context.Background(), script, []string{productFlowTicketKeyPrefix + ticket}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return productFlowTicketClaims{}, errors.New("ticket is invalid or expired")
		}
		return productFlowTicketClaims{}, err
	}
	payload, ok := result.(string)
	if !ok || payload == "" {
		return productFlowTicketClaims{}, errors.New("ticket is invalid or expired")
	}
	var claims productFlowTicketClaims
	if err := common.UnmarshalJsonStr(payload, &claims); err != nil {
		return productFlowTicketClaims{}, err
	}
	return claims, nil
}

func consumeProductFlowTicketFromMemory(ticket string) (productFlowTicketClaims, error) {
	productFlowMemoryTickets.Lock()
	defer productFlowMemoryTickets.Unlock()
	now := time.Now()
	record, ok := productFlowMemoryTickets.items[ticket]
	if !ok || record.ExpiresAt.Before(now) {
		delete(productFlowMemoryTickets.items, ticket)
		return productFlowTicketClaims{}, errors.New("ticket is invalid or expired")
	}
	delete(productFlowMemoryTickets.items, ticket)
	return record.Claims, nil
}

func pruneExpiredProductFlowTicketsLocked(now time.Time) {
	for ticket, record := range productFlowMemoryTickets.items {
		if record.ExpiresAt.Before(now) {
			delete(productFlowMemoryTickets.items, ticket)
		}
	}
}

func isValidProductFlowSecret(c *gin.Context, expected string) bool {
	actual := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(actual), "bearer ") {
		actual = strings.TrimSpace(actual[7:])
	}
	if actual == "" {
		actual = strings.TrimSpace(c.GetHeader("X-ProductFlow-SSO-Secret"))
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
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
