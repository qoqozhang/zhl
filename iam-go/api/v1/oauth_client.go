package v1

import (
	"errors"

	"iam-go/pkg/response"
	"iam-go/services"

	"github.com/gin-gonic/gin"
)

type OAuthClientAPI struct {
	svc *services.OAuthClientService
}

func NewOAuthClientAPI(svc *services.OAuthClientService) *OAuthClientAPI {
	return &OAuthClientAPI{svc: svc}
}

func (api *OAuthClientAPI) Create(c *gin.Context) {
	var req services.OAuthClientCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	r, err := api.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrOAuthClientExists) {
			response.BadRequest(c, "client_id 已存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, r)
}

func (api *OAuthClientAPI) Get(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的客户端 ID")
		return
	}

	r, err := api.svc.GetByIDByNum(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrOAuthClientNotFound) {
			response.BadRequest(c, "客户端不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, r)
}

func (api *OAuthClientAPI) List(c *gin.Context) {
	var params services.OAuthClientListParams
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

func (api *OAuthClientAPI) Update(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的客户端 ID")
		return
	}

	var req services.OAuthClientUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	r, err := api.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, services.ErrOAuthClientNotFound) {
			response.BadRequest(c, "客户端不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, r)
}

func (api *OAuthClientAPI) Delete(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的客户端 ID")
		return
	}

	if err := api.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrOAuthClientNotFound) {
			response.BadRequest(c, "客户端不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

// ResetSecret POST /v1/oauth/clients/:id/reset-secret（admin）
// 重置 client_secret，返回新明文密钥（只显示一次）
func (api *OAuthClientAPI) ResetSecret(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的客户端 ID")
		return
	}

	client, err := api.svc.GetByIDByNum(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrOAuthClientNotFound) {
			response.BadRequest(c, "客户端不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	secret, err := api.svc.ResetSecret(c.Request.Context(), client.ClientID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{
		"client_id":     client.ClientID,
		"client_name":   client.ClientName,
		"client_secret": secret,
		"message":       "请妥善保管，明文仅显示一次",
	})
}