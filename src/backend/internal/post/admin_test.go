package post

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeDraft(t *testing.T) {
	normalized, err := normalizeDraft(DraftInput{
		Slug: "  hello-world  ", Title: "  标题  ", ContentMarkdown: "\n正文\n",
	})
	if err != nil {
		t.Fatalf("normalizeDraft() error = %v", err)
	}
	if normalized.Slug != "hello-world" || normalized.Title != "标题" || normalized.ContentMarkdown != "\n正文\n" {
		t.Fatalf("normalized = %#v", normalized)
	}
}

func TestNormalizeDraftRejectsInvalidIdentityFields(t *testing.T) {
	tests := []DraftInput{
		{Slug: "Uppercase", Title: "标题"},
		{Slug: "two--hyphens", Title: "标题"},
		{Slug: "", Title: "标题"},
		{Slug: "valid-slug", Title: "   "},
		{Slug: "valid-slug", Title: strings.Repeat("字", 201)},
	}
	for _, input := range tests {
		if _, err := normalizeDraft(input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("normalizeDraft(%#v) error = %v, want ErrInvalidInput", input, err)
		}
	}
}
