package post

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeDraft(t *testing.T) {
	normalized, err := normalizeDraft(DraftInput{
		Slug: "  hello-world  ", Title: "  标题  ", ContentMarkdown: "\n正文\n",
		Category: Category{Slug: "  engineering  ", Name: "  工程  "},
		Tags:     []string{" Go ", "go", "React"},
	})
	if err != nil {
		t.Fatalf("normalizeDraft() error = %v", err)
	}
	if normalized.Slug != "hello-world" || normalized.Title != "标题" || normalized.ContentMarkdown != "\n正文\n" {
		t.Fatalf("normalized = %#v", normalized)
	}
	if normalized.Category != (Category{Slug: "engineering", Name: "工程"}) || len(normalized.Tags) != 2 {
		t.Fatalf("normalized taxonomy = %#v, %#v", normalized.Category, normalized.Tags)
	}
}

func TestNormalizeDraftRejectsInvalidIdentityFields(t *testing.T) {
	validCategory := Category{Slug: "engineering", Name: "工程"}
	tests := []DraftInput{
		{Slug: "Uppercase", Title: "标题", Category: validCategory},
		{Slug: "two--hyphens", Title: "标题", Category: validCategory},
		{Slug: "", Title: "标题", Category: validCategory},
		{Slug: "valid-slug", Title: "   ", Category: validCategory},
		{Slug: "valid-slug", Title: strings.Repeat("字", 201), Category: validCategory},
		{Slug: "valid-slug", Title: "标题", Category: Category{Name: "工程"}},
		{Slug: "valid-slug", Title: "标题", Category: Category{Slug: "engineering"}},
		{Slug: "valid-slug", Title: "标题", Category: validCategory, Tags: []string{""}},
	}
	for _, input := range tests {
		if _, err := normalizeDraft(input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("normalizeDraft(%#v) error = %v, want ErrInvalidInput", input, err)
		}
	}
}
