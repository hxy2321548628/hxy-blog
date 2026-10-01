package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"hxy-blog/backend/internal/auth"
)

type authServiceStub struct {
	loginSession   auth.Session
	loginErr       error
	refreshSession auth.Session
	refreshErr     error
	logoutErr      error
	logoutToken    string
}

func (stub *authServiceStub) Login(_ context.Context, _, _ string) (auth.Session, error) {
	return stub.loginSession, stub.loginErr
}

func (stub *authServiceStub) Refresh(_ context.Context, _ string) (auth.Session, error) {
	return stub.refreshSession, stub.refreshErr
}

func (stub *authServiceStub) Logout(_ context.Context, rawRefresh string) error {
	stub.logoutToken = rawRefresh
	return stub.logoutErr
}

type accessTokenVerifierStub struct {
	claims auth.AccessClaims
	err    error
}

func (stub accessTokenVerifierStub) Parse(_ string) (auth.AccessClaims, error) {
	return stub.claims, stub.err
}

func TestLoginSetsProtectedRefreshCookie(t *testing.T) {
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	service := &authServiceStub{loginSession: auth.Session{
		Admin:            auth.Admin{ID: 42, Username: "admin"},
		AccessToken:      "access-token",
		AccessExpiresAt:  now.Add(auth.AccessTokenLifetime),
		RefreshToken:     "refresh-token",
		RefreshExpiresAt: now.Add(auth.RefreshTokenLifetime),
	}}
	router := newAuthTestRouter(service, accessTokenVerifierStub{}, secureAuthHandlerConfig())
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{\"username\":\"admin\",\"password\":\"secret\"}"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	cookie := response.Header().Get("Set-Cookie")
	for _, part := range []string{"__Secure-hxy_refresh=refresh-token", "Path=/api/auth", "HttpOnly", "Secure", "SameSite=Strict"} {
		if !strings.Contains(cookie, part) {
			t.Fatalf("Set-Cookie = %q, missing %q", cookie, part)
		}
	}
	if strings.Contains(response.Body.String(), "refresh-token") {
		t.Fatal("response body leaked refresh token")
	}
	var body struct {
		AccessToken string `json:"accessToken"`
		Admin       struct {
			ID       uint64 `json:"id"`
			Username string `json:"username"`
		} `json:"admin"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AccessToken != "access-token" || body.Admin.ID != 42 || body.Admin.Username != "admin" {
		t.Fatalf("body = %#v", body)
	}
}

func TestLoginReturnsSameErrorForInvalidCredentials(t *testing.T) {
	service := &authServiceStub{loginErr: auth.ErrInvalidCredentials}
	router := newAuthTestRouter(service, accessTokenVerifierStub{}, secureAuthHandlerConfig())
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{\"username\":\"unknown\",\"password\":\"wrong\"}"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if body := strings.TrimSpace(response.Body.String()); body != "{\"code\":\"INVALID_CREDENTIALS\",\"message\":\"用户名或密码错误\"}" {
		t.Fatalf("body = %q", body)
	}
}

func TestLoginRateLimitStopsRepeatedFailures(t *testing.T) {
	service := &authServiceStub{loginErr: auth.ErrInvalidCredentials}
	router := newAuthTestRouter(service, accessTokenVerifierStub{}, secureAuthHandlerConfig())

	for attempt := 1; attempt <= loginFailureLimit+1; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{\"username\":\"admin\",\"password\":\"wrong\"}"))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		wantStatus := http.StatusUnauthorized
		if attempt > loginFailureLimit {
			wantStatus = http.StatusTooManyRequests
		}
		if response.Code != wantStatus {
			t.Fatalf("attempt %d status = %d, want %d", attempt, response.Code, wantStatus)
		}
	}
}

func TestRefreshRequiresAllowedOriginAndRotatesCookie(t *testing.T) {
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	service := &authServiceStub{refreshSession: auth.Session{
		Admin:            auth.Admin{ID: 42, Username: "admin"},
		AccessToken:      "next-access",
		AccessExpiresAt:  now.Add(auth.AccessTokenLifetime),
		RefreshToken:     "next-refresh",
		RefreshExpiresAt: now.Add(auth.RefreshTokenLifetime),
	}}
	router := newAuthTestRouter(service, accessTokenVerifierStub{}, secureAuthHandlerConfig())

	for _, test := range []struct {
		name       string
		origin     string
		wantStatus int
	}{
		{name: "missing origin", wantStatus: http.StatusForbidden},
		{name: "wrong origin", origin: "https://evil.example", wantStatus: http.StatusForbidden},
		{name: "allowed origin", origin: "https://hxy2333.site", wantStatus: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
			request.Header.Set("Origin", test.origin)
			request.AddCookie(&http.Cookie{Name: secureRefreshCookieName, Value: "current-refresh"})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status code = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantStatus == http.StatusOK && !strings.Contains(response.Header().Get("Set-Cookie"), "next-refresh") {
				t.Fatalf("Set-Cookie = %q", response.Header().Get("Set-Cookie"))
			}
		})
	}
}

func TestRefreshReplayClearsCookie(t *testing.T) {
	service := &authServiceStub{refreshErr: auth.ErrRefreshReplay}
	router := newAuthTestRouter(service, accessTokenVerifierStub{}, secureAuthHandlerConfig())
	request := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	request.Header.Set("Origin", "https://hxy2333.site")
	request.AddCookie(&http.Cookie{Name: secureRefreshCookieName, Value: "replayed-refresh"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if cookie := response.Header().Get("Set-Cookie"); !strings.Contains(cookie, "Max-Age=0") {
		t.Fatalf("Set-Cookie = %q, want cleared cookie", cookie)
	}
}

func TestLogoutRevokesSessionAndClearsCookie(t *testing.T) {
	service := &authServiceStub{}
	router := newAuthTestRouter(service, accessTokenVerifierStub{}, secureAuthHandlerConfig())
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.Header.Set("Origin", "https://hxy2333.site")
	request.AddCookie(&http.Cookie{Name: secureRefreshCookieName, Value: "refresh-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || service.logoutToken != "refresh-token" {
		t.Fatalf("status=%d logoutToken=%q", response.Code, service.logoutToken)
	}
	if cookie := response.Header().Get("Set-Cookie"); !strings.Contains(cookie, "Max-Age=0") {
		t.Fatalf("Set-Cookie = %q, want cleared cookie", cookie)
	}
}

func TestMeRequiresValidBearerToken(t *testing.T) {
	service := &authServiceStub{}
	verifier := accessTokenVerifierStub{claims: auth.AccessClaims{AdminID: 42, Username: "admin"}}
	router := newAuthTestRouter(service, verifier, secureAuthHandlerConfig())

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != "{\"id\":42,\"username\":\"admin\"}" {
		t.Fatalf("status=%d body=%q", response.Code, strings.TrimSpace(response.Body.String()))
	}

	invalidRouter := newAuthTestRouter(service, accessTokenVerifierStub{err: errors.New("invalid token")}, secureAuthHandlerConfig())
	request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")
	response = httptest.NewRecorder()
	invalidRouter.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token status = %d", response.Code)
	}
}

func newAuthTestRouter(
	service authSessionService,
	verifier accessTokenVerifier,
	config authHandlerConfig,
) http.Handler {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerAuthRoutes(router, service, verifier, config, newLoginLimiter())
	return router
}

func secureAuthHandlerConfig() authHandlerConfig {
	return authHandlerConfig{
		secureCookie: true,
		allowedOrigins: map[string]struct{}{
			"https://hxy2333.site": {},
		},
	}
}
