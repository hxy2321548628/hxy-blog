// Package post 实现文章的业务与持久化边界。
package post

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("post not found")

const (
	DefaultPage     = 1
	DefaultPageSize = 10
	MaxPageSize     = 50
)

// Pagination 是已经过 HTTP 边界校验的分页参数。
type Pagination struct {
	Page     int
	PageSize int
}

// Summary 只包含列表页需要的公开字段，避免为每一项读取整篇 Markdown。
type Summary struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"publishedAt"`
}

// Detail 是访客详情页可以读取的完整公开文章，不包含内部状态和更新时间。
type Detail struct {
	Slug            string    `json:"slug"`
	Title           string    `json:"title"`
	ContentMarkdown string    `json:"contentMarkdown"`
	PublishedAt     time.Time `json:"publishedAt"`
}

// ListResult 同时返回当前页和总数，前端可以不额外请求地判断下一页。
type ListResult struct {
	Items    []Summary `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int64     `json:"total"`
}

// Repository 封装文章查询语义，不向 HTTP 层暴露 GORM。
type Repository struct {
	database *gorm.DB
}

func NewRepository(database *gorm.DB) *Repository {
	return &Repository{database: database}
}

func (repository *Repository) ListPublished(ctx context.Context, pagination Pagination) ([]Summary, int64, error) {
	query := repository.database.WithContext(ctx).Table("posts").Where("status = ?", "published")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count published posts: %w", err)
	}

	// 总数为零时仍返回 [] 而不是 null，保持 API 数组契约稳定。
	items := make([]Summary, 0)
	offset := (pagination.Page - 1) * pagination.PageSize
	if err := query.
		Select("slug", "title", "published_at").
		Order("published_at DESC").
		Order("id DESC").
		Limit(pagination.PageSize).
		Offset(offset).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list published posts: %w", err)
	}
	return items, total, nil
}

func (repository *Repository) GetPublishedBySlug(ctx context.Context, slug string) (Detail, error) {
	var detail Detail
	err := repository.database.WithContext(ctx).
		Table("posts").
		Select("slug", "title", "content_markdown", "published_at").
		Where("slug = ? AND status = ?", slug, "published").
		Take(&detail).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 草稿与不存在的文章使用同一结果，避免公开接口泄露未发布 slug。
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, fmt.Errorf("get published post by slug: %w", err)
	}
	return detail, nil
}

// Service 作为 Handler 与持久化的边界，后续发布规则不会渗入 HTTP 层。
type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) ListPublished(ctx context.Context, pagination Pagination) (ListResult, error) {
	items, total, err := service.repository.ListPublished(ctx, pagination)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{
		Items:    items,
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Total:    total,
	}, nil
}

func (service *Service) GetPublishedBySlug(ctx context.Context, slug string) (Detail, error) {
	return service.repository.GetPublishedBySlug(ctx, slug)
}
