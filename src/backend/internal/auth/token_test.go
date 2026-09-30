package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	manager := newTestTokenManager(t, SigningKey{ID: "current", Secret: testSecret("current")}, nil, now)

	raw, expiresAt, err := manager.Issue(42, "admin")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if !expiresAt.Equal(now.Add(AccessTokenLifetime)) {
		t.Fatalf("expiresAt = %v", expiresAt)
	}

	claims, err := manager.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if claims.AdminID != 42 || claims.Username != "admin" {
		t.Fatalf("claims = %#v", claims)
	}
	if claims.Issuer != accessTokenIssuer || len(claims.Audience) != 1 || claims.Audience[0] != accessTokenAudience {
		t.Fatalf("registered claims = %#v", claims.RegisteredClaims)
	}
	if claims.ID == "" || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		t.Fatalf("required claims missing: %#v", claims.RegisteredClaims)
	}
}

func TestAccessTokenAcceptsPreviousKeyDuringRotation(t *testing.T) {
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	previous := SigningKey{ID: "previous", Secret: testSecret("previous")}
	oldManager := newTestTokenManager(t, previous, nil, now)
	raw, _, err := oldManager.Issue(42, "admin")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	manager := newTestTokenManager(
		t,
		SigningKey{ID: "current", Secret: testSecret("current")},
		&previous,
		now,
	)
	if _, err := manager.Parse(raw); err != nil {
		t.Fatalf("Parse() previous key error = %v", err)
	}
}

func TestAccessTokenRejectsExpiredAndWrongAlgorithmTokens(t *testing.T) {
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	key := SigningKey{ID: "current", Secret: testSecret("current")}
	manager := newTestTokenManager(t, key, nil, now)
	raw, _, err := manager.Issue(42, "admin")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	manager.now = func() time.Time { return now.Add(AccessTokenLifetime + tokenClockLeeway + time.Second) }
	if _, err := manager.Parse(raw); err == nil {
		t.Fatal("Parse() expired token error = nil")
	}

	claims := AccessClaims{
		AdminID:  42,
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    accessTokenIssuer,
			Audience:  jwt.ClaimStrings{accessTokenAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenLifetime)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        "token-id",
		},
	}
	wrongAlgorithm := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	wrongAlgorithm.Header["kid"] = key.ID
	wrongRaw, err := wrongAlgorithm.SignedString(key.Secret)
	if err != nil {
		t.Fatalf("sign wrong algorithm token: %v", err)
	}
	manager.now = func() time.Time { return now }
	if _, err := manager.Parse(wrongRaw); err == nil {
		t.Fatal("Parse() wrong algorithm error = nil")
	}
}

func TestAccessTokenRejectsUnknownKeyAndMissingClaims(t *testing.T) {
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	key := SigningKey{ID: "current", Secret: testSecret("current")}
	manager := newTestTokenManager(t, key, nil, now)

	claims := AccessClaims{AdminID: 42, Username: "admin"}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "unknown"
	raw, err := token.SignedString(key.Secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := manager.Parse(raw); err == nil {
		t.Fatal("Parse() unknown key error = nil")
	}

	token.Header["kid"] = key.ID
	raw, err = token.SignedString(key.Secret)
	if err != nil {
		t.Fatalf("sign token without claims: %v", err)
	}
	if _, err := manager.Parse(raw); err == nil {
		t.Fatal("Parse() missing claims error = nil")
	}
}

func newTestTokenManager(t *testing.T, current SigningKey, previous *SigningKey, now time.Time) *AccessTokenManager {
	t.Helper()
	manager, err := NewAccessTokenManager(current, previous)
	if err != nil {
		t.Fatalf("NewAccessTokenManager() error = %v", err)
	}
	manager.now = func() time.Time { return now }
	return manager
}

func testSecret(label string) []byte {
	return []byte(strings.Repeat(label+"-", 8))
}
