package config

import (
	"fmt"
	"os"
	"strings"
)

func GetEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func RequireEnv(key string) (string, error) {
	v := GetEnv(key)
	if v == "" {
		return "", fmt.Errorf("required env var %s is not set", key)
	}
	return v, nil
}
