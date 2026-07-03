package middleware

import (
	"crynux_relay/api/tools"
	"crynux_relay/api/v2/response"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	errMissingAuthorizationHeader = errors.New("authorization header required")
	errInvalidAuthorizationHeader = errors.New("invalid authorization header format")
)

func getAuthorizedAddress(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", errMissingAuthorizationHeader
	}

	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		return "", errInvalidAuthorizationHeader
	}

	claims, err := tools.ValidateToken(tokenParts[1])
	if err != nil {
		return "", err
	}
	return claims.Address, nil
}

// JWTAuthMiddleware validates JWT token in request headers.
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		address, err := getAuthorizedAddress(c)
		if errors.Is(err, errMissingAuthorizationHeader) {
			c.JSON(http.StatusUnauthorized, response.Response{
				Message: "Authorization header required",
			})
			c.Abort()
			return
		}

		if errors.Is(err, errInvalidAuthorizationHeader) {
			c.JSON(http.StatusUnauthorized, response.Response{
				Message: "Invalid authorization header format. Use 'Bearer <token>'",
			})
			c.Abort()
			return
		}

		if err != nil {
			c.JSON(http.StatusUnauthorized, response.Response{
				Message: "Invalid or expired token",
			})
			c.Abort()
			return
		}

		c.Set("user_address", address)
		c.Next()
	}
}

// GetAuthorizedAddress returns the JWT address without aborting the request.
func GetAuthorizedAddress(c *gin.Context) (string, bool) {
	address, err := getAuthorizedAddress(c)
	if err != nil {
		return "", false
	}
	return address, true
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
