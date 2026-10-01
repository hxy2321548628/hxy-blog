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
	MaxTagsPerPost  = 10
)

// DraftInput 是创建和保存草稿共用的可编辑字段；状态与发布时间只能由发布动作改变。
type DraftInput struct {
	Slug            string   `json:"slug"`
	Title           string   `json:"title"`
	ContentMarkdown string   `json:"contentMarkdown"`
	Category        Category `json:"category"`
	Tags            []string `json:"tags"`
}

// AdminSummary 包含后台列表需要的状态信息，不读取整篇 Markdown。
type AdminSummary struct {
	ID          uint64     `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"publishedAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	Category    Category   `json:"category"`
	Tags        []string   `json:"tags"`
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
	Category        Category   `json:"category"`
	Tags            []string   `json:"tags"`
}

type postRecord struct {
	ID              uint64
	Slug            string
	Title           string
	ContentMarkdown string
	CategoryID      uint64
	Status          string
	PublishedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (postRecord) TableName() string { return "posts" }

func (repository *Repository) ListAdmin(ctx context.Context) ([]AdminSummary, error) {
	rows := make([]adminSummaryRow, 0)
	if err := repository.database.WithContext(ctx).
		Table("posts AS post").
		Select("post.id", "post.slug", "post.title", "post.status", "post.published_at", "post.updated_at", "category.slug AS category_slug", "category.name AS category_name").
		Joins("JOIN categories AS category ON category.id = post.category_id").
		Order("post.updated_at DESC").
		Order("post.id DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list administrator posts: %w", err)
	}
	return repository.adminSummariesWithTags(ctx, rows)
}

func (repository *Repository) GetAdmin(ctx context.Context, id uint64) (AdminDetail, error) {
	var row adminDetailRow
	err := repository.database.WithContext(ctx).
		Table("posts AS post").
		Select("post.id", "post.slug", "post.title", "post.content_markdown", "post.status", "post.published_at", "post.created_at", "post.updated_at", "category.slug AS category_slug", "category.name AS category_name").
		Joins("JOIN categories AS category ON category.id = post.category_id").
		Where("post.id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminDetail{}, ErrNotFound
	}
	if err != nil {
		return AdminDetail{}, fmt.Errorf("get administrator post: %w", err)
	}
	return repository.adminDetailWithTags(ctx, row)
}

func (repository *Repository) CreateDraft(ctx context.Context, input DraftInput) (AdminDetail, error) {
	var postID uint64
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		categoryID, tagIDs, err := resolveTaxonomy(ctx, transaction, input)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		record := postRecord{
			Slug: input.Slug, Title: input.Title, ContentMarkdown: input.ContentMarkdown,
			CategoryID: categoryID, Status: StatusDraft, CreatedAt: now, UpdatedAt: now,
		}
		if err := transaction.WithContext(ctx).Create(&record).Error; err != nil {
			return err
		}
		postID = record.ID
		return replacePostTags(ctx, transaction, record.ID, tagIDs)
	})
	if err != nil {
		if isDuplicateKey(err) {
			return AdminDetail{}, ErrSlugConflict
		}
		return AdminDetail{}, fmt.Errorf("create draft post: %w", err)
	}
	return repository.GetAdmin(ctx, postID)
}

func (repository *Repository) Update(ctx context.Context, id uint64, input DraftInput) (AdminDetail, error) {
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		categoryID, tagIDs, err := resolveTaxonomy(ctx, transaction, input)
		if err != nil {
			return err
		}
		result := transaction.WithContext(ctx).
			Table("posts").
			Where("id = ?", id).
			Updates(map[string]any{
				// 已发布文章的 slug 是稳定公开地址，编辑正文时不应让旧链接失效。
				"slug":             gorm.Expr("CASE WHEN status = ? THEN slug ELSE ? END", StatusPublished, input.Slug),
				"title":            input.Title,
				"content_markdown": input.ContentMarkdown,
				"category_id":      categoryID,
				"updated_at":       time.Now().UTC(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return replacePostTags(ctx, transaction, id, tagIDs)
	})
	if err != nil {
		if isDuplicateKey(err) {
			return AdminDetail{}, ErrSlugConflict
		}
		if errors.Is(err, ErrNotFound) {
			return AdminDetail{}, err
		}
		return AdminDetail{}, fmt.Errorf("update post: %w", err)
	}
	return repository.GetAdmin(ctx, id)
}

func (repository *Repository) Delete(ctx context.Context, id uint64) error {
	result := repository.database.WithContext(ctx).Where("id = ?", id).Delete(&postRecord{})
	if result.Error != nil {
		return fmt.Errorf("delete post: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
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

func (service *Service) Update(ctx context.Context, id uint64, input DraftInput) (AdminDetail, error) {
	normalized, err := normalizeDraft(input)
	if err != nil {
		return AdminDetail{}, err
	}
	return service.repository.Update(ctx, id, normalized)
}

func (service *Service) Publish(ctx context.Context, id uint64) (AdminDetail, error) {
	return service.repository.Publish(ctx, id)
}

func (service *Service) Delete(ctx context.Context, id uint64) error {
	return service.repository.Delete(ctx, id)
}

func normalizeDraft(input DraftInput) (DraftInput, error) {
	input.Slug = strings.TrimSpace(input.Slug)
	input.Title = strings.TrimSpace(input.Title)
	input.Category.Slug = strings.TrimSpace(input.Category.Slug)
	input.Category.Name = strings.TrimSpace(input.Category.Name)
	if len(input.Slug) == 0 || len(input.Slug) > 200 || !validSlug.MatchString(input.Slug) {
		return DraftInput{}, ErrInvalidInput
	}
	if input.Title == "" || utf8.RuneCountInString(input.Title) > 200 {
		return DraftInput{}, ErrInvalidInput
	}
	if len(input.Category.Slug) == 0 || len(input.Category.Slug) > 100 || !validSlug.MatchString(input.Category.Slug) {
		return DraftInput{}, ErrInvalidInput
	}
	if input.Category.Name == "" || utf8.RuneCountInString(input.Category.Name) > 80 {
		return DraftInput{}, ErrInvalidInput
	}
	if len(input.Tags) > MaxTagsPerPost {
		return DraftInput{}, ErrInvalidInput
	}
	normalizedTags := make([]string, 0, len(input.Tags))
	seenTags := make(map[string]struct{}, len(input.Tags))
	for _, tag := range input.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || utf8.RuneCountInString(tag) > 40 {
			return DraftInput{}, ErrInvalidInput
		}
		key := strings.ToLower(tag)
		if _, exists := seenTags[key]; exists {
			continue
		}
		seenTags[key] = struct{}{}
		normalizedTags = append(normalizedTags, tag)
	}
	input.Tags = normalizedTags
	return input, nil
}

// ValidCategorySlug 供 HTTP 边界复用与写入一致的分类路由规则。
func ValidCategorySlug(slug string) bool {
	return len(slug) > 0 && len(slug) <= 100 && validSlug.MatchString(slug)
}

func isDuplicateKey(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
