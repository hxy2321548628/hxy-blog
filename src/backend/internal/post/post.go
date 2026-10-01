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
	Page         int
	PageSize     int
	CategorySlug string
}

// Category 是文章所属的单一分类，slug 用于稳定筛选，name 用于展示。
type Category struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// CategorySummary 只统计已发布文章，避免侧边栏泄露草稿分类。
type CategorySummary struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	PostCount int64  `json:"postCount"`
}

// Summary 只包含列表页需要的公开字段，避免为每一项读取整篇 Markdown。
type Summary struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"publishedAt"`
	Category    Category  `json:"category"`
	Tags        []string  `json:"tags"`
}

// Detail 是访客详情页可以读取的完整公开文章，不包含内部状态和更新时间。
type Detail struct {
	Slug            string    `json:"slug"`
	Title           string    `json:"title"`
	ContentMarkdown string    `json:"contentMarkdown"`
	PublishedAt     time.Time `json:"publishedAt"`
	Category        Category  `json:"category"`
	Tags            []string  `json:"tags"`
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
	query := repository.database.WithContext(ctx).
		Table("posts AS post").
		Joins("JOIN categories AS category ON category.id = post.category_id").
		Where("post.status = ?", StatusPublished)
	if pagination.CategorySlug != "" {
		query = query.Where("category.slug = ?", pagination.CategorySlug)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count published posts: %w", err)
	}

	// 总数为零时仍返回 [] 而不是 null，保持 API 数组契约稳定。
	rows := make([]summaryRow, 0)
	offset := (pagination.Page - 1) * pagination.PageSize
	if err := query.
		Select("post.id", "post.slug", "post.title", "post.published_at", "category.slug AS category_slug", "category.name AS category_name").
		Order("post.published_at DESC").
		Order("post.id DESC").
		Limit(pagination.PageSize).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list published posts: %w", err)
	}
	items, err := repository.summariesWithTags(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (repository *Repository) GetPublishedBySlug(ctx context.Context, slug string) (Detail, error) {
	var row detailRow
	err := repository.database.WithContext(ctx).
		Table("posts AS post").
		Select("post.id", "post.slug", "post.title", "post.content_markdown", "post.published_at", "category.slug AS category_slug", "category.name AS category_name").
		Joins("JOIN categories AS category ON category.id = post.category_id").
		Where("post.slug = ? AND post.status = ?", slug, StatusPublished).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 草稿与不存在的文章使用同一结果，避免公开接口泄露未发布 slug。
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, fmt.Errorf("get published post by slug: %w", err)
	}
	tags, err := repository.tagsByPostIDs(ctx, []uint64{row.ID})
	if err != nil {
		return Detail{}, err
	}
	return Detail{
		Slug: row.Slug, Title: row.Title, ContentMarkdown: row.ContentMarkdown,
		PublishedAt: row.PublishedAt,
		Category:    Category{Slug: row.CategorySlug, Name: row.CategoryName},
		Tags:        tags[row.ID],
	}, nil
}

func (repository *Repository) ListPublishedCategories(ctx context.Context) ([]CategorySummary, error) {
	items := make([]CategorySummary, 0)
	if err := repository.database.WithContext(ctx).
		Table("categories AS category").
		Select("category.slug", "category.name", "COUNT(post.id) AS post_count").
		Joins("JOIN posts AS post ON post.category_id = category.id AND post.status = ?", StatusPublished).
		Group("category.id, category.slug, category.name").
		Order("category.name ASC").
		Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("list published categories: %w", err)
	}
	return items, nil
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

func (service *Service) ListPublishedCategories(ctx context.Context) ([]CategorySummary, error) {
	return service.repository.ListPublishedCategories(ctx)
}
