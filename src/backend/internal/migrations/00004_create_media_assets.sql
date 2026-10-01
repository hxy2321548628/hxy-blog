-- +goose Up
-- 数据库只保存供应商无关的对象键和校验元数据；原始图片正文仍位于 COS。
CREATE TABLE media_assets (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    object_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    mime_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    size_bytes BIGINT UNSIGNED NOT NULL,
    width INT UNSIGNED NOT NULL,
    height INT UNSIGNED NOT NULL,
    checksum_sha256 BINARY(32) NOT NULL,
    status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_media_assets_object_key (object_key),
    KEY idx_media_assets_created_at (created_at, id),
    CONSTRAINT chk_media_assets_mime_type CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/webp')),
    CONSTRAINT chk_media_assets_size CHECK (size_bytes > 0 AND size_bytes <= 10485760),
    CONSTRAINT chk_media_assets_dimensions CHECK (
        width > 0 AND width <= 8192
        AND height > 0 AND height <= 8192
        AND width * height <= 20000000
    ),
    CONSTRAINT chk_media_assets_status CHECK (status IN ('ready', 'failed'))
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE media_assets;
