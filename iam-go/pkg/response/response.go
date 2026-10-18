package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// JSON 统一响应结构
type JSON struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

func write(c *gin.Context, httpStatus, code int, msg string, data interface{}) {
	c.JSON(httpStatus, JSON{
		Code:      code,
		Message:   msg,
		Data:      data,
		RequestID: c.GetHeader("X-Request-ID"),
	})
}

func OK(c *gin.Context, data interface{}) {
	write(c, http.StatusOK, 0, "ok", data)
}

func BadRequest(c *gin.Context, msg string) {
	write(c, http.StatusBadRequest, 40000, msg, nil)
}

func Unauthorized(c *gin.Context, msg string) {
	write(c, http.StatusUnauthorized, 40100, msg, nil)
}

func Forbidden(c *gin.Context, msg string) {
	write(c, http.StatusForbidden, 40300, msg, nil)
}

func InternalError(c *gin.Context, msg string) {
	write(c, http.StatusInternalServerError, 50000, msg, nil)
}
