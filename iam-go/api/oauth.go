package api

import (
	"errors"
	"net/http"
	"strings"

	jwtutil "iam-go/pkg/jwt"
	"iam-go/pkg/response"
	"iam-go/pkg/session"
	"iam-go/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OAuthAPI struct {
	clientSvc    *services.OAuthClientService
	codeSvc      *services.OAuthCodeService
	userSvc      *services.UserService
	jwtMgr       *jwtutil.Manager
	sessionStore *session.Store
}

func NewOAuthAPI(db *gorm.DB, jwtMgr *jwtutil.Manager, sessionStore *session.Store) *OAuthAPI {
	return &OAuthAPI{
		clientSvc:    services.NewOAuthClientService(db),
		codeSvc:      services.NewOAuthCodeService(db),
		userSvc:      services.NewUserService(db),
		jwtMgr:       jwtMgr,
		sessionStore: sessionStore,
	}
}

// Authorize GET /oauth2/authorize
// 重定向到登录页（前端实现）或直接展示授权页
// 由于 OAuth 2.0 标准要求 HTTP 302 重定向 + HTML 同意页，后端这里只做参数校验
// 实际登录 Session + 同意页建议由前端配合实现
// 这里提供一个简化版：如果已带 access_token（Cookie 或 Header），直接生成 code 并重定向
func (api *OAuthAPI) Authorize(c *gin.Context) {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	responseType := c.Query("response_type")
	scope := c.Query("scope")
	state := c.Query("state")

	if clientID == "" {
		oauthError(c, http.StatusBadRequest, "invalid_request", "missing client_id", state)
		return
	}
	if redirectURI == "" {
		oauthError(c, http.StatusBadRequest, "invalid_request", "missing redirect_uri", state)
		return
	}
	if responseType != "code" {
		oauthError(c, http.StatusBadRequest, "unsupported_response_type", "only code is supported", state)
		return
	}

	client, err := api.clientSvc.GetByID(c.Request.Context(), clientID)
	if err != nil {
		if errors.Is(err, services.ErrOAuthClientNotFound) {
			oauthError(c, http.StatusBadRequest, "invalid_client", "client not found", state)
		} else if errors.Is(err, services.ErrOAuthClientDisabled) {
			oauthError(c, http.StatusBadRequest, "invalid_client", "client disabled", state)
		} else {
			oauthError(c, http.StatusInternalServerError, "server_error", err.Error(), state)
		}
		return
	}

	if err := api.clientSvc.ValidateRedirectURI(client, redirectURI); err != nil {
		oauthError(c, http.StatusBadRequest, "invalid_request", "redirect_uri not registered", state)
		return
	}

	userID := uint(0)

	if api.sessionStore != nil {
		sid, err := c.Cookie("iam_session")
		if err == nil && sid != "" {
			if data, err := api.sessionStore.Get(c.Request.Context(), sid); err == nil && data != nil {
				userID = data.UserID
			}
		}
	}

	if userID == 0 {
		claims, _ := api.parseAccessToken(c)
		if claims != nil {
			userID = claims.UserID
		}
	}

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    40100,
			"message": "login required, POST /v1/users/login first (creates session cookie)",
		})
		return
	}

	if scope == "" {
		scope = client.Scopes
	}

	code, err := api.codeSvc.Create(c.Request.Context(), clientID, userID, redirectURI, scope)
	if err != nil {
		oauthError(c, http.StatusInternalServerError, "server_error", "failed to generate code", state)
		return
	}

	separator := "?"
	if strings.Contains(redirectURI, "?") {
		separator = "&"
	}
	loc := redirectURI + separator + "code=" + code
	if state != "" {
		loc += "&state=" + state
	}
	c.Redirect(http.StatusFound, loc)
}

// Token POST /oauth2/token
// grant_type: authorization_code / refresh_token
func (api *OAuthAPI) Token(c *gin.Context) {
	grantType := c.PostForm("grant_type")

	switch grantType {
	case "authorization_code":
		api.handleCodeExchange(c)
	case "refresh_token":
		api.handleRefreshToken(c)
	case "client_credentials":
		api.handleClientCredentials(c)
	default:
		oauthTokenError(c, http.StatusBadRequest, "unsupported_grant_type", "")
	}
}

func (api *OAuthAPI) handleCodeExchange(c *gin.Context) {
	code := c.PostForm("code")
	redirectURI := c.PostForm("redirect_uri")
	clientID := c.PostForm("client_id")
	clientSecret := c.PostForm("client_secret")
	codeVerifier := c.PostForm("code_verifier")

	if code == "" {
		oauthTokenError(c, http.StatusBadRequest, "invalid_request", "missing code")
		return
	}
	if redirectURI == "" {
		oauthTokenError(c, http.StatusBadRequest, "invalid_request", "missing redirect_uri")
		return
	}

	if clientID == "" {
		clientID = c.Query("client_id")
	}
	if clientSecret == "" {
		clientSecret = c.Query("client_secret")
	}

	if clientID == "" || clientSecret == "" {
		oauthTokenError(c, http.StatusBadRequest, "invalid_client", "missing client credentials")
		return
	}

	client, err := api.clientSvc.Authenticate(c.Request.Context(), clientID, clientSecret)
	if err != nil {
		oauthTokenError(c, http.StatusUnauthorized, "invalid_client", "invalid client credentials")
		return
	}
	_ = client

	authCode, err := api.codeSvc.Consume(c.Request.Context(), code, clientID, redirectURI)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrOAuthCodeNotFound),
			errors.Is(err, services.ErrOAuthCodeMismatch):
			oauthTokenError(c, http.StatusBadRequest, "invalid_grant", "code not found or mismatch")
		case errors.Is(err, services.ErrOAuthCodeUsed):
			oauthTokenError(c, http.StatusBadRequest, "invalid_grant", "code already used")
		case errors.Is(err, services.ErrOAuthCodeExpired):
			oauthTokenError(c, http.StatusBadRequest, "invalid_grant", "code expired")
		default:
			oauthTokenError(c, http.StatusInternalServerError, "server_error", err.Error())
		}
		return
	}

	if !services.VerifyPKCE(codeVerifier, c.Query("code_challenge"), c.Query("code_challenge_method")) {
		oauthTokenError(c, http.StatusBadRequest, "invalid_grant", "pkce verification failed")
		return
	}

	u, roleKeys, err := api.userSvc.GetWithRoleKeys(c.Request.Context(), authCode.UserID)
	if err != nil {
		oauthTokenError(c, http.StatusInternalServerError, "server_error", "user not found")
		return
	}
	role := services.PickHighestRole(roleKeys)

	accessToken, err := api.jwtMgr.GenerateAccessWithOAuth(u.ID, u.Username, role, clientID, authCode.Scopes)
	if err != nil {
		oauthTokenError(c, http.StatusInternalServerError, "server_error", "failed to generate access_token")
		return
	}

	refreshToken, _, err := api.jwtMgr.GenerateRefresh(c.Request.Context(), u.ID)
	if err != nil {
		oauthTokenError(c, http.StatusInternalServerError, "server_error", "failed to generate refresh_token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    1800,
		"refresh_token": refreshToken,
		"scope":         authCode.Scopes,
	})
}

func (api *OAuthAPI) handleRefreshToken(c *gin.Context) {
	refreshToken := c.PostForm("refresh_token")
	clientID := c.PostForm("client_id")
	clientSecret := c.PostForm("client_secret")

	if refreshToken == "" {
		oauthTokenError(c, http.StatusBadRequest, "invalid_request", "missing refresh_token")
		return
	}

	claims, err := api.jwtMgr.ParseRefresh(c.Request.Context(), refreshToken)
	if err != nil {
		oauthTokenError(c, http.StatusBadRequest, "invalid_grant", "invalid refresh_token")
		return
	}

	if clientID != "" && clientSecret != "" {
		if _, err := api.clientSvc.Authenticate(c.Request.Context(), clientID, clientSecret); err != nil {
			oauthTokenError(c, http.StatusUnauthorized, "invalid_client", "invalid client credentials")
			return
		}
	}

	u, roleKeys, err := api.userSvc.GetWithRoleKeys(c.Request.Context(), claims.UserID)
	if err != nil {
		oauthTokenError(c, http.StatusInternalServerError, "server_error", "user not found")
		return
	}
	role := services.PickHighestRole(roleKeys)

	accessToken, err := api.jwtMgr.GenerateAccess(u.ID, u.Username, role)
	if err != nil {
		oauthTokenError(c, http.StatusInternalServerError, "server_error", "failed to generate access_token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   1800,
	})
}

func (api *OAuthAPI) handleClientCredentials(c *gin.Context) {
	clientID := c.PostForm("client_id")
	clientSecret := c.PostForm("client_secret")

	client, err := api.clientSvc.Authenticate(c.Request.Context(), clientID, clientSecret)
	if err != nil {
		oauthTokenError(c, http.StatusUnauthorized, "invalid_client", "invalid client credentials")
		return
	}

	token, err := api.jwtMgr.GenerateAccess(0, client.ClientID, "client")
	if err != nil {
		oauthTokenError(c, http.StatusInternalServerError, "server_error", "failed to generate access_token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   1800,
		"scope":        client.Scopes,
	})
}

// UserInfo GET /oauth2/userinfo
// 返回当前 access_token 对应的用户信息（OIDC 标准端点）
func (api *OAuthAPI) UserInfo(c *gin.Context) {
	claims, _ := api.parseAccessToken(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40100, "message": "unauthorized"})
		return
	}

	u, roleKeys, err := api.userSvc.GetWithRoleKeys(c.Request.Context(), claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50000, "message": "user not found"})
		return
	}

	response.OK(c, gin.H{
		"user_id":     u.ID,
		"username":    u.Username,
		"real_name":   u.RealName,
		"email":       u.Email,
		"phone":       u.Phone,
		"avatar_url":  u.AvatarURL,
		"role_keys":   roleKeys,
		"client_id":   claims.ClientID,
		"scope":       claims.Scope,
	})
}

func oauthError(c *gin.Context, httpStatus int, errCode, errDesc, state string) {
	loc := c.Query("redirect_uri")
	if loc != "" {
		separator := "?"
		if strings.Contains(loc, "?") {
			separator = "&"
		}
		loc += separator + "error=" + errCode + "&error_description=" + errDesc
		if state != "" {
			loc += "&state=" + state
		}
		c.Redirect(http.StatusFound, loc)
		return
	}
	c.JSON(httpStatus, gin.H{
		"error":             errCode,
		"error_description": errDesc,
	})
}

func oauthTokenError(c *gin.Context, httpStatus int, errCode, errDesc string) {
	c.JSON(httpStatus, gin.H{
		"error":             errCode,
		"error_description": errDesc,
	})
}

func (api *OAuthAPI) parseAccessToken(c *gin.Context) (*jwtutil.AccessClaims, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil, errors.New("no auth header")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errors.New("invalid auth format")
	}
	return api.jwtMgr.ParseAccess(parts[1])
}