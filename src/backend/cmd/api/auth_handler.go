package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"hxy-blog/backend/internal/auth"
)

const (
	secureRefreshCookieName   = "__Secure-hxy_refresh"
	insecureRefreshCookieName = "hxy_refresh_dev"
	authRequestBodyLimit      = 4 << 10
	loginFailureLimit         = 5
	loginFailureWindow        = 15 * time.Minute
)

type authSessionService interface {
	Login(ctx context.Context, username, password string) (auth.Session, error)
	Refresh(ctx context.Context, rawRefresh string) (auth.Session, error)
	Logout(ctx context.Context, rawRefresh string) error
}

type accessTokenVerifier interface {
	Parse(raw string) (auth.AccessClaims, error)
}

type authHandlerConfig struct {
	secureCookie   bool
	allowedOrigins map[string]struct{}
}

type authHTTPDependencies struct {
	sessions authSessionService
	tokens   accessTokenVerifier
	config   authHandlerConfig
}

func registerAuthRoutes(
	router *gin.Engine,
	sessions authSessionService,
	tokens accessTokenVerifier,
	config authHandlerConfig,
	limiter *loginLimiter,
) {
	router.POST("/api/auth/login", loginHandler(sessions, config, limiter))
	router.POST("/api/auth/refresh", requireTrustedOrigin(config, refreshHandler(sessions, config)))
	router.POST("/api/auth/logout", requireTrustedOrigin(config, logoutHandler(sessions, config)))
	router.GET("/api/auth/me", meHandler(tokens))
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	AccessToken     string        `json:"accessToken"`
	AccessExpiresAt time.Time     `json:"accessExpiresAt"`
	Admin           adminResponse `json:"admin"`
}

type adminResponse struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

func loginHandler(sessions authSessionService, config authHandlerConfig, limiter *loginLimiter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var request loginRequest
		if err := decodeJSONBody(ctx, &request); err != nil {
			respondError(ctx, http.StatusBadRequest, "INVALID_REQUEST", "请求格式无效")
			return
		}
		source := ctx.ClientIP()
		if !limiter.Allow(source, request.Username) {
			respondError(ctx, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "登录尝试过多，请稍后再试")
			return
		}
		session, err := sessions.Login(ctx.Request.Context(), request.Username, request.Password)
		if errors.Is(err, auth.ErrInvalidCredentials) {
			limiter.RecordFailure(source, request.Username)
			respondError(ctx, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
			return
		}
		if err != nil {
			slog.Error("administrator login failed", "error", err)
			respondError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用")
			return
		}
		limiter.Reset(source, request.Username)
		setRefreshCookie(ctx, config, session.RefreshToken, session.RefreshExpiresAt)
		respondSession(ctx, session)
	}
}

func refreshHandler(sessions authSessionService, config authHandlerConfig) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		cookie, err := ctx.Request.Cookie(refreshCookieName(config))
		if err != nil {
			clearRefreshCookie(ctx, config)
			respondError(ctx, http.StatusUnauthorized, "SESSION_EXPIRED", "登录状态已失效")
			return
		}
		session, err := sessions.Refresh(ctx.Request.Context(), cookie.Value)
		if errors.Is(err, auth.ErrInvalidSession) || errors.Is(err, auth.ErrRefreshReplay) {
			clearRefreshCookie(ctx, config)
			respondError(ctx, http.StatusUnauthorized, "SESSION_EXPIRED", "登录状态已失效")
			return
		}
		if err != nil {
			slog.Error("refresh administrator session failed", "error", err)
			respondError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用")
			return
		}
		setRefreshCookie(ctx, config, session.RefreshToken, session.RefreshExpiresAt)
		respondSession(ctx, session)
	}
}

func logoutHandler(sessions authSessionService, config authHandlerConfig) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if cookie, err := ctx.Request.Cookie(refreshCookieName(config)); err == nil {
			if err := sessions.Logout(ctx.Request.Context(), cookie.Value); err != nil {
				slog.Error("logout administrator session failed", "error", err)
				respondError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用")
				return
			}
		}
		clearRefreshCookie(ctx, config)
		ctx.Status(http.StatusNoContent)
	}
}

func meHandler(tokens accessTokenVerifier) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		raw, ok := bearerToken(ctx.GetHeader("Authorization"))
		if !ok {
			respondError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "需要登录")
			return
		}
		claims, err := tokens.Parse(raw)
		if err != nil {
			respondError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "需要登录")
			return
		}
		ctx.JSON(http.StatusOK, adminResponse{ID: claims.AdminID, Username: claims.Username})
	}
}

func requireAccessToken(tokens accessTokenVerifier) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		raw, ok := bearerToken(ctx.GetHeader("Authorization"))
		if !ok {
			respondError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "需要登录")
			ctx.Abort()
			return
		}
		if _, err := tokens.Parse(raw); err != nil {
			respondError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "需要登录")
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

func decodeJSONBody(ctx *gin.Context, target any) error {
	return decodeJSONBodyWithLimit(ctx, target, authRequestBodyLimit)
}

func decodeJSONBodyWithLimit(ctx *gin.Context, target any, limit int64) error {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, limit)
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	// 第二次解码必须到 EOF，拒绝在合法 JSON 后拼接额外对象。
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}

func requireTrustedOrigin(config authHandlerConfig, next gin.HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !trustedRequestOrigin(ctx.Request, config.allowedOrigins) {
			respondError(ctx, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "请求来源无效")
			return
		}
		next(ctx)
	}
}

func trustedRequestOrigin(request *http.Request, allowed map[string]struct{}) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		referer, err := url.Parse(request.Referer())
		if err != nil || referer.Scheme == "" || referer.Host == "" {
			return false
		}
		origin = referer.Scheme + "://" + referer.Host
	}
	_, ok := allowed[origin]
	return ok
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	raw := strings.TrimPrefix(header, prefix)
	return raw, raw != "" && !strings.ContainsAny(raw, " \t\r\n")
}

func respondSession(ctx *gin.Context, session auth.Session) {
	ctx.JSON(http.StatusOK, authResponse{
		AccessToken:     session.AccessToken,
		AccessExpiresAt: session.AccessExpiresAt,
		Admin: adminResponse{
			ID:       session.Admin.ID,
			Username: session.Admin.Username,
		},
	})
}

func respondError(ctx *gin.Context, status int, code, message string) {
	ctx.JSON(status, gin.H{"code": code, "message": message})
}

func refreshCookieName(config authHandlerConfig) string {
	if config.secureCookie {
		return secureRefreshCookieName
	}
	return insecureRefreshCookieName
}

func setRefreshCookie(ctx *gin.Context, config authHandlerConfig, value string, expiresAt time.Time) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     refreshCookieName(config),
		Value:    value,
		Path:     "/api/auth",
		Expires:  expiresAt,
		Secure:   config.secureCookie,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearRefreshCookie(ctx *gin.Context, config authHandlerConfig) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     refreshCookieName(config),
		Path:     "/api/auth",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		Secure:   config.secureCookie,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func corsMiddleware(allowed map[string]struct{}) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		_, ok := allowed[origin]
		if origin != "" && ok {
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Access-Control-Allow-Credentials", "true")
			ctx.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			ctx.Header("Vary", "Origin")
		}
		if ctx.Request.Method == http.MethodOptions {
			if !ok {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next()
	}
}

type loginAttempt struct {
	failures    int
	windowStart time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
	now      func() time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: make(map[string]loginAttempt), now: time.Now}
}

func (limiter *loginLimiter) Allow(source, username string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	return limiter.allowedLocked("source:"+normalizedSource(source), now) &&
		limiter.allowedLocked("account:"+strings.ToLower(strings.TrimSpace(username)), now)
}

func (limiter *loginLimiter) RecordFailure(source, username string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	limiter.recordLocked("source:"+normalizedSource(source), now)
	limiter.recordLocked("account:"+strings.ToLower(strings.TrimSpace(username)), now)
}

func (limiter *loginLimiter) Reset(source, username string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	delete(limiter.attempts, "source:"+normalizedSource(source))
	delete(limiter.attempts, "account:"+strings.ToLower(strings.TrimSpace(username)))
}

func (limiter *loginLimiter) allowedLocked(key string, now time.Time) bool {
	attempt, ok := limiter.attempts[key]
	if !ok || now.Sub(attempt.windowStart) >= loginFailureWindow {
		delete(limiter.attempts, key)
		return true
	}
	return attempt.failures < loginFailureLimit
}

func (limiter *loginLimiter) recordLocked(key string, now time.Time) {
	attempt, ok := limiter.attempts[key]
	if !ok || now.Sub(attempt.windowStart) >= loginFailureWindow {
		limiter.attempts[key] = loginAttempt{failures: 1, windowStart: now}
		return
	}
	attempt.failures++
	limiter.attempts[key] = attempt
}

func normalizedSource(source string) string {
	if host, _, err := net.SplitHostPort(source); err == nil {
		return host
	}
	return source
}
