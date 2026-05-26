package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type productFlowAPIResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    productFlowTicketClaims `json:"data"`
}

func setupProductFlowSSOTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Option{},
		&model.Channel{},
		&model.Ability{},
		&model.Model{},
		&model.Log{},
	))
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func initProductFlowSSOOptions(t *testing.T) {
	t.Helper()

	model.InitOptionMap()
}

func seedProductFlowSSOOptions(t *testing.T, values map[string]string) {
	t.Helper()

	for key, value := range values {
		require.NoError(t, model.UpdateOption(key, value))
	}
}

func seedProductFlowSSODefaultOptions(t *testing.T) {
	t.Helper()

	seedProductFlowImageModel(t, model.DB, "image", "gpt-image-2")
	seedProductFlowSSOOptions(t, map[string]string{
		productFlowOptionBaseURL:         "https://image.example.com",
		productFlowOptionSharedSecret:    "test-secret",
		productFlowOptionTokenName:       "Atelier",
		productFlowOptionTokenGroup:      "image",
		productFlowOptionImageModel:      "gpt-image-2",
		productFlowOptionTicketTTL:       "60",
		productFlowOptionSessionTTL:      "3600",
		productFlowOptionAdminSessionTTL: "3600",
	})
}

func prepareProductFlowSSOTest(t *testing.T) *gorm.DB {
	t.Helper()

	db := setupProductFlowSSOTestDB(t)
	initProductFlowSSOOptions(t)
	seedProductFlowSSODefaultOptions(t)
	resetProductFlowMemoryTickets(t)
	return db
}

func resetProductFlowMemoryTickets(t *testing.T) {
	t.Helper()

	productFlowMemoryTickets.Lock()
	productFlowMemoryTickets.items = map[string]productFlowMemoryTicket{}
	productFlowMemoryTickets.Unlock()
}

func seedProductFlowImageModel(t *testing.T, db *gorm.DB, group string, modelName string) {
	t.Helper()

	channel := model.Channel{
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   group + "-image-channel",
		Models: modelName,
		Group:  group,
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: modelName,
		Endpoints: `["image-generation"]`,
		Status:    1,
	}).Error)
}

func seedProductFlowTextModel(t *testing.T, db *gorm.DB, group string, modelName string) {
	t.Helper()

	channel := model.Channel{
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   group + "-text-channel",
		Models: modelName,
		Group:  group,
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: modelName,
		Endpoints: `["openai-response"]`,
		Status:    1,
	}).Error)
}

func seedProductFlowUser(t *testing.T, db *gorm.DB) model.User {
	t.Helper()

	user := model.User{
		Username:    "alice",
		Password:    "password123",
		DisplayName: "Alice",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Email:       "alice@example.com",
		Group:       "default",
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func productFlowSSORouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("productflow-sso-test"))))
	router.GET("/api/productflow/sso/start", StartProductFlowSSO)
	router.POST("/api/productflow/sso/verify", VerifyProductFlowSSO)
	return router
}

func loginProductFlowSession(t *testing.T, router *gin.Engine, user model.User) []*http.Cookie {
	t.Helper()

	router.GET("/test/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		session.Set("role", user.Role)
		session.Set("status", user.Status)
		session.Set("group", user.Group)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test/login", nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func decodeProductFlowResponse(t *testing.T, recorder *httptest.ResponseRecorder) productFlowAPIResponse {
	t.Helper()

	var response productFlowAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestProductFlowStartRedirectsUnauthenticatedUsersToSignIn(t *testing.T) {
	prepareProductFlowSSOTest(t)
	router := productFlowSSORouter()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	require.Equal(t, "/sign-in?redirect=%2Fapi%2Fproductflow%2Fsso%2Fstart", recorder.Header().Get("Location"))
}

func TestProductFlowStartRedirectsUnauthenticatedUsersBeforeConfigValidation(t *testing.T) {
	prepareProductFlowSSOTest(t)
	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, ""))
	router := productFlowSSORouter()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	require.Equal(t, "/sign-in?redirect=%2Fapi%2Fproductflow%2Fsso%2Fstart", recorder.Header().Get("Location"))
	require.NotContains(t, recorder.Body.String(), "Atelier base URL")
}

func TestProductFlowStartShowsFriendlyHTMLForBrowserConfigErrors(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, ""))
	router := productFlowSSORouter()
	user := seedProductFlowUser(t, db)
	user.Role = common.RoleRootUser
	require.NoError(t, db.Save(&user).Error)
	cookies := loginProductFlowSession(t, router, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	request.Header.Set("Accept", "text/html")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
	require.Contains(t, recorder.Body.String(), "Atelier SSO")
	require.Contains(t, recorder.Body.String(), "/system-settings/operations/atelier-sso")
	require.Contains(t, recorder.Body.String(), "Atelier base URL is not configured")
	require.NotContains(t, recorder.Body.String(), `{"success":false`)
}

func TestProductFlowStartKeepsJSONForAPIConfigErrors(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, ""))
	router := productFlowSSORouter()
	user := seedProductFlowUser(t, db)
	cookies := loginProductFlowSession(t, router, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	request.Header.Set("Accept", "application/json")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
	response := decodeProductFlowResponse(t, recorder)
	require.False(t, response.Success)
	require.Equal(t, "Atelier SSO is not ready. Please contact an administrator.", response.Message)
}

func TestProductFlowConfigIgnoresEnvDefaults(t *testing.T) {
	setupProductFlowSSOTestDB(t)
	t.Setenv("PRODUCTFLOW_BASE_URL", "https://env.example.com")
	t.Setenv("PRODUCTFLOW_SSO_SECRET", "env-secret")
	t.Setenv("PRODUCTFLOW_TOKEN_NAME", "EnvFlow")
	t.Setenv("PRODUCTFLOW_TOKEN_GROUP", "env-group")
	t.Setenv("PRODUCTFLOW_SSO_TICKET_TTL_SECONDS", "900")
	t.Setenv("PRODUCTFLOW_SESSION_TTL_SECONDS", "1800")
	initProductFlowSSOOptions(t)

	cfg := getProductFlowSSOConfig()
	require.Empty(t, cfg.BaseURL)
	require.Empty(t, cfg.SharedSecret)
	require.Equal(t, "Atelier", cfg.TokenName)
	require.Empty(t, cfg.TokenGroup)
	require.Empty(t, cfg.ImageModel)
	require.Equal(t, productFlowDefaultTicketTTL, cfg.TicketTTLSeconds)
	require.Equal(t, productFlowDefaultSessionTTL, cfg.SessionTTLSeconds)
	require.Equal(t, productFlowDefaultAdminSessionTTL, cfg.AdminSessionTTLSeconds)
}

func TestProductFlowStartRejectsDisabledUsers(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	router := productFlowSSORouter()
	user := seedProductFlowUser(t, db)
	user.Status = common.UserStatusDisabled
	cookies := loginProductFlowSession(t, router, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "user is not allowed")
}

func TestProductFlowStartCreatesTokenAndRedirectsWithOneTimeTicket(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	router := productFlowSSORouter()
	user := seedProductFlowUser(t, db)
	cookies := loginProductFlowSession(t, router, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	redirectURL := recorder.Header().Get("Location")
	require.True(t, strings.HasPrefix(redirectURL, "https://image.example.com/auth/new-api/callback?ticket="))
	require.NotContains(t, redirectURL, "sk-")

	var token model.Token
	require.NoError(t, db.First(&token, "user_id = ? AND name = ?", user.Id, "Atelier").Error)
	require.Equal(t, common.TokenStatusEnabled, token.Status)
	require.True(t, token.UnlimitedQuota)
	require.False(t, token.ModelLimitsEnabled)
	require.Empty(t, token.ModelLimits)
	require.Equal(t, "image", token.Group)
	require.Equal(t, int64(-1), token.ExpiredTime)

	ticket := strings.TrimPrefix(redirectURL, "https://image.example.com/auth/new-api/callback?ticket=")
	verify := httptest.NewRecorder()
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/productflow/sso/verify", bytes.NewBufferString(`{"ticket":"`+ticket+`"}`))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Authorization", "Bearer test-secret")
	router.ServeHTTP(verify, verifyReq)

	require.Equal(t, http.StatusOK, verify.Code)
	response := decodeProductFlowResponse(t, verify)
	require.True(t, response.Success)
	require.Equal(t, "alice", response.Data.Username)
	require.Equal(t, "alice@example.com", response.Data.Email)
	require.Equal(t, "default", response.Data.Group)
	require.Equal(t, "1", response.Data.Role)
	require.Equal(t, "Atelier", response.Data.TokenName)
	require.Equal(t, "image", response.Data.TokenGroup)
	require.Equal(t, "gpt-image-2", response.Data.ImageModel)
	require.Equal(t, []string{"gpt-image-2"}, response.Data.ImageModels)
	require.Equal(t, "sk-"+token.Key, response.Data.Token)
	require.NotEmpty(t, response.Data.TokenID)
	require.Equal(t, 3600, response.Data.ExpiresInSeconds)
}

func TestProductFlowStartIncludesAllEnabledImageModelsForTokenGroup(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	seedProductFlowImageModel(t, db, "image", "gpt-image-3")
	seedProductFlowTextModel(t, db, "image", "gpt-5.4")
	seedProductFlowTextModel(t, db, "image", "gpt-5.5")
	router := productFlowSSORouter()
	user := seedProductFlowUser(t, db)
	cookies := loginProductFlowSession(t, router, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	ticket := strings.TrimPrefix(
		recorder.Header().Get("Location"),
		"https://image.example.com/auth/new-api/callback?ticket=",
	)
	verify := httptest.NewRecorder()
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/productflow/sso/verify", bytes.NewBufferString(`{"ticket":"`+ticket+`"}`))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Authorization", "Bearer test-secret")
	router.ServeHTTP(verify, verifyReq)

	require.Equal(t, http.StatusOK, verify.Code)
	response := decodeProductFlowResponse(t, verify)
	require.Equal(t, "gpt-image-2", response.Data.ImageModel)
	require.Equal(t, []string{"gpt-image-2", "gpt-image-3"}, response.Data.ImageModels)
	require.Equal(t, "gpt-5.4", response.Data.TextModel)
	require.Equal(t, []string{"gpt-5.4", "gpt-5.5"}, response.Data.TextModels)
}

func TestProductFlowStartDefaultsSingleImageModelForLegacyConfig(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	require.NoError(t, model.UpdateOption(productFlowOptionImageModel, ""))
	router := productFlowSSORouter()
	user := seedProductFlowUser(t, db)
	cookies := loginProductFlowSession(t, router, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	ticket := strings.TrimPrefix(
		recorder.Header().Get("Location"),
		"https://image.example.com/auth/new-api/callback?ticket=",
	)
	verify := httptest.NewRecorder()
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/productflow/sso/verify", bytes.NewBufferString(`{"ticket":"`+ticket+`"}`))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Authorization", "Bearer test-secret")
	router.ServeHTTP(verify, verifyReq)

	require.Equal(t, http.StatusOK, verify.Code)
	response := decodeProductFlowResponse(t, verify)
	require.Equal(t, "gpt-image-2", response.Data.ImageModel)
}

func TestProductFlowStartUsesAdminSessionTTLForAdminUsers(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	router := productFlowSSORouter()
	user := seedProductFlowUser(t, db)
	user.Role = common.RoleAdminUser
	require.NoError(t, db.Save(&user).Error)
	require.NoError(t, model.UpdateOption(productFlowOptionSessionTTL, "7200"))
	require.NoError(t, model.UpdateOption(productFlowOptionAdminSessionTTL, "900"))
	cookies := loginProductFlowSession(t, router, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	ticket := strings.TrimPrefix(
		recorder.Header().Get("Location"),
		"https://image.example.com/auth/new-api/callback?ticket=",
	)

	verify := httptest.NewRecorder()
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/productflow/sso/verify", bytes.NewBufferString(`{"ticket":"`+ticket+`"}`))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Authorization", "Bearer test-secret")
	router.ServeHTTP(verify, verifyReq)

	require.Equal(t, http.StatusOK, verify.Code)
	response := decodeProductFlowResponse(t, verify)
	require.Equal(t, "10", response.Data.Role)
	require.Equal(t, 900, response.Data.ExpiresInSeconds)
}

func TestProductFlowTokenIsReusedAndUpdatedFromConfig(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	user := seedProductFlowUser(t, db)
	existing := model.Token{
		UserId:             user.Id,
		Name:               "Atelier",
		Key:                "existing-key",
		Status:             common.TokenStatusDisabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        123,
		RemainQuota:        10,
		UnlimitedQuota:     false,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		Group:              "old",
		CrossGroupRetry:    true,
	}
	require.NoError(t, db.Create(&existing).Error)

	token, err := getOrCreateProductFlowToken(user.Id, getProductFlowSSOConfig())
	require.NoError(t, err)
	require.Equal(t, existing.Id, token.Id)
	require.Equal(t, "existing-key", token.Key)
	require.Equal(t, common.TokenStatusEnabled, token.Status)
	require.Equal(t, int64(-1), token.ExpiredTime)
	require.True(t, token.UnlimitedQuota)
	require.False(t, token.ModelLimitsEnabled)
	require.Empty(t, token.ModelLimits)
	require.Equal(t, "image", token.Group)
	require.False(t, token.CrossGroupRetry)
}

func TestProductFlowStartUsesDatabaseBackedOptions(t *testing.T) {
	db := prepareProductFlowSSOTest(t)

	seedProductFlowImageModel(t, db, "db-image", "gpt-image-3")
	require.NoError(t, model.UpdateOption(productFlowOptionBaseURL, "https://db.example.com/"))
	require.NoError(t, model.UpdateOption(productFlowOptionSharedSecret, "db-secret"))
	require.NoError(t, model.UpdateOption(productFlowOptionTokenName, "ProductFlow DB"))
	require.NoError(t, model.UpdateOption(productFlowOptionTokenGroup, "db-image"))
	require.NoError(t, model.UpdateOption(productFlowOptionImageModel, "gpt-image-3"))
	require.NoError(t, model.UpdateOption(productFlowOptionTicketTTL, "90"))
	require.NoError(t, model.UpdateOption(productFlowOptionSessionTTL, "7200"))
	require.NoError(t, model.UpdateOption(productFlowOptionAdminSessionTTL, "1800"))

	router := productFlowSSORouter()
	user := seedProductFlowUser(t, db)
	cookies := loginProductFlowSession(t, router, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/productflow/sso/start", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusFound, recorder.Code)
	redirectURL := recorder.Header().Get("Location")
	require.True(t, strings.HasPrefix(redirectURL, "https://db.example.com/auth/new-api/callback?ticket="))

	ticket := strings.TrimPrefix(redirectURL, "https://db.example.com/auth/new-api/callback?ticket=")
	verify := httptest.NewRecorder()
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/productflow/sso/verify", bytes.NewBufferString(`{"ticket":"`+ticket+`"}`))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Authorization", "Bearer db-secret")
	router.ServeHTTP(verify, verifyReq)

	require.Equal(t, http.StatusOK, verify.Code)
	response := decodeProductFlowResponse(t, verify)
	require.True(t, response.Success)
	require.Equal(t, "ProductFlow DB", response.Data.TokenName)
	require.Equal(t, "db-image", response.Data.TokenGroup)
	require.Equal(t, "gpt-image-3", response.Data.ImageModel)
	require.Equal(t, 7200, response.Data.ExpiresInSeconds)

	var token model.Token
	require.NoError(t, db.First(&token, "user_id = ? AND name = ?", user.Id, "ProductFlow DB").Error)
	require.False(t, token.ModelLimitsEnabled)
	require.Empty(t, token.ModelLimits)
	require.Equal(t, "db-image", token.Group)
}

func TestProductFlowSSOConfigRejectsStaleImageModelForTokenGroup(t *testing.T) {
	prepareProductFlowSSOTest(t)

	_, err, failingKey := normalizeBatchOptionUpdates([]OptionUpdateRequest{
		{Key: productFlowOptionTokenGroup, Value: "image"},
		{Key: productFlowOptionImageModel, Value: "gpt-image-1"},
	})

	require.Error(t, err)
	require.Equal(t, productFlowOptionImageModel, failingKey)
	require.Contains(t, err.Error(), "is not enabled")
}

func TestProductFlowSSOConfigAllowsBlankImageModelForUserSelection(t *testing.T) {
	db := prepareProductFlowSSOTest(t)
	seedProductFlowImageModel(t, db, "image", "gpt-image-3")

	_, err, failingKey := normalizeBatchOptionUpdates([]OptionUpdateRequest{
		{Key: productFlowOptionTokenGroup, Value: "image"},
		{Key: productFlowOptionImageModel, Value: ""},
	})

	require.NoError(t, err)
	require.Empty(t, failingKey)
}

func TestProductFlowSSOConfigRejectsTokenGroupWithoutImageModels(t *testing.T) {
	prepareProductFlowSSOTest(t)

	_, err, failingKey := normalizeBatchOptionUpdates([]OptionUpdateRequest{
		{Key: productFlowOptionTokenGroup, Value: "text-only"},
		{Key: productFlowOptionImageModel, Value: "gpt-image-2"},
	})

	require.Error(t, err)
	require.Equal(t, productFlowOptionImageModel, failingKey)
	require.Contains(t, err.Error(), "has no enabled image-generation models")
}
