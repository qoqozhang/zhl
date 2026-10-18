package v1

import (
	"errors"

	jwtutil "iam-go/pkg/jwt"
	"iam-go/pkg/response"
	"iam-go/services"

	"github.com/gin-gonic/gin"
)

type DepartmentAPI struct {
	svc    *services.DepartmentService
	jwtMgr *jwtutil.Manager
}

func NewDepartmentAPI(svc *services.DepartmentService, jwtMgr *jwtutil.Manager) *DepartmentAPI {
	return &DepartmentAPI{svc: svc, jwtMgr: jwtMgr}
}

// Create POST /v1/departments（admin）
func (api *DepartmentAPI) Create(c *gin.Context) {
	var req services.DepartmentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	d, err := api.svc.Create(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDepartmentExists):
			response.BadRequest(c, "部门编码已存在")
		case errors.Is(err, services.ErrDepartmentNotFound):
			response.BadRequest(c, "父部门不存在")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.OK(c, d)
}

// Get GET /v1/departments/:id
func (api *DepartmentAPI) Get(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的部门 ID")
		return
	}

	d, err := api.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrDepartmentNotFound) {
			response.BadRequest(c, "部门不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, d)
}

// List GET /v1/departments
func (api *DepartmentAPI) List(c *gin.Context) {
	var params services.DepartmentListParams
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

// Update PUT /v1/departments/:id（admin）
func (api *DepartmentAPI) Update(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的部门 ID")
		return
	}

	var req services.DepartmentUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	d, err := api.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDepartmentNotFound):
			response.BadRequest(c, "部门不存在")
		case errors.Is(err, services.ErrDepartmentExists):
			response.BadRequest(c, "部门编码已存在")
		case errors.Is(err, services.ErrDepartmentParentCycle):
			response.BadRequest(c, "父级不能是自身或其后代")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.OK(c, d)
}

// Delete DELETE /v1/departments/:id（admin）
func (api *DepartmentAPI) Delete(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的部门 ID")
		return
	}

	if err := api.svc.Delete(c.Request.Context(), id); err != nil {
		switch {
		case errors.Is(err, services.ErrDepartmentNotFound):
			response.BadRequest(c, "部门不存在")
		case errors.Is(err, services.ErrDepartmentHasChildren):
			response.BadRequest(c, "该部门存在子部门，无法删除")
		case errors.Is(err, services.ErrDepartmentHasUsers):
			response.BadRequest(c, "该部门下存在用户，无法删除")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.OK(c, nil)
}