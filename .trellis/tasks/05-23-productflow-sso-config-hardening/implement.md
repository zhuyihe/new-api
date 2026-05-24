---
task: productflow-sso-config-hardening
parent_task: productflow-sso
created: 2026-05-23
---

# Implementation Plan

## Phase 0: P0 Bug Fix + zh/en i18n (0.5 day, independently releasable)

Goal: ship the secret-clearing bug fix and Chinese/English admin UI text in a
single commit that does not depend on any of the architectural work in later
phases.

### Files

- `web/default/src/features/system-settings/integrations/productflow-sso-settings-section.tsx`
- `web/default/src/i18n/locales/zh.json`
- `web/default/src/i18n/locales/en.json`
- `web/default/src/i18n/locales/fr.json` (sync only)
- `web/default/src/i18n/locales/ja.json` (sync only)
- `web/default/src/i18n/locales/ru.json` (sync only)
- `web/default/src/i18n/locales/vi.json` (sync only)

### Steps

- [ ] In `productflow-sso-settings-section.tsx` `onSubmit`, augment
      `changedKeys` filter:

      ```ts
      const changedKeys = (Object.keys(normalized) as Array<keyof NormalizedProductFlowSSOValues>)
        .filter((key) => normalized[key] !== baselineRef.current[key])
        .filter((key) => !(key === 'productflow_sso.shared_secret' && normalized[key] === ''))
      ```

- [ ] Add a unit test (or extend an existing one) that asserts: given
      `defaultValues.shared_secret = ''` and a change to `token_name`, the
      submit only fires one update call without `shared_secret`.
- [ ] Add 24 keys to `zh.json` covering every `t()` call in this section plus
      the planned net-new components. The keys mirror the English source
      strings used in `t('...')`. Suggested groups:

      ```
      // Section header
      "ProductFlow SSO": "ProductFlow SSO",
      "Configure the New API bridge used to open ProductFlow in a new tab":
        "配置 New API 与 ProductFlow 图像工作区的 SSO 桥",
      "Configure the ProductFlow image workspace bridge":
        "配置 ProductFlow 图像工作区桥",

      // Existing field labels
      "ProductFlow base URL": "ProductFlow 公网地址",
      "Public ProductFlow address used for the callback.":
        "ProductFlow 的公网地址，用于 SSO 回调。",
      "https://image.example.com": "https://image.example.com",
      "Token name": "Token 名称",
      "Name of the dedicated New API token for ProductFlow.":
        "为 ProductFlow 分配的专属 New API Token 名称。",
      "ProductFlow": "ProductFlow",
      "Token group": "Token 分组",
      "Optional New API group assigned to the token.":
        "Token 所属的 New API 分组（可选）。",
      "image": "image",
      "SSO shared secret": "SSO 共享密钥",
      "Leave blank to keep the existing secret":
        "留空则保留原密钥",
      "Used by ProductFlow to verify server-to-server tickets.":
        "ProductFlow 用于校验服务间 ticket 的共享密钥。",
      "Token model whitelist": "Token 可用模型白名单",
      "gpt-image-1, veo-3, seedance-1": "gpt-image-1, veo-3, seedance-1",
      "Comma-separated models allowed for the ProductFlow token.":
        "逗号分隔的可用模型列表。",
      "Ticket TTL (seconds)": "Ticket 有效期 (秒)",
      "How long the one-time SSO ticket stays valid.":
        "一次性 SSO ticket 的有效时长。",
      "Session TTL (seconds)": "会话有效期 (秒)",
      "Lifetime hint returned to ProductFlow after login.":
        "登录成功后告知 ProductFlow 的会话时长建议。",
      "Save ProductFlow settings": "保存 ProductFlow 配置",
      "Saving...": "保存中...",
      "No changes to save": "没有可保存的修改",
      "Provide a valid URL starting with http:// or https://":
        "请输入以 http:// 或 https:// 开头的合法地址",
      "Enter a positive integer": "请输入正整数"
      ```

- [ ] Add the same 24 keys to `en.json` with English values (most are
      identity mappings; some can be wording polish).
- [ ] From `web/default/`, run `bun run i18n:sync`. Verify zero errors and
      that `fr.json / ja.json / ru.json / vi.json` now contain the 24 keys
      with English fallback values.
- [ ] Check `web/default/src/i18n/locales/_reports/` for the diff report;
      confirm no `extras` show up for the touched section.

### Validation

```bash
cd web/default
bun run lint
bun run typecheck
bun run i18n:sync
```

Manual: open the admin page in `?lng=zh` query, confirm every label is
Chinese; verify saving the form with a partially-filled secret does not
clear it.

## Phase 1: Backend - enabled, status, batch, test, audit (1.0 day)

### Files

- `controller/productflow_sso_config.go`
- `controller/productflow_sso.go`
- `controller/productflow_sso_test_endpoint.go` (new)
- `controller/productflow_sso_status.go` (new)
- `controller/option.go` (add batch endpoint near `UpdateOption`)
- `controller/option_batch.go` (new, optional split for cleanliness)
- `model/log.go` (no schema change; only call-site additions)
- `router/api-router.go` (register 3 new endpoints)

### Steps

#### R3 - `enabled` field

- [ ] Add to `productflow_sso_config.go`:

      ```go
      const productFlowOptionEnabled = "productflow_sso.enabled"

      // In productFlowSSOConfig
      Enabled bool

      // In getProductFlowSSOConfig
      Enabled: getProductFlowOptionBool(productFlowOptionEnabled, true),
      ```

- [ ] Add `getProductFlowOptionBool(optionKey string, fallback bool) bool` helper
      mirroring the existing int/string helpers; trim and parse `"true"` /
      `"false"` (case insensitive).
- [ ] In `model/option.go` `InitOptionMap` (the existing default-seed block
      around lines 82-88 that already seeds the other six `productflow_sso.*`
      keys), add the new key right after the existing ProductFlow block so
      fresh installs and existing deploys both pick up the safe default:

      ```go
      common.OptionMap["productflow_sso.enabled"] = strconv.FormatBool(true)
      ```

      Placement matters: this must run before
      `WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup` is called from
      `main.go`, otherwise the `Enabled` field will read its hard-coded
      fallback (also `true`) instead of the seeded value, which works today
      but leaves a sharp edge for the next person who changes the helper.
      The existing seed order already satisfies this; just keep the new line
      in the same block.

- [ ] In `validateForStart`, add as first check:

      ```go
      if !cfg.Enabled {
          return errSSODisabled
      }
      ```

      where `errSSODisabled = errors.New("ProductFlow SSO is disabled")` is a
      package-level sentinel.

- [ ] In `StartProductFlowSSO`, distinguish disabled from misconfigured so the
      503 message is operator-friendly:

      ```go
      if err := cfg.validateForStart(); err != nil {
          if errors.Is(err, errSSODisabled) {
              c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "ProductFlow SSO is disabled"})
              return
          }
          c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": err.Error()})
          return
      }
      ```

- [ ] In `WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup`, the existing
      early return on `validateForStart() != nil` must be restructured so the
      INFO is logged when disabled but otherwise-valid:

      ```go
      func WarnIfProductFlowSSOTicketFallbackIsRiskyOnStartup() {
          cfg := getProductFlowSSOConfig()
          if !cfg.Enabled {
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
          common.SysLog("WARN: ...existing message...")
      }
      ```

#### R5 - `PUT /api/option/batch` with in-transaction audit

- [ ] In `controller/option.go` (or new `option_batch.go`), implement:

      ```go
      type BatchOptionUpdateRequest struct {
          Updates []OptionUpdateRequest `json:"updates"`
      }

      type BatchOptionUpdateResponse struct {
          Success    bool     `json:"success"`
          Message    string   `json:"message,omitempty"`
          FailedKeys []string `json:"failed_keys,omitempty"`
      }

      func UpdateOptionsBatch(c *gin.Context) {
          adminIDValue, _ := c.Get("id")
          adminID, _ := adminIDValue.(int)

          var req BatchOptionUpdateRequest
          if err := common.DecodeJson(c.Request.Body, &req); err != nil {
              c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
              return
          }
          if len(req.Updates) == 0 {
              c.JSON(http.StatusOK, BatchOptionUpdateResponse{Success: true})
              return
          }

          // Normalize values to strings using same logic as UpdateOption.
          normalized := make([]OptionUpdateRequest, 0, len(req.Updates))
          for _, upd := range req.Updates {
              valStr := coerceOptionValueToString(upd.Value)
              if normalizedValue, err := validateProductFlowOptionValue(upd.Key, valStr); err == nil {
                  valStr = normalizedValue
              } else {
                  c.JSON(http.StatusBadRequest, BatchOptionUpdateResponse{
                      Success:    false,
                      Message:    err.Error(),
                      FailedKeys: []string{upd.Key},
                  })
                  return
              }
              normalized = append(normalized, OptionUpdateRequest{Key: upd.Key, Value: valStr})
          }

          var changes []map[string]string
          err := model.DB.Transaction(func(tx *gorm.DB) error {
              for _, upd := range normalized {
                  oldValue := readOptionValue(tx, upd.Key)
                  newValue := upd.Value.(string)
                  if oldValue == newValue {
                      continue
                  }
                  if err := writeOptionValue(tx, upd.Key, newValue); err != nil {
                      return err
                  }
                  changes = append(changes, map[string]string{
                      "key":    upd.Key,
                      "before": maskIfSensitive(upd.Key, oldValue),
                      "after":  maskIfSensitive(upd.Key, newValue),
                  })
              }
              if len(changes) == 0 {
                  return nil
              }
              adminUsername, _ := model.GetUsernameById(adminID, false)
              other := map[string]interface{}{
                  "admin_info": map[string]interface{}{
                      "admin_id":       adminID,
                      "admin_username": adminUsername,
                      "changes":        changes,
                  },
              }
              return tx.Create(&model.Log{
                  UserId:    adminID,
                  Username:  adminUsername,
                  CreatedAt: common.GetTimestamp(),
                  Type:      model.LogTypeManage,
                  Content:   "ProductFlow SSO config updated",
                  Other:     common.MapToJsonStr(other),
              }).Error
          })

          if err != nil {
              // Log raw cause server-side for diagnosis; return generic
              // message client-side (same policy as Q2 / PRD R4).
              common.SysError(fmt.Sprintf(
                  "ProductFlow SSO batch save transaction failed: %v", err,
              ))
              c.JSON(http.StatusInternalServerError, BatchOptionUpdateResponse{
                  Success: false,
                  Message: "Failed to save settings (see server logs)",
              })
              return
          }
          c.JSON(http.StatusOK, BatchOptionUpdateResponse{Success: true})
      }

      func maskIfSensitive(key, value string) string {
          lower := strings.ToLower(key)
          if strings.HasSuffix(lower, "secret") ||
              strings.HasSuffix(lower, "key") ||
              strings.HasSuffix(lower, "token") {
              if value == "" {
                  return "(empty)"
              }
              sum := sha256.Sum256([]byte(value))
              return "***" + hex.EncodeToString(sum[:])[:8]
          }
          return value
      }
      ```

- [ ] `readOptionValue` and `writeOptionValue` should reuse the existing
      OptionMap mutation path (`model.UpdateOption` or equivalent). If the
      existing code only exposes `UpdateOption(tx)` that handles a single
      key, refactor to expose the inner transactional helper.
- [ ] `coerceOptionValueToString` extracted from the existing
      `UpdateOption.switch option.Value.(type)` block.

#### R4 - `GET /api/productflow/sso/status`

- [ ] In `controller/productflow_sso_status.go`:

      ```go
      type ProductFlowSSOStatusResponse struct {
          Enabled            bool                   `json:"enabled"`
          Configured         bool                   `json:"configured"`
          RedisEnabled       bool                   `json:"redis_enabled"`
          CallbackURLPreview string                 `json:"callback_url_preview"`
          LastTestResult     *productFlowTestResult `json:"last_test_result"`
      }

      func GetProductFlowSSOStatus(c *gin.Context) {
          cfg := getProductFlowSSOConfig()
          configured := cfg.BaseURL != "" && cfg.SharedSecret != ""
          var callbackPreview string
          if cfg.BaseURL != "" {
              callbackPreview = strings.TrimRight(cfg.BaseURL, "/") + "/auth/new-api/callback"
          }
          lastTest := loadLastTestResult()
          c.JSON(http.StatusOK, ProductFlowSSOStatusResponse{
              Enabled:            cfg.Enabled,
              Configured:         configured,
              RedisEnabled:       common.RedisEnabled,
              CallbackURLPreview: callbackPreview,
              LastTestResult:     lastTest,
          })
      }
      ```

- [ ] `loadLastTestResult()` reads OptionMap key
      `productflow_sso.last_test_result` (JSON-encoded). Returns nil if
      missing/malformed.

#### R6 - `POST /api/productflow/sso/test`

- [ ] In `controller/productflow_sso_test_endpoint.go`:

      ```go
      type productFlowTestRequest struct {
          BaseURL string `json:"base_url"`
      }

      type productFlowTestResult struct {
          OK            bool   `json:"ok"`
          Category      string `json:"category"`
          LatencyMs     int    `json:"latency_ms"`
          TestedAgainst string `json:"tested_against"`
          TestedAt      int64  `json:"tested_at"`
          Message       string `json:"message"`
      }

      var productFlowTestHTTPClient = &http.Client{Timeout: 3 * time.Second}

      func TestProductFlowSSOConnection(c *gin.Context) {
          var req productFlowTestRequest
          _ = common.DecodeJson(c.Request.Body, &req)

          cfg := getProductFlowSSOConfig()
          baseURL := strings.TrimSpace(req.BaseURL)
          source := "draft"
          if baseURL == "" {
              baseURL = cfg.BaseURL
              source = "saved"
          }
          if baseURL == "" {
              c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "no base_url to test"})
              return
          }

          result := performHealthProbe(baseURL, source)
          persistLastTestResult(result)
          common.SysLog(fmt.Sprintf(
              "ProductFlow SSO test connection: base_url=%s result=%s latency=%dms",
              baseURL, result.Category, result.LatencyMs,
          ))
          c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
      }

      func performHealthProbe(baseURL, source string) productFlowTestResult {
          start := time.Now()
          fullURL := strings.TrimRight(baseURL, "/") + "/api/health/sso"
          req, _ := http.NewRequest(http.MethodGet, fullURL, nil)
          resp, err := productFlowTestHTTPClient.Do(req)
          latency := int(time.Since(start).Milliseconds())
          now := time.Now().Unix()
          if err != nil {
              return productFlowTestResult{
                  OK: false, Category: "network_error", LatencyMs: latency,
                  TestedAgainst: source, TestedAt: now,
                  Message: classifyTransportError(err),
              }
          }
          defer resp.Body.Close()
          if resp.StatusCode != http.StatusOK {
              return productFlowTestResult{
                  OK: false, Category: "application_error", LatencyMs: latency,
                  TestedAgainst: source, TestedAt: now,
                  Message: fmt.Sprintf("HTTP %d from ProductFlow", resp.StatusCode),
              }
          }
          // Parse body; require ok:true and supports_sso:true.
          var body struct {
              OK           bool   `json:"ok"`
              Version      string `json:"version"`
              SupportsSSO  bool   `json:"supports_sso"`
          }
          if err := common.DecodeJson(resp.Body, &body); err != nil {
              return productFlowTestResult{
                  OK: false, Category: "application_error", LatencyMs: latency,
                  TestedAgainst: source, TestedAt: now,
                  Message: "ProductFlow returned invalid health body",
              }
          }
          if !body.OK || !body.SupportsSSO {
              return productFlowTestResult{
                  OK: false, Category: "application_error", LatencyMs: latency,
                  TestedAgainst: source, TestedAt: now,
                  Message: "ProductFlow reports SSO not supported",
              }
          }
          return productFlowTestResult{
              OK: true, Category: "connected", LatencyMs: latency,
              TestedAgainst: source, TestedAt: now,
              Message: fmt.Sprintf("ProductFlow %s", body.Version),
          }
      }

      func classifyTransportError(err error) string {
          // Always log the raw cause for operator diagnosis; never return it to
          // the client (PRD R4: message must never carry raw exception text).
          common.SysError(fmt.Sprintf(
              "ProductFlow SSO test transport error: %v", err,
          ))

          if errors.Is(err, context.DeadlineExceeded) {
              return "Connection timed out after 3s"
          }
          var urlErr *url.Error
          if errors.As(err, &urlErr) {
              if urlErr.Timeout() {
                  return "Connection timed out after 3s"
              }
              // Map structural kinds to safe, fixed strings. The raw
              // urlErr.Err.Error() may include host, port, internal paths, and
              // OS-specific cause descriptions; those go to SysError above.
              if strings.Contains(strings.ToLower(urlErr.Err.Error()), "tls") ||
                  strings.Contains(strings.ToLower(urlErr.Err.Error()), "certificate") {
                  return "TLS certificate error"
              }
              if strings.Contains(strings.ToLower(urlErr.Err.Error()), "no such host") {
                  return "DNS lookup failed"
              }
              if strings.Contains(strings.ToLower(urlErr.Err.Error()), "connection refused") {
                  return "Connection refused"
              }
              return "Network error contacting ProductFlow"
          }
          return "Network error contacting ProductFlow"
      }

      func persistLastTestResult(result productFlowTestResult) {
          encoded, _ := common.Marshal(result)
          model.UpdateOption("productflow_sso.last_test_result", string(encoded))
      }

      func loadLastTestResult() *productFlowTestResult {
          common.OptionMapRWMutex.RLock()
          raw, ok := common.OptionMap["productflow_sso.last_test_result"]
          common.OptionMapRWMutex.RUnlock()
          if !ok || raw == "" {
              return nil
          }
          var out productFlowTestResult
          if err := common.UnmarshalJsonStr(raw, &out); err != nil {
              return nil
          }
          return &out
      }
      ```

#### R13 - SSO lifecycle audit

- [ ] In `controller/productflow_sso.go` `StartProductFlowSSO`, after the
      `cfg.validateForStart()` block but before `redirectProductFlowUser`,
      compute the ticket inside `redirectProductFlowUser` and log inside that
      function (since the ticket is generated there). Add to
      `redirectProductFlowUser`:

      ```go
      // After ticket generation and storeProductFlowTicket success:
      ticketHash := hex.EncodeToString(sha256.New().Sum([]byte(ticket)))[:16]
      // (use crypto/sha256 properly: hasher := sha256.New(); hasher.Write([]byte(ticket)); ticketHash := hex.EncodeToString(hasher.Sum(nil))[:16])
      model.RecordLog(user.Id, model.LogTypeSystem, fmt.Sprintf(
          "productflow_sso start: ticket=%s callback=%s",
          ticketHash, callbackURL,
      ))
      ```

- [ ] In failure branches of `StartProductFlowSSO`, before each error return:

      ```go
      // disabled
      model.RecordLog(0, model.LogTypeSystem, "productflow_sso start failed: reason=disabled")
      // not configured / invalid base_url
      model.RecordLog(0, model.LogTypeSystem, fmt.Sprintf("productflow_sso start failed: reason=%s", err.Error()))
      // user disabled
      model.RecordLog(userID, model.LogTypeSystem, "productflow_sso start failed: reason=user_disabled")
      ```

- [ ] In `VerifyProductFlowSSO` failure branches, log:

      ```go
      ticketHash := hashTicket(req.Ticket)  // hex first 16 chars
      model.RecordLog(0, model.LogTypeSystem, fmt.Sprintf(
          "productflow_sso verify failed: ticket=%s reason=%s",
          ticketHash, reason,
      ))
      ```

#### Router

- [ ] In `router/api-router.go` `optionRoute` group, add:

      ```go
      optionRoute.PUT("/batch", controller.UpdateOptionsBatch)
      ```

- [ ] Add a new productflow SSO admin group (RootAuth):

      ```go
      productFlowSSOAdmin := apiRouter.Group("/productflow/sso")
      productFlowSSOAdmin.Use(middleware.RootAuth())
      {
          productFlowSSOAdmin.GET("/status", controller.GetProductFlowSSOStatus)
          productFlowSSOAdmin.POST("/test", controller.TestProductFlowSSOConnection)
      }
      ```

### Validation

```bash
go test ./controller -run ProductFlow -count=1 -timeout 60s
go build ./...
```

New tests:

- [ ] `controller/option_batch_test.go`: batch with 3 valid keys commits all
      3 atomically + writes one `Log` row with 3 changes; batch with one
      invalid rolls back all.
- [ ] `controller/productflow_sso_status_test.go`: `Enabled=false`,
      `Configured=true`, `RedisEnabled=false` returns shape correctly.
- [ ] `controller/productflow_sso_test_endpoint_test.go`: mock HTTP server
      returning the three response classes; assert classifier output for each.
- [ ] `controller/productflow_sso_warn_test.go` (extend existing): assert
      INFO line appears when `enabled=false` with otherwise valid config.

## Phase 2: ProductFlow Health Probe (0.3 day, cross-repo)

### Files

- `ProductFlow/backend/src/productflow_backend/presentation/routes/health.py` (new or extend)
- `ProductFlow/backend/pyproject.toml` (add slowapi if not present)
- `ProductFlow/backend/src/productflow_backend/presentation/app.py` (wire limiter)

### Steps

- [ ] Confirm whether `slowapi` is already a dependency. If not, add
      `slowapi = "^0.1.9"` (or current stable) and run `uv lock`.
- [ ] In `presentation/app.py`, register the limiter:

      ```python
      from slowapi import Limiter, _rate_limit_exceeded_handler
      from slowapi.errors import RateLimitExceeded
      from slowapi.util import get_remote_address

      limiter = Limiter(key_func=get_remote_address)
      app.state.limiter = limiter
      app.add_exception_handler(RateLimitExceeded, _rate_limit_exceeded_handler)
      ```

- [ ] Create `routes/health.py`:

      ```python
      from fastapi import APIRouter, Request
      from productflow_backend.application.new_api_sso import get_sso_config
      from productflow_backend import __version__

      router = APIRouter()

      @router.get("/api/health/sso")
      async def health_sso(request: Request) -> dict:
          """SSO probe used by new-api Test Connection button.

          Public, unauthenticated. Rate-limited via slowapi to deter scanning.
          Returns minimum info needed for the new-api side to confirm reachability,
          version compatibility, and SSO module presence.
          """
          cfg = get_sso_config()
          return {
              "ok": True,
              "version": __version__,
              "supports_sso": bool(cfg.shared_secret),
          }
      ```

      Apply `@limiter.limit("6/minute")` decorator (slowapi requires it on
      the handler; verify the correct decorator import path matches the
      version pinned above).

- [ ] Register the router in the app:

      ```python
      from productflow_backend.presentation.routes import health
      app.include_router(health.router)
      ```

- [ ] If ProductFlow does not currently expose `__version__`, add it (either
      from `pyproject.toml` via `importlib.metadata` or hardcoded constant).

### Validation

```bash
cd ProductFlow/backend
./.venv/Scripts/python.exe -m pytest tests/test_health.py -q
./.venv/Scripts/python.exe -m pytest tests -q
./.venv/Scripts/python.exe -m ruff check src tests
```

New test:

- [ ] `tests/test_health.py`: assert `GET /api/health/sso` returns the
      documented schema; assert the 7th request from the same client returns
      429.

Manual:

- Start ProductFlow locally with `productflow_sso.shared_secret = "x"`,
  curl the endpoint, verify schema.
- Repeat 7 times in 60s, verify 7th gets 429.

## Phase 3: Front-end - status card + toggle + callback URL + banner + base_url warnings (1.0 day)

### Files

- `web/default/src/features/system-settings/integrations/productflow-sso-status-card.tsx` (new)
- `web/default/src/features/system-settings/integrations/productflow-sso-settings-section.tsx`
- `web/default/src/features/system-settings/api/productflow-sso.ts` (new)
- `web/default/src/features/system-settings/hooks/use-productflow-sso-status.ts` (new)
- `web/default/src/features/auth/lib/productflow-sso.ts` (extend with callback path constant + helper)
- `web/default/src/features/system-settings/operations/section-registry.tsx` (pass `enabled` and `last_test_result` into defaultValues)
- `web/default/src/features/system-settings/types.ts` (add `productflow_sso.enabled: boolean` and `productflow_sso.last_test_result: string` to `OperationsSettings`)
- `web/default/src/i18n/locales/zh.json` and `en.json` (additional keys)

### Steps

#### Constant + helper

- [ ] In `web/default/src/features/auth/lib/productflow-sso.ts`:

      ```ts
      export const PRODUCTFLOW_SSO_START_PATH = '/api/productflow/sso/start'
      export const PRODUCTFLOW_SSO_CALLBACK_PATH = '/auth/new-api/callback'

      export function isProductFlowSsoStartPath(value?: string): boolean {
        return value === PRODUCTFLOW_SSO_START_PATH
      }

      export function buildProductFlowCallbackURL(baseUrl: string): string {
        if (!baseUrl) return ''
        return baseUrl.replace(/\/+$/, '') + PRODUCTFLOW_SSO_CALLBACK_PATH
      }
      ```

#### API + hook

- [ ] `api/productflow-sso.ts`:

      ```ts
      import { request } from '@/lib/api-client'

      export type ProductFlowSSOTestResult = {
        ok: boolean
        category: 'connected' | 'network_error' | 'application_error' | 'other'
        latency_ms: number
        tested_against: 'draft' | 'saved'
        tested_at: number
        message: string
      }

      export type ProductFlowSSOStatus = {
        enabled: boolean
        configured: boolean
        redis_enabled: boolean
        callback_url_preview: string
        last_test_result: ProductFlowSSOTestResult | null
      }

      export async function fetchProductFlowSSOStatus(): Promise<ProductFlowSSOStatus> {
        const { data } = await request.get('/api/productflow/sso/status')
        return data
      }

      export async function testProductFlowSSOConnection(baseUrl?: string): Promise<ProductFlowSSOTestResult> {
        const body = baseUrl ? { base_url: baseUrl } : {}
        const { data } = await request.post('/api/productflow/sso/test', body)
        return data.data
      }
      ```

- [ ] `hooks/use-productflow-sso-status.ts`:

      ```ts
      import { useQuery } from '@tanstack/react-query'
      import { fetchProductFlowSSOStatus } from '../api/productflow-sso'

      export function useProductFlowSSOStatus() {
        return useQuery({
          queryKey: ['productflow-sso-status'],
          queryFn: fetchProductFlowSSOStatus,
          staleTime: 30_000,
          refetchOnWindowFocus: false,
        })
      }
      ```

#### Status card component

- [ ] `productflow-sso-status-card.tsx` accepts:

      ```ts
      interface ProductFlowSSOStatusCardProps {
        formEnabled: boolean
        onToggleEnabled: (next: boolean) => void
        baseUrlDraft: string
        isDirty: boolean
      }
      ```

- [ ] Render layout per the design in research/grill-me-2026-05-23.md
      "Q9 layout preview":
  - Switch + "ProductFlow SSO" title
  - Status chip: `connected` / `disabled` / `not configured` / `configuration error`
  - Callback URL preview row with copy button (uses
    `buildProductFlowCallbackURL(baseUrlDraft)`)
  - Redis warning chip if `status.redis_enabled === false` and
    `status.configured && formEnabled`
  - "Unsaved changes" badge if `isDirty`
  - Refresh button (icon-only `RefreshCw`) that
    `queryClient.invalidateQueries(['productflow-sso-status'])`

- [ ] Below the card, render the disabled banner if `formEnabled === false`:

      ```tsx
      <Alert variant="warning" className="mt-2">
        <AlertDescription>
          {t('SSO is disabled. Saved configuration will take effect on next enable.')}
        </AlertDescription>
      </Alert>
      ```

#### Wire form

- [ ] Add `productflow_sso.enabled` to the schema (zod):

      ```ts
      'productflow_sso.enabled': z.boolean()
      ```

- [ ] Add to default values and normalization. Persist via batch endpoint
      as string `"true"` / `"false"`.
- [ ] Render the status card above the existing fields:

      ```tsx
      <ProductFlowSSOStatusCard
        formEnabled={form.watch('productflow_sso.enabled')}
        onToggleEnabled={(v) => form.setValue('productflow_sso.enabled', v, { shouldDirty: true })}
        baseUrlDraft={form.watch('productflow_sso.base_url')}
        isDirty={form.formState.isDirty}
      />
      ```

#### base_url warnings (R11)

- [ ] Implement `classifyBaseUrl(url)` per PRD R11.
- [ ] Render warnings inside the base_url field's `FormDescription` block.

#### beforeunload

- [ ] In the section component, register:

      ```tsx
      useEffect(() => {
        if (!form.formState.isDirty) return
        const handler = (e: BeforeUnloadEvent) => {
          e.preventDefault()
          e.returnValue = ''
        }
        window.addEventListener('beforeunload', handler)
        return () => window.removeEventListener('beforeunload', handler)
      }, [form.formState.isDirty])
      ```

#### i18n additions

- [ ] Add to zh.json and en.json (mirror to fr/ja/ru/vi via sync):

      ```
      "ProductFlow SSO is enabled": ...
      "ProductFlow SSO is disabled": ...
      "Not configured": ...
      "Configuration incomplete": ...
      "Callback URL": ...
      "Copy": ...
      "Copied": ...
      "Refresh status": ...
      "Unsaved changes": ...
      "Redis not enabled (single-process fallback mode)": ...
      "SSO is disabled. Saved configuration will take effect on next enable.": ...
      "Recommend HTTPS for production": ...
      "Loopback detected, OK for dev, not for production": ...
      "Private IP detected, browser may not reach across origins": ...
      ```

### Validation

```bash
cd web/default
bun run typecheck
bun run lint
bun run i18n:sync
```

Manual: load page with and without `productflow_sso.enabled=true` server-side;
verify toggle reflects state; verify callback URL preview updates live as you
type; verify Copy button copies and toasts; verify Redis warning chip appears
correctly; verify the disabled banner appears only when toggle is off; verify
beforeunload triggers on tab close with unsaved changes.

## Phase 4: Front-end - test connection + batch save + secret confirm + TTL + strength meter (0.7 day)

### Files

- `web/default/src/features/system-settings/integrations/productflow-sso-settings-section.tsx`
- `web/default/src/features/system-settings/api/option.ts` (add batch update)
- `web/default/src/features/system-settings/hooks/use-update-productflow-sso-options.ts` (new)
- `web/default/src/features/system-settings/integrations/secret-strength-meter.tsx` (new)
- `web/default/src/i18n/locales/zh.json` and `en.json`

### Steps

#### Batch save mutation

- [ ] `api/option.ts` add:

      ```ts
      export async function updateOptionsBatch(updates: { key: string; value: string }[]) {
        const { data } = await request.put('/api/option/batch', { updates })
        return data
      }
      ```

- [ ] `hooks/use-update-productflow-sso-options.ts`:

      ```ts
      export function useUpdateProductFlowSSOOptions() {
        const queryClient = useQueryClient()
        return useMutation({
          mutationFn: (updates: { key: string; value: string }[]) => updateOptionsBatch(updates),
          onSuccess: (data) => {
            if (data.success) {
              queryClient.invalidateQueries({ queryKey: ['system-options'] })
              queryClient.invalidateQueries({ queryKey: ['productflow-sso-status'] })
              toast.success(i18next.t('ProductFlow settings saved'))
            } else {
              toast.error(data.message || i18next.t('Failed to save settings'))
            }
          },
          onError: (error: Error) => {
            toast.error(error.message || i18next.t('Failed to save settings'))
          },
        })
      }
      ```

- [ ] In `onSubmit`, replace the `for...await` loop with a single call:

      ```ts
      const updates = changedKeys.map((key) => ({ key, value: String(normalized[key]) }))
      await updateBatch.mutateAsync(updates)
      baselineRef.current = normalized
      form.reset(formDefaults)  // mark form not dirty after success
      ```

#### Test connection

- [ ] Add `useMutation` for `testProductFlowSSOConnection`.
- [ ] Render a `Test Connection` button next to Save:

      ```tsx
      <Button
        type="button"
        variant="outline"
        disabled={testMutation.isPending}
        onClick={() => testMutation.mutate(form.getValues('productflow_sso.base_url'))}
      >
        {testMutation.isPending ? t('Testing...') : t('Test Connection')}
      </Button>
      ```

- [ ] On success: toast with category + latency; the status card's
      `last_test_result` will refresh on next status fetch
      (`onSuccess` invalidates).

#### Secret strength meter

- [ ] `secret-strength-meter.tsx`:

      ```tsx
      function classifyStrength(value: string): 'weak' | 'medium' | 'strong' | null {
        if (!value) return null
        const len = value.length
        const hasLetters = /[A-Za-z]/.test(value)
        const hasDigits = /[0-9]/.test(value)
        const hasSymbols = /[^A-Za-z0-9]/.test(value)
        if (len < 16 || !(hasLetters && hasDigits)) return 'weak'
        if (len >= 24 && hasLetters && hasDigits && hasSymbols) return 'strong'
        return 'medium'
      }

      export function SecretStrengthMeter({ value }: { value: string }) {
        const strength = classifyStrength(value)
        if (!strength) return null
        const { t } = useTranslation()
        const colorMap = {
          weak: 'bg-red-500',
          medium: 'bg-amber-500',
          strong: 'bg-green-500',
        }
        const labelMap = {
          weak: t('Weak'),
          medium: t('Medium'),
          strong: t('Strong'),
        }
        return (
          <div className="mt-2 flex items-center gap-2 text-xs">
            <div className={cn('h-1.5 w-16 rounded', colorMap[strength])} />
            <span>{labelMap[strength]}</span>
          </div>
        )
      }
      ```

- [ ] Render below the secret field's `FormControl`.
- [ ] Update zod schema (R10):

      ```ts
      'productflow_sso.shared_secret': z.string().refine(
        (value) => value === '' || (value.length >= 16 && /[A-Za-z]/.test(value) && /[0-9]/.test(value)),
        t('Secret must be at least 16 chars with letters and digits'),
      )
      ```

#### Secret modification confirmation

- [ ] Use shadcn `AlertDialog`:

      ```tsx
      const [confirmOpen, setConfirmOpen] = useState(false)
      const pendingSubmitRef = useRef<ProductFlowSSOFormValues | null>(null)

      const onSubmit = (values: ProductFlowSSOFormValues) => {
        if (form.formState.dirtyFields['productflow_sso.shared_secret']) {
          pendingSubmitRef.current = values
          setConfirmOpen(true)
          return
        }
        return actuallySubmit(values)
      }

      const actuallySubmit = async (values: ProductFlowSSOFormValues) => {
        const normalized = normalizeFormValues(values)
        const changedKeys = computeChangedKeys(normalized, baselineRef.current)
        if (changedKeys.length === 0) {
          toast.info(t('No changes to save'))
          return
        }
        const updates = changedKeys.map((key) => ({ key, value: String(normalized[key]) }))
        await updateBatch.mutateAsync(updates)
        baselineRef.current = normalized
        form.reset(formDefaults)
      }
      ```

      AlertDialog content:

      ```tsx
      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Modify shared secret?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('This will immediately invalidate all current ProductFlow sessions. All logged-in users will need to re-authorize through New API.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => { pendingSubmitRef.current = null; setConfirmOpen(false) }}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction onClick={async () => {
              const v = pendingSubmitRef.current
              setConfirmOpen(false)
              if (v) await actuallySubmit(v)
            }}>
              {t('Yes, modify secret')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      ```

#### TTL human conversion

- [ ] Add `formatSecondsHuman(s, t)` per PRD R9.
- [ ] Render the result under each TTL field via `FormDescription`.
- [ ] Add warnings (both non-blocking, amber text under the field — see PRD R9):
  - `session_ttl_seconds > 2592000` (30 days) -> amber warning
    `Long session lifetimes increase abuse risk`
  - `ticket_ttl_seconds < 10` -> amber warning
    `Ticket TTL too short to complete SSO redirect`
  Save button stays enabled in both cases (consistent with `classifyBaseUrl`
  pattern from R11). Hard blocking is reserved for schema-level violations
  (negative or non-integer), which the existing zod refine already catches.

#### Save button visual

- [ ] Apply `animate-pulse` to Save button when `form.formState.isDirty`.
- [ ] Disable Save button when `!form.formState.isDirty || updateBatch.isPending`.

### Validation

```bash
cd web/default
bun run typecheck
bun run lint
```

Manual matrix:

- Edit `token_name` only, click Save: one toast, one network call, no
  secret confirm dialog.
- Edit `shared_secret`, click Save: AlertDialog opens, clicking Cancel
  preserves the form state with dirty intact.
- Edit `shared_secret`, click Save, click confirm: batch endpoint hits with
  the new secret, status card refreshes, dialog closes.
- Edit `session_ttl_seconds = 60`: helper shows `≈ 60 seconds`; change to
  `1209600`: helper shows `≈ 14 days`.
- Type secret `abc123`: strength shows Weak (red); type `Abc123def456ghi789jk`:
  Medium; add `!@#`: Strong.
- Click Test Connection with a draft URL different from saved: toast shows
  the draft result; status card chip updates to draft tested.

## Phase 5: Detail dialog + spec + minority i18n + e2e (0.5 day)

### Files

- `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`
- `web/default/src/features/usage-logs/types.ts`
- `.trellis/spec/backend/productflow-sso.md`
- `web/default/src/i18n/locales/*.json`

### Steps

#### R14 - generic manage changes table

- [ ] In `types.ts` extend the `admin_info` shape (line 108-111 area):

      ```ts
      // Manage audit fields (type=3, admin only)
      admin_username?: string
      admin_id?: number | string
      changes?: Array<{ key: string; before: string; after: string }>
      ```

- [ ] In `details-dialog.tsx`, after the existing `manageOperator` block,
      add inside the `isManage` rendering branch:

      ```tsx
      {isManage && Array.isArray(adminInfo?.changes) && adminInfo.changes.length > 0 ? (
        <div className="rounded-md border p-3">
          <Label className="text-xs font-medium">
            {t('Configuration changes')}
          </Label>
          <div className="mt-2 grid grid-cols-[1fr_auto_1fr] items-center gap-x-3 gap-y-1 text-xs">
            {adminInfo.changes.map((c) => (
              <Fragment key={c.key}>
                <div className="font-mono text-muted-foreground truncate">{c.key}</div>
                <div className="text-muted-foreground">→</div>
                <div className="font-mono truncate">
                  <span className="line-through text-muted-foreground">{c.before}</span>{' '}
                  <span>{c.after}</span>
                </div>
              </Fragment>
            ))}
          </div>
        </div>
      ) : null}
      ```

- [ ] Add `Configuration changes` to zh.json and en.json. Sync minority
      locales.

#### Spec updates (R15)

- [ ] `.trellis/spec/backend/productflow-sso.md`: add new sections:

  - **Configuration Surface**: enumerate the 7 keys (including new
    `productflow_sso.enabled`), types, defaults, validation rules, and the
    `productflow_sso.last_test_result` JSON envelope.
  - **Two-side Contracts** (locked, do not break):
    - Callback path: `/auth/new-api/callback`
    - Health probe: `GET /api/health/sso` returning `{ok, version, supports_sso}`,
      rate limited at `6/minute` per IP.
  - **Audit Logging**:
    - `LogTypeManage` writes carry `adminInfo.changes[] = [{key, before, after}]`
      (sensitive keys masked as `***` + sha8).
    - `LogTypeSystem` writes for SSO start success + all failures carry
      `ticket_hash` (16 hex chars).
    - SSO `verify success` is intentionally not in DB.
  - **Disabled mode**: `productflow_sso.enabled=false` returns 503 to start
    and silences the Redis-fallback WARN; emits one INFO line at startup if
    base_url + secret are otherwise configured.

- [ ] Update the existing "Deployment Modes" table to reflect the new flag.

#### Minority i18n sync

- [ ] From `web/default/`, run `bun run i18n:sync` once more. Inspect
      `_reports/` for any missing keys; the four minority locales should now
      all have English fallback values for the new keys.

#### e2e

- [ ] Run manually (no Playwright suite required):
  - Phase 0 happy path: save without touching secret keeps it intact.
  - Disable SSO: verify `StartProductFlowSSO` returns 503.
  - Re-enable: verify a real SSO flow completes end to end.
  - Modify secret: verify confirm dialog, verify currently-logged-in
    ProductFlow session becomes 401 on next backend call.
  - Test Connection against valid URL: connected chip.
  - Test Connection against `http://localhost:99` (closed port): network_error
    chip within ~3s.
  - Check `/usage-logs/common?type=3` for the SSO config change audit row;
    open detail dialog, verify changes table renders.
  - Check `/usage-logs/common?type=4` for SSO start success entries with
    ticket_hash.

### Validation

```bash
cd web/default
bun run typecheck
bun run lint
bun run i18n:sync
bun run build  # final build smoke
```

```bash
cd .. && go test ./controller -run ProductFlow -count=1 -timeout 60s
```

## Cross-cutting Risks

See `prd.md` Risk Points. Mitigations:

- Each phase carries its own validation gate; do not advance until the gate
  passes.
- Phase 0 ships independently to land the P0 bug fix as fast as possible.
- Phase 2 (ProductFlow side) can run in parallel with Phase 1 (new-api side)
  if two operators are available; otherwise Phase 1 first so the test
  connection endpoint can be validated end-to-end at the start of Phase 3.

## Current Status

- Not started.
- All architectural decisions sealed in `research/grill-me-2026-05-23.md`.
- Phase 0 is the recommended first commit; it provides immediate user-facing
  value (i18n + P0 bug fix) without depending on backend schema work.
