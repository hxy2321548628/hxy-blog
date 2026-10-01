-- +goose Up
-- 分类使用稳定 slug 支持前台筛选；分类名称可以使用中文展示。
CREATE TABLE categories (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    slug VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(80) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_categories_slug (slug),
    CONSTRAINT chk_categories_slug_not_empty CHECK (CHAR_LENGTH(slug) > 0),
    CONSTRAINT chk_categories_name_not_empty CHECK (CHAR_LENGTH(TRIM(name)) > 0)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

-- 历史文章不应因新增必填外键而无法迁移，先统一归入“未分类”。
INSERT INTO categories (slug, name, created_at)
VALUES ('uncategorized', '未分类', UTC_TIMESTAMP(6));

ALTER TABLE posts ADD COLUMN category_id BIGINT UNSIGNED NULL AFTER content_markdown;
UPDATE posts
SET category_id = (SELECT id FROM categories WHERE slug = 'uncategorized');
ALTER TABLE posts
    MODIFY COLUMN category_id BIGINT UNSIGNED NOT NULL,
    ADD KEY idx_posts_category (category_id, status, published_at, id),
    ADD CONSTRAINT fk_posts_category
        FOREIGN KEY (category_id) REFERENCES categories (id)
        ON UPDATE RESTRICT ON DELETE RESTRICT;

CREATE TABLE tags (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(40) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_tags_name (name),
    CONSTRAINT chk_tags_name_not_empty CHECK (CHAR_LENGTH(TRIM(name)) > 0)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE post_tags (
    post_id BIGINT UNSIGNED NOT NULL,
    tag_id BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (post_id, tag_id),
    KEY idx_post_tags_tag (tag_id, post_id),
    CONSTRAINT fk_post_tags_post
        FOREIGN KEY (post_id) REFERENCES posts (id)
        ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT fk_post_tags_tag
        FOREIGN KEY (tag_id) REFERENCES tags (id)
        ON UPDATE RESTRICT ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE post_tags;
DROP TABLE tags;
ALTER TABLE posts
    DROP FOREIGN KEY fk_posts_category,
    DROP INDEX idx_posts_category,
    DROP COLUMN category_id;
DROP TABLE categories;
