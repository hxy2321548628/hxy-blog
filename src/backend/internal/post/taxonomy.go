package post

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type summaryRow struct {
	ID           uint64
	Slug         string
	Title        string
	PublishedAt  time.Time
	CategorySlug string
	CategoryName string
}

type detailRow struct {
	ID              uint64
	Slug            string
	Title           string
	ContentMarkdown string
	PublishedAt     time.Time
	CategorySlug    string
	CategoryName    string
}

type adminSummaryRow struct {
	ID           uint64
	Slug         string
	Title        string
	Status       string
	PublishedAt  *time.Time
	UpdatedAt    time.Time
	CategorySlug string
	CategoryName string
}

type adminDetailRow struct {
	ID              uint64
	Slug            string
	Title           string
	ContentMarkdown string
	Status          string
	PublishedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CategorySlug    string
	CategoryName    string
}

type tagRow struct {
	PostID uint64
	Name   string
}

type categoryRecord struct {
	ID        uint64
	Slug      string
	Name      string
	CreatedAt time.Time
}

func (categoryRecord) TableName() string { return "categories" }

type tagRecord struct {
	ID        uint64
	Name      string
	CreatedAt time.Time
}

func (tagRecord) TableName() string { return "tags" }

type postTagRecord struct {
	PostID uint64
	TagID  uint64
}

func (postTagRecord) TableName() string { return "post_tags" }

func (repository *Repository) summariesWithTags(ctx context.Context, rows []summaryRow) ([]Summary, error) {
	postIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		postIDs = append(postIDs, row.ID)
	}
	tags, err := repository.tagsByPostIDs(ctx, postIDs)
	if err != nil {
		return nil, err
	}

	items := make([]Summary, 0, len(rows))
	for _, row := range rows {
		items = append(items, Summary{
			Slug: row.Slug, Title: row.Title, PublishedAt: row.PublishedAt,
			Category: Category{Slug: row.CategorySlug, Name: row.CategoryName},
			Tags:     tags[row.ID],
		})
	}
	return items, nil
}

func (repository *Repository) adminSummariesWithTags(ctx context.Context, rows []adminSummaryRow) ([]AdminSummary, error) {
	postIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		postIDs = append(postIDs, row.ID)
	}
	tags, err := repository.tagsByPostIDs(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	items := make([]AdminSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, AdminSummary{
			ID: row.ID, Slug: row.Slug, Title: row.Title, Status: row.Status,
			PublishedAt: row.PublishedAt, UpdatedAt: row.UpdatedAt,
			Category: Category{Slug: row.CategorySlug, Name: row.CategoryName},
			Tags:     tags[row.ID],
		})
	}
	return items, nil
}

func (repository *Repository) adminDetailWithTags(ctx context.Context, row adminDetailRow) (AdminDetail, error) {
	tags, err := repository.tagsByPostIDs(ctx, []uint64{row.ID})
	if err != nil {
		return AdminDetail{}, err
	}
	return AdminDetail{
		ID: row.ID, Slug: row.Slug, Title: row.Title, ContentMarkdown: row.ContentMarkdown,
		Status: row.Status, PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Category: Category{Slug: row.CategorySlug, Name: row.CategoryName},
		Tags:     tags[row.ID],
	}, nil
}

func (repository *Repository) tagsByPostIDs(ctx context.Context, postIDs []uint64) (map[uint64][]string, error) {
	result := make(map[uint64][]string, len(postIDs))
	for _, postID := range postIDs {
		// 即使没有标签也返回 []，避免 API 在空数组和 null 之间漂移。
		result[postID] = []string{}
	}
	if len(postIDs) == 0 {
		return result, nil
	}

	rows := make([]tagRow, 0)
	if err := repository.database.WithContext(ctx).
		Table("post_tags AS post_tag").
		Select("post_tag.post_id", "tag.name").
		Joins("JOIN tags AS tag ON tag.id = post_tag.tag_id").
		Where("post_tag.post_id IN ?", postIDs).
		Order("tag.name ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list post tags: %w", err)
	}
	for _, row := range rows {
		result[row.PostID] = append(result[row.PostID], row.Name)
	}
	return result, nil
}

func resolveTaxonomy(ctx context.Context, transaction *gorm.DB, input DraftInput) (uint64, []uint64, error) {
	var category categoryRecord
	err := transaction.WithContext(ctx).Where("slug = ?", input.Category.Slug).Take(&category).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil, fmt.Errorf("find category: %w", err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		category = categoryRecord{
			Slug: input.Category.Slug, Name: input.Category.Name, CreatedAt: time.Now().UTC(),
		}
		if err := transaction.WithContext(ctx).Create(&category).Error; err != nil {
			return 0, nil, fmt.Errorf("create category: %w", err)
		}
	}

	tagIDs := make([]uint64, 0, len(input.Tags))
	for _, name := range input.Tags {
		var tag tagRecord
		err := transaction.WithContext(ctx).Where("name = ?", name).Take(&tag).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil, fmt.Errorf("find tag: %w", err)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tag = tagRecord{Name: name, CreatedAt: time.Now().UTC()}
			if err := transaction.WithContext(ctx).Create(&tag).Error; err != nil {
				return 0, nil, fmt.Errorf("create tag: %w", err)
			}
		}
		tagIDs = append(tagIDs, tag.ID)
	}
	return category.ID, tagIDs, nil
}

func replacePostTags(ctx context.Context, transaction *gorm.DB, postID uint64, tagIDs []uint64) error {
	if err := transaction.WithContext(ctx).Where("post_id = ?", postID).Delete(&postTagRecord{}).Error; err != nil {
		return fmt.Errorf("clear post tags: %w", err)
	}
	links := make([]postTagRecord, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		links = append(links, postTagRecord{PostID: postID, TagID: tagID})
	}
	if len(links) > 0 {
		if err := transaction.WithContext(ctx).Create(&links).Error; err != nil {
			return fmt.Errorf("save post tags: %w", err)
		}
	}
	return nil
}
