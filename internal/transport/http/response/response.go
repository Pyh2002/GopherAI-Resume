package response

import "github.com/gin-gonic/gin"

const (
	CodeOK                 = 0
	CodeBadRequest         = 40000
	CodeUnauthorized       = 40100
	CodeInternalServer     = 50000
	CodeUsernameExists     = 40001
	CodeEmailExists        = 40002
	CodeInvalidCredentials = 40101
	CodeSessionNotFound    = 40401
)

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(200, APIResponse{
		Code:    CodeOK,
		Message: "ok",
		Data:    data,
	})
}

func Error(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, APIResponse{
		Code:    code,
		Message: message,
	})
}
