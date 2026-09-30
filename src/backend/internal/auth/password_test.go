package auth

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"
)

func TestPasswordHasherUsesRequiredArgon2idParameters(t *testing.T) {
	hasher := NewPasswordHasher(rand.Reader)

	encoded, err := hasher.Hash(context.Background(), "correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=1$") {
		t.Fatalf("encoded hash uses unexpected parameters: %q", encoded)
	}

	matched, err := hasher.Verify(context.Background(), "correct horse battery staple", encoded)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !matched {
		t.Fatal("Verify() = false, want true")
	}

	matched, err = hasher.Verify(context.Background(), "wrong password", encoded)
	if err != nil {
		t.Fatalf("Verify() wrong password error = %v", err)
	}
	if matched {
		t.Fatal("Verify() wrong password = true, want false")
	}
}

func TestPasswordHasherRejectsMalformedHash(t *testing.T) {
	hasher := NewPasswordHasher(rand.Reader)

	if _, err := hasher.Verify(context.Background(), "password", "$argon2id$broken"); err == nil {
		t.Fatal("Verify() error = nil, want malformed hash error")
	}
}

func TestPasswordHasherRejectsInvalidPasswordLength(t *testing.T) {
	hasher := NewPasswordHasher(rand.Reader)

	for _, password := range []string{"", strings.Repeat("x", MaxPasswordBytes+1)} {
		if _, err := hasher.Hash(context.Background(), password); err == nil {
			t.Fatalf("Hash() error = nil for password length %d", len(password))
		}
	}
}
