//go:build integration

package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestRepositoryRefreshRotationAndReplayRevocation(t *testing.T) {
	database := openAuthIntegrationDatabase(t)
	username := fmt.Sprintf("auth-integration-%d", time.Now().UnixNano())
	adminID, username, createdAdmin := ensureAuthTestAdmin(t, database, username)
	familyDigest := sha256.Sum256([]byte(username))
	familyID := append([]byte(nil), familyDigest[:refreshFamilyIDBytes]...)
	t.Cleanup(func() {
		deleteAuthTestFamily(t, database, familyID)
		if createdAdmin {
			database.Exec("DELETE FROM admins WHERE id = ?", adminID)
		}
	})

	now := time.Now().UTC().Truncate(time.Microsecond)
	currentHash := sha256.Sum256([]byte(username + "-current-refresh-token"))
	repository := NewRepository(database)
	if err := repository.CreateRefreshToken(context.Background(), RefreshToken{
		AdminID:          adminID,
		TokenHash:        currentHash,
		FamilyID:         familyID,
		ExpiresAt:        now.Add(RefreshTokenLifetime),
		SessionExpiresAt: now.Add(SessionAbsoluteLifetime),
		CreatedAt:        now,
	}); err != nil {
		t.Fatalf("CreateRefreshToken() error = %v", err)
	}

	nextHashes := [][sha256.Size]byte{
		sha256.Sum256([]byte(username + "-next-refresh-token-a")),
		sha256.Sum256([]byte(username + "-next-refresh-token-b")),
	}
	type outcome struct {
		result RotationResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	var start sync.WaitGroup
	start.Add(2)
	for _, nextHash := range nextHashes {
		go func(hash [sha256.Size]byte) {
			start.Done()
			start.Wait()
			result, err := repository.RotateRefreshToken(context.Background(), currentHash, hash, now.Add(time.Second))
			outcomes <- outcome{result: result, err: err}
		}(nextHash)
	}

	var successCount int
	var replayCount int
	for range nextHashes {
		result := <-outcomes
		switch {
		case result.err == nil:
			successCount++
			if result.result.Admin.ID != adminID || result.result.Admin.Username != username {
				t.Fatalf("rotation admin = %#v", result.result.Admin)
			}
		case errors.Is(result.err, ErrRefreshReplay):
			replayCount++
		default:
			t.Fatalf("RotateRefreshToken() error = %v", result.err)
		}
	}
	if successCount != 1 || replayCount != 1 {
		t.Fatalf("rotation outcomes: success=%d replay=%d", successCount, replayCount)
	}

	var active int64
	if err := database.Table("refresh_tokens").
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Count(&active).Error; err != nil {
		t.Fatalf("count active refresh tokens: %v", err)
	}
	if active != 0 {
		t.Fatalf("active refresh tokens after replay = %d, want 0", active)
	}
}

func deleteAuthTestFamily(t *testing.T, database *gorm.DB, familyID []byte) {
	t.Helper()
	for {
		result := database.Exec(`
			DELETE token
			FROM refresh_tokens AS token
			LEFT JOIN refresh_tokens AS child ON child.parent_id = token.id
			WHERE token.family_id = ? AND child.id IS NULL`, familyID)
		if result.Error != nil {
			t.Errorf("clean refresh token family: %v", result.Error)
			return
		}
		if result.RowsAffected == 0 {
			return
		}
	}
}

func openAuthIntegrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	required := func(key string) string {
		value := os.Getenv(key)
		if value == "" {
			t.Fatalf("missing required integration test environment variable %s", key)
		}
		return value
	}
	config := mysql.NewConfig()
	config.User = required("MYSQL_USER")
	config.Passwd = required("MYSQL_PASSWORD")
	config.Net = "tcp"
	config.Addr = fmt.Sprintf("%s:%s", required("MYSQL_HOST"), required("MYSQL_PORT"))
	config.DBName = required("MYSQL_DATABASE")
	config.ParseTime = true
	config.Loc = time.UTC
	database, err := gorm.Open(gormmysql.Open(config.FormatDSN()))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return database
}

func ensureAuthTestAdmin(t *testing.T, database *gorm.DB, username string) (uint64, string, bool) {
	t.Helper()
	var existing struct {
		ID       uint64
		Username string
	}
	err := database.Table("admins").Select("id", "username").Order("id").Take(&existing).Error
	if err == nil {
		return existing.ID, existing.Username, false
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("query existing administrator: %v", err)
	}

	now := time.Now().UTC()
	result := database.Exec(
		"INSERT INTO admins (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)",
		username,
		"$argon2id$integration-test-hash",
		now,
		now,
	)
	if result.Error != nil {
		t.Fatalf("insert administrator: %v", result.Error)
	}
	var admin struct {
		ID uint64
	}
	if err := database.Table("admins").Select("id").Where("username = ?", username).Take(&admin).Error; err != nil {
		t.Fatalf("load administrator ID: %v", err)
	}
	return admin.ID, username, true
}
