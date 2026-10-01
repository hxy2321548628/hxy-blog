package post

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	ErrInvalidInput = errors.New("invalid post input")
	ErrSlugConflict = errors.New("post slug already exists")
	ErrNotDraft     = errors.New("post is not a draft")

	validSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// DraftInput 是创建和保存草稿共用的可编辑字段；状态与发布时间只能由发布动作改变。
type DraftInput struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	ContentMarkdown string `json:"contentMarkdown"`
}

// AdminSummary 包含后台列表需要的状态信息，不读取整篇 Markdown。
type AdminSummary struct {
	ID          uint64     `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"publishedAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// AdminDetail 是后台编辑器读取的完整文章。
type AdminDetail struct {
	ID              uint64     `json:"id"`
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	ContentMarkdown string     `json:"contentMarkdown"`
	Status          string     `json:"status"`
	PublishedAt     *time.Time `json:"publishedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type postRecord struct {
	ID              uint64
	Slug            string
	Title           string
	ContentMarkdown string
	Status          string
	PublishedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (postRecord) TableName() string { return "posts" }

func (repository *Repository) ListAdmin(ctx context.Context) ([]AdminSummary, error) {
	items := make([]AdminSummary, 0)
	if err := repository.database.WithContext(ctx).
		Table("posts").
		Select("id", "slug", "title", "status", "published_at", "updated_at").
		Order("updated_at DESC").
		Order("id DESC").
		Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("list administrator posts: %w", err)
	}
	return items, nil
}

func (repository *Repository) GetAdmin(ctx context.Context, id uint64) (AdminDetail, error) {
	var detail AdminDetail
	err := repository.database.WithContext(ctx).
		Table("posts").
		Where("id = ?", id).
		Take(&detail).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminDetail{}, ErrNotFound
	}
	if err != nil {
		return AdminDetail{}, fmt.Errorf("get administrator post: %w", err)
	}
	return detail, nil
}

func (repository *Repository) CreateDraft(ctx context.Context, input DraftInput) (AdminDetail, error) {
	now := time.Now().UTC()
	record := postRecord{
		Slug: input.Slug, Title: input.Title, ContentMarkdown: input.ContentMarkdown,
		Status: StatusDraft, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.database.WithContext(ctx).Create(&record).Error; err != nil {
		if isDuplicateKey(err) {
			return AdminDetail{}, ErrSlugConflict
		}
		return AdminDetail{}, fmt.Errorf("create draft post: %w", err)
	}
	return repository.GetAdmin(ctx, record.ID)
}

func (repository *Repository) UpdateDraft(ctx context.Context, id uint64, input DraftInput) (AdminDetail, error) {
	result := repository.database.WithContext(ctx).
		Table("posts").
		Where("id = ? AND status = ?", id, StatusDraft).
		Updates(map[string]any{
			"slug": input.Slug, "title": input.Title,
			"content_markdown": input.ContentMarkdown, "updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		if isDuplicateKey(result.Error) {
			return AdminDetail{}, ErrSlugConflict
		}
		return AdminDetail{}, fmt.Errorf("update draft post: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		detail, err := repository.GetAdmin(ctx, id)
		if err != nil {
			return AdminDetail{}, err
		}
		if detail.Status != StatusDraft {
			return AdminDetail{}, ErrNotDraft
		}
	}
	return repository.GetAdmin(ctx, id)
}

func (repository *Repository) Publish(ctx context.Context, id uint64) (AdminDetail, error) {
	now := time.Now().UTC()
	// 状态、标题和正文条件与更新放在同一条 SQL 中，避免并发保存空正文后仍被发布。
	result := repository.database.WithContext(ctx).
		Table("posts").
		Where("id = ? AND status = ?", id, StatusDraft).
		Where("CHAR_LENGTH(TRIM(title)) > 0 AND CHAR_LENGTH(TRIM(content_markdown)) > 0").
		Updates(map[string]any{
			"status": StatusPublished, "published_at": now, "updated_at": now,
		})
	if result.Error != nil {
		return AdminDetail{}, fmt.Errorf("publish draft post: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		detail, err := repository.GetAdmin(ctx, id)
		if err != nil {
			return AdminDetail{}, err
		}
		if detail.Status != StatusDraft {
			return AdminDetail{}, ErrNotDraft
		}
		return AdminDetail{}, ErrInvalidInput
	}
	return repository.GetAdmin(ctx, id)
}

func (service *Service) ListAdmin(ctx context.Context) ([]AdminSummary, error) {
	return service.repository.ListAdmin(ctx)
}

func (service *Service) GetAdmin(ctx context.Context, id uint64) (AdminDetail, error) {
	return service.repository.GetAdmin(ctx, id)
}

func (service *Service) CreateDraft(ctx context.Context, input DraftInput) (AdminDetail, error) {
	normalized, err := normalizeDraft(input)
	if err != nil {
		return AdminDetail{}, err
	}
	return service.repository.CreateDraft(ctx, normalized)
}

func (service *Service) UpdateDraft(ctx context.Context, id uint64, input DraftInput) (AdminDetail, error) {
	normalized, err := normalizeDraft(input)
	if err != nil {
		return AdminDetail{}, err
	}
	return service.repository.UpdateDraft(ctx, id, normalized)
}

func (service *Service) Publish(ctx context.Context, id uint64) (AdminDetail, error) {
	return service.repository.Publish(ctx, id)
}

func normalizeDraft(input DraftInput) (DraftInput, error) {
	input.Slug = strings.TrimSpace(input.Slug)
	input.Title = strings.TrimSpace(input.Title)
	if len(input.Slug) == 0 || len(input.Slug) > 200 || !validSlug.MatchString(input.Slug) {
		return DraftInput{}, ErrInvalidInput
	}
	if input.Title == "" || utf8.RuneCountInString(input.Title) > 200 {
		return DraftInput{}, ErrInvalidInput
	}
	return input, nil
}

func isDuplicateKey(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
