// Package user gates routes that need "this is a real, identified wallet"
// without requiring custodian privilege — every SIWE login already gets a
// role: "user" (or "custodian") JWT (see src/service/auth), this just wires
// it into routing the way custodianMiddleware.Auth already does for
// custodian-only routes.
package user

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

type Claims struct {
	WalletAddress string
	Role          string
}

// Auth accepts any valid token — role "user" or "custodian" — since both are
// an authenticated wallet. It does not gate on privilege, only identity.
func Auth(cfg auth.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Unauthorized(c, "missing or invalid authorization header")
			return
		}

		claims, err := auth.ParseAccessToken(cfg, parts[1])
		if err != nil {
			response.Unauthorized(c, "invalid or expired token")
			return
		}

		c.Set("user", Claims{WalletAddress: claims.WalletAddress, Role: claims.Role})
		c.Next()
	}
}

func Get(c *gin.Context) (Claims, bool) {
	value, ok := c.Get("user")
	if !ok {
		return Claims{}, false
	}
	claims, ok := value.(Claims)
	return claims, ok
}
