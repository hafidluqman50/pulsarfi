package custodian

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

		if claims.Role != "custodian" {
			response.Forbidden(c, "custodian access required")
			return
		}

		c.Set("custodian", Claims{WalletAddress: claims.WalletAddress, Role: claims.Role})
		c.Next()
	}
}

func Get(c *gin.Context) (Claims, bool) {
	value, ok := c.Get("custodian")
	if !ok {
		return Claims{}, false
	}
	claims, ok := value.(Claims)
	return claims, ok
}
