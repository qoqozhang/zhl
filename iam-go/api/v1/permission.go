package v1

import (
	"errors"

	jwtutil "iam-go/pkg/jwt"
	"iam-go/pkg/response"
	"iam-go/services"

	"github.com/gin-gonic/gin"
)

type PermissionAPI struct {
	svc    *services.PermissionService
	jwtMgr *jwtutil.Manager
}

func NewPermissionAPI(svc *services.PermissionService, jwtMgr *jwtutil.Manager) *PermissionAPI {
	return &PermissionAPI{svc: svc, jwtMgr: jwtMgr}
}

func (api *PermissionAPI) Create(c *gin.Context) {
	var req services.PermissionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	p, err := api.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrPermissionExists) {
			response.BadRequest(c, "权限标识已存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, p)
}

func (api *PermissionAPI) Get(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的权限 ID")
		return
	}

	p, err := api.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrPermissionNotFound) {
			response.BadRequest(c, "权限不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, p)
}

func (api *PermissionAPI) List(c *gin.Context) {
	var params services.PermissionListParams
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

func (api *PermissionAPI) Update(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的权限 ID")
		return
	}

	var req services.PermissionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	p, err := api.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPermissionNotFound):
			response.BadRequest(c, "权限不存在")
		case errors.Is(err, services.ErrPermissionExists):
			response.BadRequest(c, "权限标识已存在")
		case errors.Is(err, services.ErrPermissionParentCycle):
			response.BadRequest(c, "父级不能是自身或其后代")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.OK(c, p)
}

func (api *PermissionAPI) Delete(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的权限 ID")
		return
	}

	if err := api.svc.Delete(c.Request.Context(), id); err != nil {
		switch {
		case errors.Is(err, services.ErrPermissionNotFound):
			response.BadRequest(c, "权限不存在")
		case errors.Is(err, services.ErrPermissionHasChildren):
			response.BadRequest(c, "存在子权限，无法删除")
		case errors.Is(err, services.ErrPermissionInUse):
			response.BadRequest(c, "权限已分配给角色，无法删除")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.OK(c, nil)
}