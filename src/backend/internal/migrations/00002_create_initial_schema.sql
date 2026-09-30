-- +goose Up
-- 文章表只保存 Markdown 事实源；渲染 HTML、分类和软删除都不在当前故事范围内。
CREATE TABLE posts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    slug VARCHAR(200) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    title VARCHAR(200) NOT NULL,
    content_markdown MEDIUMTEXT NOT NULL,
    status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    published_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_posts_slug (slug),
    KEY idx_posts_public_list (status, published_at, id),
    CONSTRAINT chk_posts_slug_not_empty CHECK (CHAR_LENGTH(slug) > 0),
    CONSTRAINT chk_posts_title_not_empty CHECK (CHAR_LENGTH(TRIM(title)) > 0),
    CONSTRAINT chk_posts_status CHECK (status IN ('draft', 'published')),
    -- 草稿不得意外出现发布时间，已发布文章则必须能稳定排序。
    CONSTRAINT chk_posts_publication CHECK (
        (status = 'draft' AND published_at IS NULL)
        OR (status = 'published' AND published_at IS NOT NULL)
    )
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

-- 密码字段只接收 Argon2id PHC 字符串，初始密码不通过迁移写入仓库。
CREATE TABLE admins (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    password_hash VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_admins_username (username),
    CONSTRAINT chk_admins_username_not_empty CHECK (CHAR_LENGTH(TRIM(username)) > 0),
    CONSTRAINT chk_admins_password_hash_not_empty CHECK (CHAR_LENGTH(password_hash) > 0)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

-- Token 只保存 SHA-256 二进制哈希；family_id 使重放时可撤销整条会话链。
CREATE TABLE refresh_tokens (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    admin_id BIGINT UNSIGNED NOT NULL,
    token_hash BINARY(32) NOT NULL,
    family_id BINARY(16) NOT NULL,
    parent_id BIGINT UNSIGNED NULL,
    expires_at DATETIME(6) NOT NULL,
    session_expires_at DATETIME(6) NOT NULL,
    used_at DATETIME(6) NULL,
    revoked_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_refresh_tokens_token_hash (token_hash),
    -- 父节点唯一可阻止并发刷新产生两条同时有效的分支。
    UNIQUE KEY uk_refresh_tokens_parent_id (parent_id),
    KEY idx_refresh_tokens_family (family_id, created_at),
    KEY idx_refresh_tokens_admin_expiry (admin_id, revoked_at, expires_at),
    CONSTRAINT fk_refresh_tokens_admin
        FOREIGN KEY (admin_id) REFERENCES admins (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_refresh_tokens_parent
        FOREIGN KEY (parent_id) REFERENCES refresh_tokens (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT chk_refresh_tokens_expiry CHECK (
        expires_at > created_at
        AND session_expires_at >= expires_at
    ),
    CONSTRAINT chk_refresh_tokens_used_at CHECK (used_at IS NULL OR used_at >= created_at),
    CONSTRAINT chk_refresh_tokens_revoked_at CHECK (revoked_at IS NULL OR revoked_at >= created_at)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

-- +goose Down
-- 先删除带外键的会话表，再删被引用的管理员表，保证 down 可重复验证。
DROP TABLE refresh_tokens;
DROP TABLE admins;
DROP TABLE posts;
