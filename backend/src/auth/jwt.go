package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	Issuer       string
	AccessSecret []byte
	AccessTTL    time.Duration
}

type Claims struct {
	WalletAddress string `json:"wallet_address"`
	Role          string `json:"role"` // "custodian" | "user"
	jwt.RegisteredClaims
}

func NewAccessToken(cfg Config, walletAddress, role string) (string, error) {
	claims := Claims{
		WalletAddress: walletAddress,
		Role:          role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			Subject:   walletAddress,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(cfg.AccessSecret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func ParseAccessToken(cfg Config, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return cfg.AccessSecret, nil
	}, jwt.WithIssuer(cfg.Issuer))
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
