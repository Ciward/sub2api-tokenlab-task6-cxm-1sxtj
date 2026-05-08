package admin

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// OpenAIOAuthHandler handles OpenAI OAuth-related operations
type OpenAIOAuthHandler struct {
	openaiOAuthService *service.OpenAIOAuthService
	adminService       service.AdminService
}

func oauthPlatformFromPath(c *gin.Context) string {
	return service.PlatformOpenAI
}

func buildOpenAITemplateSummary(account *service.Account) OpenAITemplateSummary {
	if account == nil {
		return OpenAITemplateSummary{}
	}
	return OpenAITemplateSummary{
		ID:             account.ID,
		Name:           account.Name,
		Platform:       account.Platform,
		Type:           account.Type,
		ProxyID:        account.ProxyID,
		Concurrency:    account.Concurrency,
		Priority:       account.Priority,
		RateMultiplier: account.RateMultiplier,
		LoadFactor:     account.LoadFactor,
		GroupIDs:       append([]int64(nil), account.GroupIDs...),
	}
}

func normalizeOpenAITemplateAccount(account *service.Account) error {
	if account == nil {
		return infraerrors.NotFound("OPENAI_TEMPLATE_ACCOUNT_NOT_FOUND", "template account not found")
	}
	if account.Platform != service.PlatformOpenAI {
		return infraerrors.BadRequest("OPENAI_TEMPLATE_ACCOUNT_INVALID_PLATFORM", "template account must be an openai account")
	}
	return nil
}

func extractCodeAndState(code, state, callbackURL string) (string, string, error) {
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	callbackURL = strings.TrimSpace(callbackURL)
	if callbackURL != "" {
		u, err := url.Parse(callbackURL)
		if err != nil {
			return "", "", infraerrors.BadRequest("OPENAI_OAUTH_INVALID_CALLBACK_URL", "invalid callback_url")
		}
		if code == "" {
			code = strings.TrimSpace(u.Query().Get("code"))
		}
		if state == "" {
			state = strings.TrimSpace(u.Query().Get("state"))
		}
	}
	if code == "" {
		return "", "", infraerrors.BadRequest("OPENAI_OAUTH_CODE_REQUIRED", "code is required")
	}
	if state == "" {
		return "", "", infraerrors.BadRequest("OPENAI_OAUTH_STATE_REQUIRED", "state is required")
	}
	return code, state, nil
}

func cloneJSONMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	buf, err := json.Marshal(src)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil
	}
	return out
}

func buildOpenAITemplateCredentials(template *service.Account, tokenInfo *service.OpenAITokenInfo, oauthService *service.OpenAIOAuthService) map[string]any {
	creds := oauthService.BuildAccountCredentials(tokenInfo)
	if template == nil {
		return creds
	}
	for _, key := range []string{"model_mapping", "compact_model_mapping", "temp_unschedulable_enabled", "temp_unschedulable_rules"} {
		if v, ok := template.Credentials[key]; ok {
			creds[key] = v
		}
	}
	return cloneJSONMap(creds)
}

func buildOpenAITemplateExtra(template *service.Account, tokenInfo *service.OpenAITokenInfo) map[string]any {
	extra := cloneJSONMap(template.Extra)
	if extra == nil {
		extra = map[string]any{}
	}
	if tokenInfo != nil {
		if tokenInfo.Email != "" {
			extra["email"] = tokenInfo.Email
		}
		if tokenInfo.PrivacyMode != "" {
			extra["privacy_mode"] = tokenInfo.PrivacyMode
		}
	}
	for _, key := range []string{"openai_oauth_responses_websockets_v2_mode", "openai_oauth_responses_websockets_v2_enabled", "openai_passthrough", "openai_oauth_passthrough", "codex_cli_only", "openai_compact_mode"} {
		if template != nil && template.Extra != nil {
			if v, ok := template.Extra[key]; ok {
				extra[key] = v
			}
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

// NewOpenAIOAuthHandler creates a new OpenAI OAuth handler
func NewOpenAIOAuthHandler(openaiOAuthService *service.OpenAIOAuthService, adminService service.AdminService) *OpenAIOAuthHandler {
	return &OpenAIOAuthHandler{
		openaiOAuthService: openaiOAuthService,
		adminService:       adminService,
	}
}

// OpenAIGenerateAuthURLRequest represents the request for generating OpenAI auth URL
type OpenAIGenerateAuthURLRequest struct {
	ProxyID     *int64 `json:"proxy_id"`
	RedirectURI string `json:"redirect_uri"`
}

type OpenAITemplateSummary struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Platform       string   `json:"platform"`
	Type           string   `json:"type"`
	ProxyID        *int64   `json:"proxy_id,omitempty"`
	Concurrency    int      `json:"concurrency"`
	Priority       int      `json:"priority"`
	RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
	LoadFactor     *int     `json:"load_factor,omitempty"`
	GroupIDs       []int64  `json:"group_ids,omitempty"`
}

type OpenAITemplateAuthURLRequest struct {
	TemplateAccountID int64  `json:"template_account_id" binding:"required"`
	ProxyID           *int64 `json:"proxy_id"`
	RedirectURI       string `json:"redirect_uri"`
}

type OpenAITemplateAuthURLResult struct {
	AuthURL   string                `json:"auth_url"`
	SessionID string                `json:"session_id"`
	Template  OpenAITemplateSummary `json:"template"`
}

type OpenAICreateFromTemplateOAuthRequest struct {
	TemplateAccountID int64   `json:"template_account_id" binding:"required"`
	SessionID         string  `json:"session_id" binding:"required"`
	Code              string  `json:"code"`
	State             string  `json:"state"`
	CallbackURL       string  `json:"callback_url"`
	RedirectURI       string  `json:"redirect_uri"`
	ProxyID           *int64  `json:"proxy_id"`
	Name              string  `json:"name"`
	Notes             *string `json:"notes"`
}

// GenerateAuthURL generates OpenAI OAuth authorization URL
// POST /api/v1/admin/openai/generate-auth-url
func (h *OpenAIOAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req OpenAIGenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body
		req = OpenAIGenerateAuthURLRequest{}
	}

	result, err := h.openaiOAuthService.GenerateAuthURL(
		c.Request.Context(),
		req.ProxyID,
		req.RedirectURI,
		oauthPlatformFromPath(c),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// GenerateTemplateAuthURL generates an OpenAI OAuth URL while binding the flow to a template account.
// POST /api/v1/admin/openai/template-auth-url
func (h *OpenAIOAuthHandler) GenerateTemplateAuthURL(c *gin.Context) {
	var req OpenAITemplateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	template, err := h.adminService.GetAccount(c.Request.Context(), req.TemplateAccountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := normalizeOpenAITemplateAccount(template); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	effectiveProxyID := req.ProxyID
	if effectiveProxyID == nil {
		effectiveProxyID = template.ProxyID
	}

	result, err := h.openaiOAuthService.GenerateAuthURL(
		c.Request.Context(),
		effectiveProxyID,
		req.RedirectURI,
		oauthPlatformFromPath(c),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, &OpenAITemplateAuthURLResult{
		AuthURL:   result.AuthURL,
		SessionID: result.SessionID,
		Template:  buildOpenAITemplateSummary(template),
	})
}

// OpenAIExchangeCodeRequest represents the request for exchanging OpenAI auth code
type OpenAIExchangeCodeRequest struct {
	SessionID   string `json:"session_id" binding:"required"`
	Code        string `json:"code" binding:"required"`
	State       string `json:"state" binding:"required"`
	RedirectURI string `json:"redirect_uri"`
	ProxyID     *int64 `json:"proxy_id"`
}

// ExchangeCode exchanges OpenAI authorization code for tokens
// POST /api/v1/admin/openai/exchange-code
func (h *OpenAIOAuthHandler) ExchangeCode(c *gin.Context) {
	var req OpenAIExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.openaiOAuthService.ExchangeCode(c.Request.Context(), &service.OpenAIExchangeCodeInput{
		SessionID:   req.SessionID,
		Code:        req.Code,
		State:       req.State,
		RedirectURI: req.RedirectURI,
		ProxyID:     req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// OpenAIRefreshTokenRequest represents the request for refreshing OpenAI token
type OpenAIRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
	RT           string `json:"rt"`
	ClientID     string `json:"client_id"`
	ProxyID      *int64 `json:"proxy_id"`
}

// RefreshToken refreshes an OpenAI OAuth token
// POST /api/v1/admin/openai/refresh-token
func (h *OpenAIOAuthHandler) RefreshToken(c *gin.Context) {
	var req OpenAIRefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(req.RT)
	}
	if refreshToken == "" {
		response.BadRequest(c, "refresh_token is required")
		return
	}

	var proxyURL string
	if req.ProxyID != nil {
		proxy, err := h.adminService.GetProxy(c.Request.Context(), *req.ProxyID)
		if err == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	// 未指定 client_id 时，根据请求路径平台自动设置默认值，避免 repository 层盲猜
	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		platform := oauthPlatformFromPath(c)
		clientID, _ = openai.OAuthClientConfigByPlatform(platform)
	}

	tokenInfo, err := h.openaiOAuthService.RefreshTokenWithClientID(c.Request.Context(), refreshToken, proxyURL, clientID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// RefreshAccountToken refreshes token for a specific OpenAI account
// POST /api/v1/admin/openai/accounts/:id/refresh
func (h *OpenAIOAuthHandler) RefreshAccountToken(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	// Get account
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	platform := oauthPlatformFromPath(c)
	if account.Platform != platform {
		response.BadRequest(c, "Account platform does not match OAuth endpoint")
		return
	}

	// Only refresh OAuth-based accounts
	if !account.IsOAuth() {
		response.BadRequest(c, "Cannot refresh non-OAuth account credentials")
		return
	}

	// Use OpenAI OAuth service to refresh token
	tokenInfo, err := h.openaiOAuthService.RefreshAccountToken(c.Request.Context(), account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Build new credentials from token info
	newCredentials := h.openaiOAuthService.BuildAccountCredentials(tokenInfo)

	// Preserve non-token settings from existing credentials
	for k, v := range account.Credentials {
		if _, exists := newCredentials[k]; !exists {
			newCredentials[k] = v
		}
	}

	updatedAccount, err := h.adminService.UpdateAccount(c.Request.Context(), accountID, &service.UpdateAccountInput{
		Credentials: newCredentials,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.AccountFromService(updatedAccount))
}

// CreateAccountFromOAuth creates a new OpenAI OAuth account from token info
// POST /api/v1/admin/openai/create-from-oauth
func (h *OpenAIOAuthHandler) CreateAccountFromOAuth(c *gin.Context) {
	var req struct {
		SessionID   string  `json:"session_id" binding:"required"`
		Code        string  `json:"code" binding:"required"`
		State       string  `json:"state" binding:"required"`
		RedirectURI string  `json:"redirect_uri"`
		ProxyID     *int64  `json:"proxy_id"`
		Name        string  `json:"name"`
		Concurrency int     `json:"concurrency"`
		Priority    int     `json:"priority"`
		GroupIDs    []int64 `json:"group_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Exchange code for tokens
	tokenInfo, err := h.openaiOAuthService.ExchangeCode(c.Request.Context(), &service.OpenAIExchangeCodeInput{
		SessionID:   req.SessionID,
		Code:        req.Code,
		State:       req.State,
		RedirectURI: req.RedirectURI,
		ProxyID:     req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Build credentials from token info
	credentials := h.openaiOAuthService.BuildAccountCredentials(tokenInfo)

	platform := oauthPlatformFromPath(c)

	// Use email as default name if not provided
	name := req.Name
	if name == "" && tokenInfo.Email != "" {
		name = tokenInfo.Email
	}
	if name == "" {
		name = "OpenAI OAuth Account"
	}

	// Create account
	account, err := h.adminService.CreateAccount(c.Request.Context(), &service.CreateAccountInput{
		Name:        name,
		Platform:    platform,
		Type:        "oauth",
		Credentials: credentials,
		Extra:       nil,
		ProxyID:     req.ProxyID,
		Concurrency: req.Concurrency,
		Priority:    req.Priority,
		GroupIDs:    req.GroupIDs,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.AccountFromService(account))
}

// CreateAccountFromTemplateOAuth creates a new OpenAI OAuth account by copying strategy config from a template account.
// POST /api/v1/admin/openai/create-from-template-oauth
func (h *OpenAIOAuthHandler) CreateAccountFromTemplateOAuth(c *gin.Context) {
	var req OpenAICreateFromTemplateOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	template, err := h.adminService.GetAccount(c.Request.Context(), req.TemplateAccountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := normalizeOpenAITemplateAccount(template); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	code, state, err := extractCodeAndState(req.Code, req.State, req.CallbackURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	effectiveProxyID := req.ProxyID
	if effectiveProxyID == nil {
		effectiveProxyID = template.ProxyID
	}

	tokenInfo, err := h.openaiOAuthService.ExchangeCode(c.Request.Context(), &service.OpenAIExchangeCodeInput{
		SessionID:   req.SessionID,
		Code:        code,
		State:       state,
		RedirectURI: req.RedirectURI,
		ProxyID:     effectiveProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	credentials := buildOpenAITemplateCredentials(template, tokenInfo, h.openaiOAuthService)
	extra := buildOpenAITemplateExtra(template, tokenInfo)
	name := strings.TrimSpace(req.Name)
	if name == "" && strings.TrimSpace(tokenInfo.Email) != "" {
		name = strings.TrimSpace(tokenInfo.Email)
	}
	if name == "" {
		name = template.Name
	}
	if name == "" {
		name = "OpenAI OAuth Account"
	}

	var notes *string
	if req.Notes != nil {
		trimmed := strings.TrimSpace(*req.Notes)
		notes = &trimmed
	} else {
		notes = template.Notes
	}

	account, err := h.adminService.CreateAccount(c.Request.Context(), &service.CreateAccountInput{
		Name:               name,
		Notes:              notes,
		Platform:           service.PlatformOpenAI,
		Type:               service.AccountTypeOAuth,
		Credentials:        credentials,
		Extra:              extra,
		ProxyID:            effectiveProxyID,
		Concurrency:        template.Concurrency,
		Priority:           template.Priority,
		RateMultiplier:     template.RateMultiplier,
		LoadFactor:         template.LoadFactor,
		GroupIDs:           append([]int64(nil), template.GroupIDs...),
		AutoPauseOnExpired: &template.AutoPauseOnExpired,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.AccountFromService(account))
}

// SyncAccountMetadata syncs OpenAI OAuth account metadata using the existing access_token,
// without performing an OAuth refresh grant. Token fields are preserved.
// POST /api/v1/admin/openai/accounts/:id/sync-metadata
func (h *OpenAIOAuthHandler) SyncAccountMetadata(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	// Get account
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Use OpenAI OAuth service to sync metadata (read-only, no token refresh)
	result, err := h.openaiOAuthService.SyncAccountMetadata(c.Request.Context(), &service.SyncAccountMetadataInput{
		Account: account,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Update account with new credentials (preserves all other account fields)
	updatedAccount, err := h.adminService.UpdateAccount(c.Request.Context(), accountID, &service.UpdateAccountInput{
		Credentials: result.UpdatedCredentials,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.AccountFromService(updatedAccount))
}
