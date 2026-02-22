package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"gopherai-resume/internal/pkg/jwtutil"
	"gopherai-resume/internal/transport/http/response"
)

const (
	ContextUserIDKey   = "user_id"
	ContextUsernameKey = "username"
)

func AuthJWT(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			response.Error(c, 401, response.CodeUnauthorized, "missing authorization header")
			c.Abort()
			return
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			response.Error(c, 401, response.CodeUnauthorized, "invalid authorization scheme")
			c.Abort()
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
		claims, err := jwtutil.ParseToken(secret, token)
		if err != nil {
			response.Error(c, 401, response.CodeUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextUsernameKey, claims.Username)
		c.Next()
	}
}
