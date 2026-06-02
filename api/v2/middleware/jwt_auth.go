package middleware

import (
	"crynux_relay/api/tools"
	"crynux_relay/api/v2/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTAuthMiddleware validates JWT token in request headers.
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, response.Response{
				Message: "Authorization header required",
			})
			c.Abort()
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, response.Response{
				Message: "Invalid authorization header format. Use 'Bearer <token>'",
			})
			c.Abort()
			return
		}

		claims, err := tools.ValidateToken(tokenParts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.Response{
				Message: "Invalid or expired token",
			})
			c.Abort()
			return
		}

		c.Set("user_address", claims.Address)
		c.Next()
	}
}

// GetUserAddress extracts user address from gin context.
func GetUserAddress(c *gin.Context) string {
	if address, exists := c.Get("user_address"); exists {
		if addr, ok := address.(string); ok {
			return addr
		}
	}
	return ""
}
