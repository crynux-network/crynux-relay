package middleware

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/api/v2/validate"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type SignatureInputBuilder func(c *gin.Context) interface{}

// JWTOrSignatureAuthMiddleware authorizes a request with either a JWT or a query signature.
func JWTOrSignatureAuthMiddleware(buildSignatureInput SignatureInputBuilder) gin.HandlerFunc {
	return func(c *gin.Context) {
		address, jwtErr := getAuthorizedAddress(c)
		if jwtErr != nil {
			timestamp, err := strconv.ParseInt(c.Query("timestamp"), 10, 64)
			if err != nil {
				rejectJWTOrSignature(c)
				return
			}

			signature := c.Query("signature")
			match, signerAddress, err := validate.ValidateSignature(
				buildSignatureInput(c),
				timestamp,
				signature,
			)
			if err != nil || !match {
				if err != nil {
					log.WithError(err).Debug("request signature validation failed")
				}
				rejectJWTOrSignature(c)
				return
			}
			address = signerAddress
		}

		c.Set("user_address", address)
		c.Next()
	}
}

func rejectJWTOrSignature(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, response.Response{
		Message: "Valid JWT or signature required",
	})
	c.Abort()
}
