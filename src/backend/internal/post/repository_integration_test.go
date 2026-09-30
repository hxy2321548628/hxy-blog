//go:build integration

package post

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestRepositoryListPublished(t *testing.T) {
	database := openIntegrationDatabase(t)
	transaction := database.Begin()
	if transaction.Error != nil {
		t.Fatalf("begin transaction: %v", transaction.Error)
	}
	// 回滚保证测试前的本地文章在测试后原样保留。
	t.Cleanup(func() { transaction.Rollback() })
	if err := transaction.Exec("DELETE FROM posts").Error; err != nil {
		t.Fatalf("clear posts in transaction: %v", err)
	}

	older := time.Date(2026, time.September, 30, 1, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	insertPost(t, transaction, "older-post", "较早文章", "published", &older)
	insertPost(t, transaction, "draft-post", "草稿", "draft", nil)
	insertPost(t, transaction, "newer-post", "较新文章", "published", &newer)

	items, total, err := NewRepository(transaction).ListPublished(
		context.Background(), Pagination{Page: 1, PageSize: 1},
	)
	if err != nil {
		t.Fatalf("ListPublished() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(items) != 1 || items[0].Slug != "newer-post" || !items[0].PublishedAt.Equal(newer) {
		t.Fatalf("items = %#v", items)
	}
}

func openIntegrationDatabase(t *testing.T) *gorm.DB {
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

func insertPost(t *testing.T, database *gorm.DB, slug, title, status string, publishedAt *time.Time) {
	t.Helper()
	now := time.Now().UTC()
	if err := database.Exec(`
		INSERT INTO posts (slug, title, content_markdown, status, published_at, created_at, updated_at)
		VALUES (?, ?, '# content', ?, ?, ?, ?)`,
		slug, title, status, publishedAt, now, now,
	).Error; err != nil {
		t.Fatalf("insert post %s: %v", slug, err)
	}
}
