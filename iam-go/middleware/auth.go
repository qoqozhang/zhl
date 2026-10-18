package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	jwtutil "iam-go/pkg/jwt"
	"iam-go/pkg/session"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserKey   = "current_user"
	CtxSessionID = "session_id"

	SessionCookieName = "iam_session"
)

type AuthClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (a *AuthClaims) toAccessClaims() *jwtutil.AccessClaims {
	return &jwtutil.AccessClaims{
		UserID:   a.UserID,
		Username: a.Username,
		Role:     a.Role,
	}
}

func claimsFromAccess(ac *jwtutil.AccessClaims) *AuthClaims {
	return &AuthClaims{
		UserID:   ac.UserID,
		Username: ac.Username,
		Role:     ac.Role,
	}
}

func claimsFromSession(sd *session.SessionData) *AuthClaims {
	return &AuthClaims{
		UserID:   sd.UserID,
		Username: sd.Username,
		Role:     sd.Role,
	}
}

// AuthMiddleware 统一认证中间件：Session Cookie 优先，JWT Bearer 兜底
func AuthMiddleware(jwtMgr *jwtutil.Manager, sessionStore *session.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := resolveAuth(c, jwtMgr, sessionStore)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 40100, "message": "unauthorized: " + err.Error(),
			})
			return
		}
		c.Set(CtxUserKey, claims)
		c.Next()
	}
}

// AuthMiddlewareOptional 可选认证：有凭证就解析，没有也放行
func AuthMiddlewareOptional(jwtMgr *jwtutil.Manager, sessionStore *session.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, _ := resolveAuth(c, jwtMgr, sessionStore)
		if claims != nil {
			c.Set(CtxUserKey, claims)
		}
		c.Next()
	}
}

func resolveAuth(c *gin.Context, jwtMgr *jwtutil.Manager, sessionStore *session.Store) (*AuthClaims, error) {
	if sessionStore != nil {
		sid, err := c.Cookie(SessionCookieName)
		if err == nil && sid != "" {
			data, err := sessionStore.Get(c.Request.Context(), sid)
			if err == nil && data != nil {
				_ = sessionStore.Touch(c.Request.Context(), sid)
				c.Set(CtxSessionID, sid)
				return claimsFromSession(data), nil
			}
		}
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			ac, err := jwtMgr.ParseAccess(parts[1])
			if err == nil {
				return claimsFromAccess(ac), nil
			}
		}
	}

	return nil, errors.New("no valid session or bearer token")
}

// JWTAuth 仅 JWT 认证（保留用于纯 API 调用场景）
func JWTAuth(jwtMgr *jwtutil.Manager) gin.HandlerFunc {
	return AuthMiddleware(jwtMgr, nil)
}

// RequireRole 角色校验中间件（必须先过 AuthMiddleware / JWTAuth）
func RequireRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(c *gin.Context) {
		val, exists := c.Get(CtxUserKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 40100, "message": "unauthorized",
			})
			return
		}
		claims, ok := val.(*AuthClaims)
		if !ok || !roleSet[claims.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": 40300, "message": "forbidden: insufficient role",
			})
			return
		}
		c.Next()
	}
}

// GetCurrentUser 从 gin.Context 取出当前登录用户 AuthClaims
func GetCurrentUser(c *gin.Context) *AuthClaims {
	val, exists := c.Get(CtxUserKey)
	if !exists {
		return nil
	}
	claims, _ := val.(*AuthClaims)
	return claims
}

// GetSessionID 从 gin.Context 取出当前 Session ID（可能为空，表示走的是 JWT）
func GetSessionID(c *gin.Context) string {
	val, exists := c.Get(CtxSessionID)
	if !exists {
		return ""
	}
	sid, _ := val.(string)
	return sid
}

// SetSessionCookie 向响应写 Session Cookie
func SetSessionCookie(c *gin.Context, sessionID string, secure bool) {
	c.SetCookie(
		SessionCookieName,
		sessionID,
		int((24*time.Hour).Seconds()),
		"/",
		"",
		secure,
		true,
	)
}

// ClearSessionCookie 清除 Session Cookie
func ClearSessionCookie(c *gin.Context) {
	c.SetCookie(
		SessionCookieName,
		"",
		-1,
		"/",
		"",
		false,
		true,
	)
}