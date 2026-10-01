package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

type authRepositoryStub struct {
	admin           Admin
	findErr         error
	created         *RefreshToken
	rotation        RotationResult
	rotationErr     error
	rotatedHash     [sha256.Size]byte
	rotatedNext     [sha256.Size]byte
	revokedHash     [sha256.Size]byte
	revokeFamilyErr error
}

func (stub *authRepositoryStub) FindAdminByUsername(_ context.Context, _ string) (Admin, error) {
	return stub.admin, stub.findErr
}

func (stub *authRepositoryStub) CreateRefreshToken(_ context.Context, token RefreshToken) error {
	stub.created = &token
	return nil
}

func (stub *authRepositoryStub) RotateRefreshToken(
	_ context.Context,
	currentHash [sha256.Size]byte,
	nextHash [sha256.Size]byte,
	_ time.Time,
) (RotationResult, error) {
	stub.rotatedHash = currentHash
	stub.rotatedNext = nextHash
	return stub.rotation, stub.rotationErr
}

func (stub *authRepositoryStub) RevokeRefreshFamily(
	_ context.Context,
	tokenHash [sha256.Size]byte,
	_ time.Time,
) error {
	stub.revokedHash = tokenHash
	return stub.revokeFamilyErr
}

func TestServiceLoginCreatesHashedRefreshSession(t *testing.T) {
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	hasher := NewPasswordHasher(rand.Reader)
	passwordHash, err := hasher.Hash(context.Background(), "correct password")
	if err != nil {
		t.Fatalf("hash fixture password: %v", err)
	}
	repository := &authRepositoryStub{admin: Admin{ID: 42, Username: "admin", PasswordHash: passwordHash}}
	manager := newTestTokenManager(t, SigningKey{ID: "current", Secret: testSecret("current")}, nil, now)
	service := newTestService(t, repository, hasher, manager, now)

	session, err := service.Login(context.Background(), "admin", "correct password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if session.RefreshToken == "" || repository.created == nil {
		t.Fatal("Login() did not create refresh session")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(session.RefreshToken)
	if err != nil || len(raw) != refreshTokenBytes {
		t.Fatalf("refresh token = %q, decode error = %v", session.RefreshToken, err)
	}
	expectedHash := sha256.Sum256([]byte(session.RefreshToken))
	if !bytes.Equal(repository.created.TokenHash[:], expectedHash[:]) {
		t.Fatal("stored refresh token hash does not match raw token")
	}
	if len(repository.created.FamilyID) != refreshFamilyIDBytes {
		t.Fatalf("family ID length = %d", len(repository.created.FamilyID))
	}
	if !session.RefreshExpiresAt.Equal(now.Add(RefreshTokenLifetime)) ||
		!repository.created.SessionExpiresAt.Equal(now.Add(SessionAbsoluteLifetime)) {
		t.Fatalf("refresh expiry = %v, stored = %#v", session.RefreshExpiresAt, repository.created)
	}
	claims, err := manager.Parse(session.AccessToken)
	if err != nil || claims.AdminID != 42 {
		t.Fatalf("access token claims = %#v, error = %v", claims, err)
	}
}

func TestServiceLoginUsesSameErrorForUnknownUserAndWrongPassword(t *testing.T) {
	hasher := NewPasswordHasher(rand.Reader)
	passwordHash, err := hasher.Hash(context.Background(), "correct password")
	if err != nil {
		t.Fatalf("hash fixture password: %v", err)
	}
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	manager := newTestTokenManager(t, SigningKey{ID: "current", Secret: testSecret("current")}, nil, now)

	tests := []struct {
		name       string
		repository *authRepositoryStub
		password   string
	}{
		{name: "unknown user", repository: &authRepositoryStub{findErr: ErrAdminNotFound}, password: "password"},
		{name: "wrong password", repository: &authRepositoryStub{admin: Admin{ID: 42, Username: "admin", PasswordHash: passwordHash}}, password: "wrong"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newTestService(t, test.repository, hasher, manager, now)
			if _, err := service.Login(context.Background(), "admin", test.password); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestServiceRefreshRotatesTokenAndIssuesAccessToken(t *testing.T) {
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	repository := &authRepositoryStub{rotation: RotationResult{
		Admin:     Admin{ID: 42, Username: "admin"},
		ExpiresAt: now.Add(RefreshTokenLifetime),
	}}
	manager := newTestTokenManager(t, SigningKey{ID: "current", Secret: testSecret("current")}, nil, now)
	service := newTestService(t, repository, NewPasswordHasher(rand.Reader), manager, now)
	currentRaw := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x24}, refreshTokenBytes))

	session, err := service.Refresh(context.Background(), currentRaw)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if repository.rotatedHash != sha256.Sum256([]byte(currentRaw)) {
		t.Fatal("RotateRefreshToken() received wrong current token hash")
	}
	if repository.rotatedNext != sha256.Sum256([]byte(session.RefreshToken)) {
		t.Fatal("RotateRefreshToken() received wrong next token hash")
	}
	claims, err := manager.Parse(session.AccessToken)
	if err != nil || claims.AdminID != 42 {
		t.Fatalf("access token claims = %#v, error = %v", claims, err)
	}
}

func TestServiceLogoutHashesRawToken(t *testing.T) {
	now := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	repository := &authRepositoryStub{}
	manager := newTestTokenManager(t, SigningKey{ID: "current", Secret: testSecret("current")}, nil, now)
	service := newTestService(t, repository, NewPasswordHasher(rand.Reader), manager, now)
	raw := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x24}, refreshTokenBytes))

	if err := service.Logout(context.Background(), raw); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if repository.revokedHash != sha256.Sum256([]byte(raw)) {
		t.Fatal("RevokeRefreshFamily() received wrong token hash")
	}
}

func newTestService(
	t *testing.T,
	repository sessionRepository,
	hasher *PasswordHasher,
	manager *AccessTokenManager,
	now time.Time,
) *Service {
	t.Helper()
	// 构造器会生成一次虚假账户哈希；额外随机字节供 Refresh Token 和 family ID 使用。
	randomBytes := bytes.NewReader(bytes.Repeat([]byte{0x42}, 256))
	service, err := NewService(context.Background(), repository, hasher, manager, randomBytes)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return now }
	return service
}
