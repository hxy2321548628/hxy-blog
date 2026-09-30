// Package post 实现文章的业务与持久化边界。
package post

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

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
