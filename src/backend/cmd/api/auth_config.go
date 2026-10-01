package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"hxy-blog/backend/internal/auth"
)

type authRuntimeConfig struct {
	currentKey  auth.SigningKey
	previousKey *auth.SigningKey
	handler     authHandlerConfig
}

func loadAuthConfig(getenv func(string) string) (authRuntimeConfig, error) {
	current := auth.SigningKey{
		ID:     strings.TrimSpace(getenv("AUTH_JWT_CURRENT_KEY_ID")),
		Secret: []byte(getenv("AUTH_JWT_CURRENT_KEY")),
	}
	if current.ID == "" || len(current.Secret) < 32 {
		return authRuntimeConfig{}, errors.New("AUTH_JWT_CURRENT_KEY_ID and a key of at least 32 bytes are required")
	}

	previousID := strings.TrimSpace(getenv("AUTH_JWT_PREVIOUS_KEY_ID"))
	previousSecret := getenv("AUTH_JWT_PREVIOUS_KEY")
	var previous *auth.SigningKey
	if previousID != "" || previousSecret != "" {
		if previousID == "" || len(previousSecret) < 32 {
			return authRuntimeConfig{}, errors.New("previous JWT key ID and key must be configured together")
		}
		value := auth.SigningKey{ID: previousID, Secret: []byte(previousSecret)}
		previous = &value
	}

	secureCookie, err := strconv.ParseBool(getenv("AUTH_SECURE_COOKIE"))
	if err != nil {
		return authRuntimeConfig{}, errors.New("AUTH_SECURE_COOKIE must be true or false")
	}
	origins, allLoopback, err := parseAllowedOrigins(getenv("AUTH_ALLOWED_ORIGINS"))
	if err != nil {
		return authRuntimeConfig{}, err
	}
	// 关闭 Secure 只允许本机开发来源，避免生产配置失误后通过明文 HTTP 传输长期会话。
	if !secureCookie && !allLoopback {
		return authRuntimeConfig{}, errors.New("insecure refresh cookies are only allowed for loopback development origins")
	}
	return authRuntimeConfig{
		currentKey:  current,
		previousKey: previous,
		handler: authHandlerConfig{
			secureCookie:   secureCookie,
			allowedOrigins: origins,
		},
	}, nil
}

func parseAllowedOrigins(raw string) (map[string]struct{}, bool, error) {
	origins := make(map[string]struct{})
	allLoopback := true
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parsed, err := url.Parse(item)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
			parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, false, fmt.Errorf("invalid allowed origin %q", item)
		}
		hostname := parsed.Hostname()
		loopback := hostname == "localhost" || net.ParseIP(hostname).IsLoopback()
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
			return nil, false, fmt.Errorf("allowed origin %q must use HTTPS unless it is loopback", item)
		}
		origins[item] = struct{}{}
		allLoopback = allLoopback && loopback
	}
	if len(origins) == 0 {
		return nil, false, errors.New("AUTH_ALLOWED_ORIGINS must contain at least one origin")
	}
	return origins, allLoopback, nil
}
