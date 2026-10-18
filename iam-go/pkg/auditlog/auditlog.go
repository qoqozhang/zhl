package auditlog

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"iam-go/middleware"
	jwtutil "iam-go/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phuslu/log"
	"gorm.io/gorm"
)

// OperationLog 操作审计日志表 model
// 表名: sys_operation_log
// 字段对齐 DDL（去掉 DB 层外键，应用层保证一致性；保留索引）
type OperationLog struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     *uint     `gorm:"index" json:"user_id"`
	Username   string    `gorm:"size:64" json:"username"`
	Action     string    `gorm:"size:128;not null" json:"action"`
	Method     string    `gorm:"size:16;not null" json:"method"`
	Path       string    `gorm:"size:255;not null" json:"path"`
	RequestID  string    `gorm:"size:64;index" json:"request_id"`
	ClientIP   string    `gorm:"size:64" json:"client_ip"`
	UserAgent  string    `gorm:"size:512" json:"user_agent"`
	StatusCode int       `gorm:"not null" json:"status_code"`
	Detail     JSONData  `gorm:"type:json" json:"detail"`
	CreatedAt  time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;index" json:"created_at"`
}

func (OperationLog) TableName() string {
	return "sys_operation_log"
}

// JSONDetail 操作详情结构
type JSONDetail struct {
	RequestBody  map[string]interface{} `json:"request_body,omitempty"`
	ResponseTime string                 `json:"response_time"`
	Err          string                 `json:"err,omitempty"`
}

// JSONData 自定义 JSON 类型（GORM 自动 marshal/unmarshal）
type JSONData map[string]interface{}

func (j JSONData) Value() ([]byte, error) { return json.Marshal(j) }
func (j *JSONData) Scan(src interface{}) error {
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	}
	return nil
}

// AuditMiddleware 操作审计日志中间件
// 在请求结束后，根据 JWT claims + 请求信息组装一条 OperationLog 写入 MySQL
func AuditMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// 生成 request_id，塞到 context 里方便后续 handler 取用
		requestID := uuid.NewString()
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// 读取请求体（handler 还要用，读完要再塞回去）
		var requestBodyBytes []byte
		if c.Request.Body != nil {
			requestBodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(strings.NewReader(string(requestBodyBytes)))
		}

		c.Next()

		// ---- 请求结束，组装审计日志 ----
		claims := middleware.GetCurrentUser(c)
		var userID *uint
		var username string
		if claims != nil {
			userID = &claims.UserID
			username = claims.Username
		} else if anonymousUser, ok := c.Get("anonymous_username"); ok {
			username, _ = anonymousUser.(string)
		}

		entry := &OperationLog{
			UserID:     userID,
			Username:   username,
			Action:     inferAction(c.Request.Method, c.Request.URL.Path),
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			RequestID:  requestID,
			ClientIP:   c.ClientIP(),
			UserAgent:  truncate(c.Request.UserAgent(), 512),
			StatusCode: c.Writer.Status(),
			CreatedAt:  start,
		}

		// Detail: 请求体脱敏 + 耗时 + 错误
		detail := JSONDetail{
			RequestBody:  desensitizeRequest(requestBodyBytes),
			ResponseTime: time.Since(start).String(),
		}
		if len(c.Errors) > 0 {
			detail.Err = c.Errors.String()
		}
		entry.Detail = JSONData{
			"request_body":  detail.RequestBody,
			"response_time": detail.ResponseTime,
			"err":           detail.Err,
		}
		// detail 为空值时精简
		if detail.Err == "" {
			delete(entry.Detail, "err")
		}

		// ---- 异步写入（不阻塞响应）----
		go func(e *OperationLog) {
			if err := db.WithContext(context.Background()).Create(e).Error; err != nil {
				log.Error().Err(err).Str("request_id", requestID).Msg("audit log insert failed")
			}
		}(entry)
	}
}

// inferAction 根据 method + path 推断操作类型
// 返回大写的 ACTION 常量字符串，如 LOGIN / CREATE_USER
func inferAction(method, path string) string {
	// 静态路由匹配（精确）
	switch path {
	case "/v1/users/login":
		return "LOGIN"
	case "/v1/users/logout":
		return "LOGOUT"
	case "/v1/users/refresh":
		return "REFRESH_TOKEN"
	case "/v1/users/me":
		if method == "GET" {
			return "GET_CURRENT_USER"
		}
	}

	// 动态路由匹配（含 :id）
	if strings.HasPrefix(path, "/v1/users") {
		parts := strings.Split(strings.TrimPrefix(path, "/v1/users"), "/")
		hasID := len(parts) >= 2 && parts[1] != "" && !strings.HasPrefix(parts[1], "me")

		switch method {
		case "POST":
			return "CREATE_USER"
		case "GET":
			if hasID {
				return "GET_USER"
			}
			return "LIST_USERS"
		case "PUT", "PATCH":
			return "UPDATE_USER"
		case "DELETE":
			return "DELETE_USER"
		}
	}

	// 其他路由：直接 METHOD_PATH
	return strings.ToUpper(method) + "_" + strings.ReplaceAll(strings.TrimPrefix(path, "/"), "/", "_")
}

// desensitizeRequest 解析请求体 JSON，脱敏敏感字段
// - password / password_confirm → "******"
// 非 JSON 请求体直接返回 nil
func desensitizeRequest(body []byte) map[string]interface{} {
	if len(body) == 0 {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	maskSensitive(raw)
	return raw
}

// 敏感字段列表
var sensitiveKeys = map[string]bool{
	"password":           true,
	"password_confirm":   true,
	"old_password":       true,
	"new_password":       true,
	"confirm_password":   true,
	"secret":             true,
	"token":              true,
	"refresh_token":      true,
	"access_token":       true,
	"api_key":            true,
	"api_secret":         true,
	"private_key":        true,
	"credit_card":        true,
	"card_number":        true,
	"cvv":                true,
}

func maskSensitive(data map[string]interface{}) {
	for k, v := range data {
		key := strings.ToLower(k)
		if sensitiveKeys[key] {
			data[k] = "******"
			continue
		}
		switch val := v.(type) {
		case map[string]interface{}:
			maskSensitive(val)
		case []interface{}:
			for _, item := range val {
				if m, ok := item.(map[string]interface{}); ok {
					maskSensitive(m)
				}
			}
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// 确保 jwtutil 引用（middleware 间接依赖 jwtutil.AccessClaims）
var _ = jwtutil.Manager{}