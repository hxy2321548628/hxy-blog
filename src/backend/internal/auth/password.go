// Package auth 实现管理员密码与会话安全边界。
package auth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	MaxPasswordBytes = 1024
	argonMemoryKiB   = 64 * 1024
	argonIterations  = 3
	argonParallelism = 1
	argonSaltBytes   = 16
	argonKeyBytes    = 32
)

// PasswordHasher 将高内存消耗的 Argon2id 并发限制在两个，保护 256 MiB API 容器。
type PasswordHasher struct {
	random io.Reader
	slots  chan struct{}
}

func NewPasswordHasher(random io.Reader) *PasswordHasher {
	return &PasswordHasher{random: random, slots: make(chan struct{}, 2)}
}

func (hasher *PasswordHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := validatePasswordLength(password); err != nil {
		return "", err
	}
	salt := make([]byte, argonSaltBytes)
	if _, err := io.ReadFull(hasher.random, salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key, err := hasher.derive(ctx, password, salt)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemoryKiB,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func (hasher *PasswordHasher) Verify(ctx context.Context, password, encoded string) (bool, error) {
	if err := validatePasswordLength(password); err != nil {
		return false, err
	}
	salt, expected, err := parsePasswordHash(encoded)
	if err != nil {
		return false, err
	}
	actual, err := hasher.derive(ctx, password, salt)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func (hasher *PasswordHasher) derive(ctx context.Context, password string, salt []byte) ([]byte, error) {
	select {
	case hasher.slots <- struct{}{}:
		defer func() { <-hasher.slots }()
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for password hash slot: %w", ctx.Err())
	}
	return argon2.IDKey(
		[]byte(password),
		salt,
		argonIterations,
		argonMemoryKiB,
		argonParallelism,
		argonKeyBytes,
	), nil
}

func validatePasswordLength(password string) error {
	if len(password) == 0 || len(password) > MaxPasswordBytes {
		return fmt.Errorf("password length must be between 1 and %d bytes", MaxPasswordBytes)
	}
	return nil
}

func parsePasswordHash(encoded string) ([]byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return nil, nil, errors.New("invalid Argon2id hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, nil, errors.New("unsupported Argon2id version")
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return nil, nil, errors.New("invalid Argon2id parameters")
	}
	// 只接受经过目标服务器基准验证的固定参数，避免损坏记录触发无界内存分配。
	if memory != argonMemoryKiB || iterations != argonIterations || parallelism != argonParallelism {
		return nil, nil, errors.New("unsupported Argon2id parameters")
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != argonSaltBytes {
		return nil, nil, errors.New("invalid Argon2id salt")
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(key) != argonKeyBytes {
		return nil, nil, errors.New("invalid Argon2id key")
	}
	return salt, key, nil
}
