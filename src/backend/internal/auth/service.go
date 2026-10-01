package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	RefreshTokenLifetime    = 7 * 24 * time.Hour
	SessionAbsoluteLifetime = 30 * 24 * time.Hour
	refreshTokenBytes       = 32
	refreshFamilyIDBytes    = 16
	maxUsernameBytes        = 64
)

var (
	ErrAdminNotFound      = errors.New("administrator not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid refresh session")
	ErrRefreshReplay      = errors.New("refresh token replay detected")
	ErrAdminAlreadyExists = errors.New("administrator already exists")
)

type Admin struct {
	ID           uint64
	Username     string
	PasswordHash string
}

type RefreshToken struct {
	AdminID          uint64
	TokenHash        [sha256.Size]byte
	FamilyID         []byte
	ExpiresAt        time.Time
	SessionExpiresAt time.Time
	CreatedAt        time.Time
}

type RotationResult struct {
	Admin     Admin
	ExpiresAt time.Time
}

type Session struct {
	Admin            Admin
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type sessionRepository interface {
	FindAdminByUsername(ctx context.Context, username string) (Admin, error)
	CreateRefreshToken(ctx context.Context, token RefreshToken) error
	RotateRefreshToken(
		ctx context.Context,
		currentHash [sha256.Size]byte,
		nextHash [sha256.Size]byte,
		now time.Time,
	) (RotationResult, error)
	RevokeRefreshFamily(ctx context.Context, tokenHash [sha256.Size]byte, now time.Time) error
}

type Service struct {
	repository sessionRepository
	hasher     *PasswordHasher
	tokens     *AccessTokenManager
	random     io.Reader
	dummyHash  string
	now        func() time.Time
}

func NewService(
	ctx context.Context,
	repository sessionRepository,
	hasher *PasswordHasher,
	tokens *AccessTokenManager,
	random io.Reader,
) (*Service, error) {
	// 未知用户名也验证一次真实 Argon2id 哈希，减少账户枚举的时间差。
	dummyHash, err := hasher.Hash(ctx, "hxy-blog-dummy-password")
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}
	return &Service{
		repository: repository,
		hasher:     hasher,
		tokens:     tokens,
		random:     random,
		dummyHash:  dummyHash,
		now:        time.Now,
	}, nil
}

func (service *Service) Login(ctx context.Context, username, password string) (Session, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > maxUsernameBytes || validatePasswordLength(password) != nil {
		return Session{}, ErrInvalidCredentials
	}

	admin, err := service.repository.FindAdminByUsername(ctx, username)
	if errors.Is(err, ErrAdminNotFound) {
		_, _ = service.hasher.Verify(ctx, password, service.dummyHash)
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, fmt.Errorf("find administrator: %w", err)
	}
	matched, err := service.hasher.Verify(ctx, password, admin.PasswordHash)
	if err != nil {
		return Session{}, fmt.Errorf("verify administrator password: %w", err)
	}
	if !matched {
		return Session{}, ErrInvalidCredentials
	}

	now := service.now().UTC()
	rawRefresh, refreshHash, err := service.newRefreshToken()
	if err != nil {
		return Session{}, err
	}
	familyID := make([]byte, refreshFamilyIDBytes)
	if _, err := io.ReadFull(service.random, familyID); err != nil {
		return Session{}, fmt.Errorf("generate refresh token family: %w", err)
	}
	refreshExpiresAt := now.Add(RefreshTokenLifetime)
	accessToken, accessExpiresAt, err := service.tokens.Issue(admin.ID, admin.Username)
	if err != nil {
		return Session{}, err
	}
	if err := service.repository.CreateRefreshToken(ctx, RefreshToken{
		AdminID:          admin.ID,
		TokenHash:        refreshHash,
		FamilyID:         familyID,
		ExpiresAt:        refreshExpiresAt,
		SessionExpiresAt: now.Add(SessionAbsoluteLifetime),
		CreatedAt:        now,
	}); err != nil {
		return Session{}, fmt.Errorf("create refresh session: %w", err)
	}
	return Session{
		Admin:            admin,
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     rawRefresh,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (service *Service) Refresh(ctx context.Context, rawRefresh string) (Session, error) {
	currentHash, ok := refreshTokenHash(rawRefresh)
	if !ok {
		return Session{}, ErrInvalidSession
	}
	nextRaw, nextHash, err := service.newRefreshToken()
	if err != nil {
		return Session{}, err
	}
	now := service.now().UTC()
	rotation, err := service.repository.RotateRefreshToken(ctx, currentHash, nextHash, now)
	if err != nil {
		return Session{}, err
	}
	accessToken, accessExpiresAt, err := service.tokens.Issue(rotation.Admin.ID, rotation.Admin.Username)
	if err != nil {
		// 轮换已提交但 Access Token 无法签发时撤销 family，避免留下客户端不知道的活跃令牌。
		_ = service.repository.RevokeRefreshFamily(ctx, nextHash, now)
		return Session{}, err
	}
	return Session{
		Admin:            rotation.Admin,
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     nextRaw,
		RefreshExpiresAt: rotation.ExpiresAt,
	}, nil
}

func (service *Service) Logout(ctx context.Context, rawRefresh string) error {
	tokenHash, ok := refreshTokenHash(rawRefresh)
	if !ok {
		return nil
	}
	if err := service.repository.RevokeRefreshFamily(ctx, tokenHash, service.now().UTC()); err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	return nil
}

func (service *Service) newRefreshToken() (string, [sha256.Size]byte, error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := io.ReadFull(service.random, raw); err != nil {
		return "", [sha256.Size]byte{}, fmt.Errorf("generate refresh token: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return encoded, sha256.Sum256([]byte(encoded)), nil
}

func refreshTokenHash(raw string) ([sha256.Size]byte, bool) {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || len(decoded) != refreshTokenBytes {
		return [sha256.Size]byte{}, false
	}
	return sha256.Sum256([]byte(raw)), true
}
