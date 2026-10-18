package v1

import (
	"errors"

	jwtutil "iam-go/pkg/jwt"
	"iam-go/pkg/response"
	"iam-go/services"

	"github.com/gin-gonic/gin"
)

type RoleAPI struct {
	svc     *services.RoleService
	rpSvc   *services.RolePermissionService
	jwtMgr  *jwtutil.Manager
}

func NewRoleAPI(svc *services.RoleService, rpSvc *services.RolePermissionService, jwtMgr *jwtutil.Manager) *RoleAPI {
	return &RoleAPI{svc: svc, rpSvc: rpSvc, jwtMgr: jwtMgr}
}

func (api *RoleAPI) Create(c *gin.Context) {
	var req services.RoleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	r, err := api.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrRoleExists) {
			response.BadRequest(c, "角色编码已存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, r)
}

func (api *RoleAPI) Get(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	r, err := api.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrRoleNotFound) {
			response.BadRequest(c, "角色不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	permKeys, _ := api.rpSvc.GetPermissionKeysForRole(c.Request.Context(), id)

	response.OK(c, gin.H{
		"role":          r,
		"permission_keys": permKeys,
	})
}

func (api *RoleAPI) List(c *gin.Context) {
	var params services.RoleListParams
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

func (api *RoleAPI) Update(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	var req services.RoleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	r, err := api.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrRoleNotFound):
			response.BadRequest(c, "角色不存在")
		case errors.Is(err, services.ErrRoleExists):
			response.BadRequest(c, "角色编码已存在")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.OK(c, r)
}

func (api *RoleAPI) Delete(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	if err := api.svc.Delete(c.Request.Context(), id); err != nil {
		switch {
		case errors.Is(err, services.ErrRoleNotFound):
			response.BadRequest(c, "角色不存在")
		case errors.Is(err, services.ErrRoleSystemDelete):
			response.BadRequest(c, "系统内置角色不可删除")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.OK(c, nil)
}

// UpdatePermissions PUT /v1/roles/:id/permissions（admin）
// 用 permission_keys 全量替换角色权限
func (api *RoleAPI) UpdatePermissions(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	var req struct {
		PermissionKeys []string `json:"permission_keys" binding:"omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if _, err := api.svc.GetByID(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrRoleNotFound) {
			response.BadRequest(c, "角色不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	if err := api.rpSvc.ReplaceByKeys(c.Request.Context(), id, req.PermissionKeys); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	keys, _ := api.rpSvc.GetPermissionKeysForRole(c.Request.Context(), id)
	response.OK(c, gin.H{"permission_keys": keys})
}