package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTokenLifetime = 10 * time.Minute
	accessTokenIssuer   = "hxy-blog"
	accessTokenAudience = "hxy-blog-admin"
	tokenClockLeeway    = 30 * time.Second
	accessTokenIDBytes  = 16
)

type SigningKey struct {
	ID     string
	Secret []byte
}

type AccessClaims struct {
	AdminID  uint64 `json:"adminId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// AccessTokenManager 只使用当前密钥签发，并在短轮换窗口内接受上一把密钥。
type AccessTokenManager struct {
	current  SigningKey
	previous *SigningKey
	now      func() time.Time
	random   io.Reader
}

func NewAccessTokenManager(current SigningKey, previous *SigningKey) (*AccessTokenManager, error) {
	if err := validateSigningKey(current); err != nil {
		return nil, fmt.Errorf("current signing key: %w", err)
	}
	current = copySigningKey(current)

	var previousCopy *SigningKey
	if previous != nil {
		if err := validateSigningKey(*previous); err != nil {
			return nil, fmt.Errorf("previous signing key: %w", err)
		}
		if previous.ID == current.ID {
			return nil, errors.New("current and previous signing key IDs must differ")
		}
		copy := copySigningKey(*previous)
		previousCopy = &copy
	}
	return &AccessTokenManager{
		current:  current,
		previous: previousCopy,
		now:      time.Now,
		random:   rand.Reader,
	}, nil
}

func (manager *AccessTokenManager) Issue(adminID uint64, username string) (string, time.Time, error) {
	if adminID == 0 || username == "" {
		return "", time.Time{}, errors.New("admin identity is required")
	}
	now := manager.now().UTC()
	expiresAt := now.Add(AccessTokenLifetime)
	tokenIDBytes := make([]byte, accessTokenIDBytes)
	if _, err := io.ReadFull(manager.random, tokenIDBytes); err != nil {
		return "", time.Time{}, fmt.Errorf("generate access token ID: %w", err)
	}
	claims := AccessClaims{
		AdminID:  adminID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    accessTokenIssuer,
			Subject:   strconv.FormatUint(adminID, 10),
			Audience:  jwt.ClaimStrings{accessTokenAudience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        hex.EncodeToString(tokenIDBytes),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = manager.current.ID
	raw, err := token.SignedString(manager.current.Secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return raw, expiresAt, nil
}

func (manager *AccessTokenManager) Parse(raw string) (AccessClaims, error) {
	var claims AccessClaims
	token, err := jwt.ParseWithClaims(
		raw,
		&claims,
		manager.keyForToken,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(accessTokenIssuer),
		jwt.WithAudience(accessTokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(tokenClockLeeway),
		jwt.WithTimeFunc(manager.now),
	)
	if err != nil || !token.Valid {
		return AccessClaims{}, fmt.Errorf("validate access token: %w", err)
	}
	if claims.AdminID == 0 || claims.Username == "" || claims.ID == "" || claims.IssuedAt == nil {
		return AccessClaims{}, errors.New("access token is missing required claims")
	}
	if claims.Subject != strconv.FormatUint(claims.AdminID, 10) {
		return AccessClaims{}, errors.New("access token subject does not match administrator")
	}
	return claims, nil
}

func (manager *AccessTokenManager) keyForToken(token *jwt.Token) (any, error) {
	if token.Method != jwt.SigningMethodHS256 {
		return nil, fmt.Errorf("unexpected signing algorithm %q", token.Method.Alg())
	}
	keyID, ok := token.Header["kid"].(string)
	if !ok || keyID == "" {
		return nil, errors.New("access token key ID is missing")
	}
	if keyID == manager.current.ID {
		return manager.current.Secret, nil
	}
	if manager.previous != nil && keyID == manager.previous.ID {
		return manager.previous.Secret, nil
	}
	return nil, errors.New("access token key ID is unknown")
}

func validateSigningKey(key SigningKey) error {
	if key.ID == "" {
		return errors.New("key ID is required")
	}
	if len(key.Secret) < 32 {
		return errors.New("HS256 secret must contain at least 32 bytes")
	}
	return nil
}

func copySigningKey(key SigningKey) SigningKey {
	return SigningKey{ID: key.ID, Secret: append([]byte(nil), key.Secret...)}
}
