//go:build integration

package post

import (
	"context"
	"errors"
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
	insertPost(t, transaction, "older-post", "较早文章", "# older", "published", &older)
	insertPost(t, transaction, "draft-post", "草稿", "# draft", "draft", nil)
	insertPost(t, transaction, "newer-post", "较新文章", "# newer", "published", &newer)

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

func TestRepositoryGetPublishedBySlug(t *testing.T) {
	database := openIntegrationDatabase(t)
	transaction := database.Begin()
	if transaction.Error != nil {
		t.Fatalf("begin transaction: %v", transaction.Error)
	}
	t.Cleanup(func() { transaction.Rollback() })
	if err := transaction.Exec("DELETE FROM posts").Error; err != nil {
		t.Fatalf("clear posts in transaction: %v", err)
	}

	publishedAt := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	insertPost(t, transaction, "published-post", "公开文章", "# 正文", "published", &publishedAt)
	insertPost(t, transaction, "draft-post", "草稿", "# 不应公开", "draft", nil)

	repository := NewRepository(transaction)
	detail, err := repository.GetPublishedBySlug(context.Background(), "published-post")
	if err != nil {
		t.Fatalf("GetPublishedBySlug() error = %v", err)
	}
	if detail.Slug != "published-post" || detail.Title != "公开文章" || detail.ContentMarkdown != "# 正文" {
		t.Fatalf("detail = %#v", detail)
	}
	if !detail.PublishedAt.Equal(publishedAt) {
		t.Fatalf("publishedAt = %v, want %v", detail.PublishedAt, publishedAt)
	}

	for _, slug := range []string{"draft-post", "missing-post"} {
		if _, err := repository.GetPublishedBySlug(context.Background(), slug); !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetPublishedBySlug(%q) error = %v, want ErrNotFound", slug, err)
		}
	}
}

func TestAdministratorDraftLifecycle(t *testing.T) {
	database := openIntegrationDatabase(t)
	transaction := database.Begin()
	if transaction.Error != nil {
		t.Fatalf("begin transaction: %v", transaction.Error)
	}
	t.Cleanup(func() { transaction.Rollback() })
	if err := transaction.Exec("DELETE FROM posts").Error; err != nil {
		t.Fatalf("clear posts in transaction: %v", err)
	}

	service := NewService(NewRepository(transaction))
	draft, err := service.CreateDraft(context.Background(), DraftInput{
		Slug: "first-draft", Title: "第一篇草稿", ContentMarkdown: "",
		Category: Category{Slug: "engineering", Name: "工程"}, Tags: []string{"Go", "MySQL"},
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if draft.Status != StatusDraft || draft.PublishedAt != nil {
		t.Fatalf("draft = %#v", draft)
	}

	if _, err := service.CreateDraft(context.Background(), DraftInput{
		Slug: "first-draft", Title: "重复 slug", Category: Category{Slug: "engineering", Name: "工程"},
	}); !errors.Is(err, ErrSlugConflict) {
		t.Fatalf("duplicate CreateDraft() error = %v, want ErrSlugConflict", err)
	}
	if _, err := service.Publish(context.Background(), draft.ID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty Publish() error = %v, want ErrInvalidInput", err)
	}

	updated, err := service.Update(context.Background(), draft.ID, DraftInput{
		Slug: "first-post", Title: "第一篇文章", ContentMarkdown: "# 正文",
		Category: Category{Slug: "engineering", Name: "工程"}, Tags: []string{"Go"},
	})
	if err != nil {
		t.Fatalf("Update() draft error = %v", err)
	}
	if updated.Slug != "first-post" || updated.ContentMarkdown != "# 正文" {
		t.Fatalf("updated = %#v", updated)
	}

	published, err := service.Publish(context.Background(), draft.ID)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.Status != StatusPublished || published.PublishedAt == nil {
		t.Fatalf("published = %#v", published)
	}
	public, err := service.GetPublishedBySlug(context.Background(), "first-post")
	if err != nil || public.ContentMarkdown != "# 正文" {
		t.Fatalf("GetPublishedBySlug() = %#v, %v", public, err)
	}
	originalPublishedAt := *published.PublishedAt
	published, err = service.Update(context.Background(), draft.ID, DraftInput{
		Slug: "changed", Title: "发布后的新标题", ContentMarkdown: "# 发布后的新正文",
		Category: Category{Slug: "notes", Name: "笔记"}, Tags: []string{"Go", "React"},
	})
	if err != nil {
		t.Fatalf("Update() published error = %v", err)
	}
	if published.Slug != "first-post" || published.Title != "发布后的新标题" || published.ContentMarkdown != "# 发布后的新正文" {
		t.Fatalf("published update = %#v", published)
	}
	if published.Category.Slug != "notes" || len(published.Tags) != 2 {
		t.Fatalf("published taxonomy = %#v, %#v", published.Category, published.Tags)
	}
	if published.PublishedAt == nil || !published.PublishedAt.Equal(originalPublishedAt) {
		t.Fatalf("publishedAt = %v, want unchanged %v", published.PublishedAt, originalPublishedAt)
	}
	public, err = service.GetPublishedBySlug(context.Background(), "first-post")
	if err != nil || public.Category.Slug != "notes" || len(public.Tags) != 2 {
		t.Fatalf("updated GetPublishedBySlug() = %#v, %v", public, err)
	}
	filtered, err := service.ListPublished(context.Background(), Pagination{
		Page: 1, PageSize: 10, CategorySlug: "notes",
	})
	if err != nil || filtered.Total != 1 || len(filtered.Items) != 1 {
		t.Fatalf("filtered ListPublished() = %#v, %v", filtered, err)
	}
	categories, err := service.ListPublishedCategories(context.Background())
	if err != nil || len(categories) != 1 || categories[0].Slug != "notes" || categories[0].PostCount != 1 {
		t.Fatalf("ListPublishedCategories() = %#v, %v", categories, err)
	}

	items, err := service.ListAdmin(context.Background())
	if err != nil || len(items) != 1 || items[0].Status != StatusPublished {
		t.Fatalf("ListAdmin() = %#v, %v", items, err)
	}
	if err := service.Delete(context.Background(), draft.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.GetAdmin(context.Background(), draft.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAdmin() after delete error = %v, want ErrNotFound", err)
	}
	if err := service.Delete(context.Background(), draft.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete() error = %v, want ErrNotFound", err)
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

func insertPost(t *testing.T, database *gorm.DB, slug, title, content, status string, publishedAt *time.Time) {
	t.Helper()
	now := time.Now().UTC()
	if err := database.Exec(`
		INSERT INTO posts (slug, title, content_markdown, category_id, status, published_at, created_at, updated_at)
		VALUES (?, ?, ?, (SELECT id FROM categories WHERE slug = 'uncategorized'), ?, ?, ?, ?)`,
		slug, title, content, status, publishedAt, now, now,
	).Error; err != nil {
		t.Fatalf("insert post %s: %v", slug, err)
	}
}
