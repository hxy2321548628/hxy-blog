package main

import (
	"strings"
	"testing"
)

func TestLoadAuthConfig(t *testing.T) {
	values := map[string]string{
		"AUTH_JWT_CURRENT_KEY_ID":  "current",
		"AUTH_JWT_CURRENT_KEY":     strings.Repeat("a", 32),
		"AUTH_JWT_PREVIOUS_KEY_ID": "previous",
		"AUTH_JWT_PREVIOUS_KEY":    strings.Repeat("b", 32),
		"AUTH_ALLOWED_ORIGINS":     "https://hxy2333.site,http://localhost:5173",
		"AUTH_SECURE_COOKIE":       "true",
	}
	config, err := loadAuthConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("loadAuthConfig() error = %v", err)
	}
	if config.currentKey.ID != "current" || config.previousKey == nil || config.previousKey.ID != "previous" {
		t.Fatalf("signing keys = %#v / %#v", config.currentKey, config.previousKey)
	}
	if !config.handler.secureCookie || len(config.handler.allowedOrigins) != 2 {
		t.Fatalf("handler config = %#v", config.handler)
	}
}

func TestLoadAuthConfigRejectsMissingOrPartialSecrets(t *testing.T) {
	tests := []map[string]string{
		{},
		{
			"AUTH_JWT_CURRENT_KEY_ID": "current",
			"AUTH_JWT_CURRENT_KEY":    "short",
			"AUTH_ALLOWED_ORIGINS":    "https://hxy2333.site",
			"AUTH_SECURE_COOKIE":      "true",
		},
		{
			"AUTH_JWT_CURRENT_KEY_ID":  "current",
			"AUTH_JWT_CURRENT_KEY":     strings.Repeat("a", 32),
			"AUTH_JWT_PREVIOUS_KEY_ID": "previous",
			"AUTH_ALLOWED_ORIGINS":     "https://hxy2333.site",
			"AUTH_SECURE_COOKIE":       "true",
		},
	}
	for index, values := range tests {
		if _, err := loadAuthConfig(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("case %d loadAuthConfig() error = nil", index)
		}
	}
}
