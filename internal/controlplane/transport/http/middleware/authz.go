package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/internal/platform/authz"
)

func Require(authorizer *authz.Authorizer, action authz.Action) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authorizer == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authorizer is not configured"})
			return
		}

		if err := authorizer.Assert(c.Request.Context(), action); err != nil {
			status := http.StatusForbidden
			if errors.Is(err, authz.ErrMissingPrincipal) {
				status = http.StatusUnauthorized
			}
			c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
			return
		}

		c.Next()
	}
}
