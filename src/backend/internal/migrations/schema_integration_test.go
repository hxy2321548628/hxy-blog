//go:build integration

package migrations_test

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestInitialSchemaEnforcesContentAndSessionRules(t *testing.T) {
	database := openIntegrationDatabase(t)
	defer database.Close()

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	// 测试数据只在事务中存在，避免污染开发者或 CI 的迁移数据库。
	t.Cleanup(func() { _ = transaction.Rollback() })
	// 单管理员约束要求测试先在事务内隔离既有数据；回滚会完整恢复本地管理员和会话。
	deleteRefreshTokensInDependencyOrder(t, transaction)
	if _, err := transaction.Exec("DELETE FROM admins"); err != nil {
		t.Fatalf("isolate administrators: %v", err)
	}

	assertTableStorage(t, transaction, "posts")
	assertTableStorage(t, transaction, "admins")
	assertTableStorage(t, transaction, "refresh_tokens")

	result, err := transaction.Exec(`
		INSERT INTO admins (username, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?)`,
		"admin", "$argon2id$integration-test-hash", time.Now().UTC(), time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert administrator: %v", err)
	}
	adminID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read administrator id: %v", err)
	}
	// MVP 只允许一个管理员；数据库约束必须覆盖并发初始化，不能只依赖应用先 count。
	assertInsertFails(t, transaction, 1062, `
		INSERT INTO admins (username, password_hash, created_at, updated_at)
		VALUES ('second-admin', '$argon2id$integration-test-hash', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`)

	now := time.Now().UTC()
	_, err = transaction.Exec(`
		INSERT INTO posts (slug, title, content_markdown, status, published_at, created_at, updated_at)
		VALUES (?, ?, ?, 'draft', NULL, ?, ?)`,
		"first-post", "第一篇文章", "# Hello", now, now,
	)
	if err != nil {
		t.Fatalf("insert draft post: %v", err)
	}

	assertInsertFails(t, transaction, 1062, `
		INSERT INTO posts (slug, title, content_markdown, status, published_at, created_at, updated_at)
		VALUES ('first-post', '重复 slug', '', 'draft', NULL, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`)
	assertInsertFails(t, transaction, 3819, `
		INSERT INTO posts (slug, title, content_markdown, status, published_at, created_at, updated_at)
		VALUES ('invalid-status', '无效状态', '', 'deleted', NULL, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`)
	assertInsertFails(t, transaction, 3819, `
		INSERT INTO posts (slug, title, content_markdown, status, published_at, created_at, updated_at)
		VALUES ('missing-published-at', '缺少发布时间', '', 'published', NULL, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`)

	expiresAt := now.Add(7 * 24 * time.Hour)
	sessionExpiresAt := now.Add(30 * 24 * time.Hour)
	result, err = transaction.Exec(`
		INSERT INTO refresh_tokens (
			admin_id, token_hash, family_id, parent_id, expires_at,
			session_expires_at, used_at, revoked_at, created_at
		) VALUES (?, UNHEX(REPEAT('11', 32)), UNHEX(REPEAT('22', 16)), NULL, ?, ?, NULL, NULL, ?)`,
		adminID, expiresAt, sessionExpiresAt, now,
	)
	if err != nil {
		t.Fatalf("insert refresh token: %v", err)
	}
	parentID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read refresh token id: %v", err)
	}

	// 哈希唯一性阻止同一明文 Token 被作为两条有效记录保存。
	assertInsertFails(t, transaction, 1062, fmt.Sprintf(`
		INSERT INTO refresh_tokens (
			admin_id, token_hash, family_id, parent_id, expires_at,
			session_expires_at, used_at, revoked_at, created_at
		) VALUES (%d, UNHEX(REPEAT('11', 32)), UNHEX(REPEAT('33', 16)), NULL,
			DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 1 DAY),
			DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 2 DAY), NULL, NULL, UTC_TIMESTAMP(6))`, adminID))

	_, err = transaction.Exec(`
		INSERT INTO refresh_tokens (
			admin_id, token_hash, family_id, parent_id, expires_at,
			session_expires_at, used_at, revoked_at, created_at
		) VALUES (?, UNHEX(REPEAT('44', 32)), UNHEX(REPEAT('22', 16)), ?, ?, ?, NULL, NULL, ?)`,
		adminID, parentID, expiresAt, sessionExpiresAt, now,
	)
	if err != nil {
		t.Fatalf("insert rotated refresh token: %v", err)
	}

	// 一个父 Token 只能轮换出一个子 Token，从数据库层阻止并发刷新产生分叉链。
	assertInsertFails(t, transaction, 1062, fmt.Sprintf(`
		INSERT INTO refresh_tokens (
			admin_id, token_hash, family_id, parent_id, expires_at,
			session_expires_at, used_at, revoked_at, created_at
		) VALUES (%d, UNHEX(REPEAT('55', 32)), UNHEX(REPEAT('22', 16)), %d,
			DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 1 DAY),
			DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 2 DAY), NULL, NULL, UTC_TIMESTAMP(6))`, adminID, parentID))

	// 会话必须归属真实管理员，避免留下无法撤销的孤立凭据。
	assertInsertFails(t, transaction, 1452, `
		INSERT INTO refresh_tokens (
			admin_id, token_hash, family_id, parent_id, expires_at,
			session_expires_at, used_at, revoked_at, created_at
		) VALUES (18446744073709551615, UNHEX(REPEAT('66', 32)), UNHEX(REPEAT('77', 16)), NULL,
			DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 1 DAY),
			DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 2 DAY), NULL, NULL, UTC_TIMESTAMP(6))`)
}

func deleteRefreshTokensInDependencyOrder(t *testing.T, transaction *sql.Tx) {
	t.Helper()
	for {
		result, err := transaction.Exec(`
			DELETE token
			FROM refresh_tokens AS token
			LEFT JOIN refresh_tokens AS child ON child.parent_id = token.id
			WHERE child.id IS NULL`)
		if err != nil {
			t.Fatalf("isolate refresh tokens: %v", err)
		}
		deleted, err := result.RowsAffected()
		if err != nil {
			t.Fatalf("read isolated refresh token count: %v", err)
		}
		if deleted == 0 {
			return
		}
	}
}

func openIntegrationDatabase(t *testing.T) *sql.DB {
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

	database, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		t.Fatalf("ping database: %v", err)
	}
	return database
}

func assertTableStorage(t *testing.T, transaction *sql.Tx, tableName string) {
	t.Helper()

	var engine string
	var collation string
	err := transaction.QueryRow(`
		SELECT engine, table_collation
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?`, tableName,
	).Scan(&engine, &collation)
	if err != nil {
		t.Fatalf("query storage for %s: %v", tableName, err)
	}
	if engine != "InnoDB" || collation != "utf8mb4_0900_ai_ci" {
		t.Fatalf("%s storage = %s/%s, want InnoDB/utf8mb4_0900_ai_ci", tableName, engine, collation)
	}
}

func assertInsertFails(t *testing.T, transaction *sql.Tx, expectedNumber uint16, statement string) {
	t.Helper()
	_, err := transaction.Exec(statement)
	if err == nil {
		t.Fatalf("statement unexpectedly succeeded: %s", statement)
	}

	var mysqlError *mysql.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != expectedNumber {
		t.Fatalf("statement error = %v, want MySQL error %d", err, expectedNumber)
	}
}
