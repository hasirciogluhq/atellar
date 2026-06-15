package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/internal/platform/authn"
)

func NodeAuth(authenticator authn.Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		credential, err := authn.ParseAuthorizationHeader(c.GetHeader(authn.HeaderAuthorization))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		principal, err := authenticator.Authenticate(c.Request.Context(), credential)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		ctx := authn.WithPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
