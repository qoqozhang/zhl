package v1

import (
	"errors"
	"net/http"
	"strconv"

	"iam-go/middleware"
	jwtutil "iam-go/pkg/jwt"
	"iam-go/pkg/response"
	"iam-go/pkg/session"
	"iam-go/services"

	"github.com/gin-gonic/gin"
)

type UserAPI struct {
	svc          *services.UserService
	jwtMgr       *jwtutil.Manager
	sessionStore *session.Store
}

func NewUserAPI(svc *services.UserService, jwtMgr *jwtutil.Manager, sessionStore *session.Store) *UserAPI {
	return &UserAPI{svc: svc, jwtMgr: jwtMgr, sessionStore: sessionStore}
}

// Create POST /v1/users
func (api *UserAPI) Create(c *gin.Context) {
	var req services.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	u, err := api.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrUserExists) {
			response.BadRequest(c, "用户名/邮箱/手机号已存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, u)
}

// Login POST /v1/users/login
func (api *UserAPI) Login(c *gin.Context) {
	var req services.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	req.ClientIP = c.ClientIP()

	u, roleKeys, err := api.svc.Login(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			response.BadRequest(c, "用户名或密码错误")
		case errors.Is(err, services.ErrUserDisabled):
			response.Forbidden(c, "账号已被禁用")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}

	role := services.PickHighestRole(roleKeys)

	if api.sessionStore != nil {
		sessionID, err := api.sessionStore.Create(c.Request.Context(), session.SessionData{
			UserID:   u.ID,
			Username: u.Username,
			Role:     role,
		})
		if err != nil {
			response.InternalError(c, "创建 session 失败: "+err.Error())
			return
		}
		middleware.SetSessionCookie(c, sessionID, false)
	}

	accessToken, err := api.jwtMgr.GenerateAccess(u.ID, u.Username, role)
	if err != nil {
		response.InternalError(c, "生成 access token 失败")
		return
	}

	refreshToken := ""
	if api.jwtMgr != nil {
		if rt, _, err := api.jwtMgr.GenerateRefresh(c.Request.Context(), u.ID); err != nil {
			response.InternalError(c, "生成 refresh token 失败: "+err.Error())
			return
		} else {
			refreshToken = rt
		}
	}

	response.OK(c, gin.H{
		"user":          u,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	})
}

// RefreshRequest 换 Token 请求
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh POST /v1/users/refresh
func (api *UserAPI) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	claims, err := api.jwtMgr.ParseRefresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.BadRequest(c, "refresh token 无效或已失效")
		return
	}

	u, err := api.svc.GetByID(c.Request.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			response.BadRequest(c, "用户不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	if u.Status != 1 {
		response.Forbidden(c, "账号已被禁用")
		return
	}

	api.jwtMgr.RevokeRefresh(c.Request.Context(), claims.ID)

	_, roleKeys, _ := api.svc.GetWithRoleKeys(c.Request.Context(), claims.UserID)
	role := services.PickHighestRole(roleKeys)

	accessToken, err := api.jwtMgr.GenerateAccess(u.ID, u.Username, role)
	if err != nil {
		response.InternalError(c, "生成 access token 失败")
		return
	}

	newRefreshToken := ""
	if rt, _, err := api.jwtMgr.GenerateRefresh(c.Request.Context(), u.ID); err == nil {
		newRefreshToken = rt
	}

	response.OK(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
		"token_type":    "Bearer",
	})
}

// Logout POST /v1/users/logout
func (api *UserAPI) Logout(c *gin.Context) {
	if api.sessionStore != nil {
		if sid := middleware.GetSessionID(c); sid != "" {
			_ = api.sessionStore.Delete(c.Request.Context(), sid)
		}
		middleware.ClearSessionCookie(c)
	}

	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.OK(c, gin.H{"message": "logged out"})
		return
	}

	claims, err := api.jwtMgr.ParseRefresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.OK(c, gin.H{"message": "logged out"})
		return
	}

	if err := api.jwtMgr.RevokeRefresh(c.Request.Context(), claims.ID); err != nil {
		response.InternalError(c, "撤销 refresh token 失败")
		return
	}

	response.OK(c, gin.H{"message": "logged out"})
}

// Me GET /v1/users/me
func (api *UserAPI) Me(c *gin.Context) {
	claims := middleware.GetCurrentUser(c)
	if claims == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code": 40100, "message": "unauthorized",
		})
		return
	}

	u, roleKeys, err := api.svc.GetWithRoleKeys(c.Request.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			response.BadRequest(c, "用户不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{
		"user":      u,
		"role_keys": roleKeys,
	})
}

// Get GET /v1/users/:id
func (api *UserAPI) Get(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return
	}

	u, err := api.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			response.BadRequest(c, "用户不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, u)
}

// List GET /v1/users
func (api *UserAPI) List(c *gin.Context) {
	var params services.ListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	result, err := api.svc.List(c.Request.Context(), &params)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// Update PUT /v1/users/:id
func (api *UserAPI) Update(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return
	}

	var req services.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	u, err := api.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			response.BadRequest(c, "用户不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, u)
}

// Delete DELETE /v1/users/:id
func (api *UserAPI) Delete(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return
	}

	if err := api.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			response.BadRequest(c, "用户不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

func parseUintID(s string) (uint, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}