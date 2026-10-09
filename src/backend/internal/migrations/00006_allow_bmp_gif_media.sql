-- +goose Up
-- 仅扩展媒体 MIME 白名单；原有图片记录及对象键不需要改写。
ALTER TABLE media_assets
    DROP CHECK chk_media_assets_mime_type,
    ADD CONSTRAINT chk_media_assets_mime_type CHECK (
        mime_type IN ('image/jpeg', 'image/png', 'image/webp', 'image/bmp', 'image/gif')
    );

-- +goose Down
-- 若已有 BMP/GIF 元数据，恢复旧约束会失败并保留数据，需先迁移这些引用后再回滚。
ALTER TABLE media_assets
    DROP CHECK chk_media_assets_mime_type,
    ADD CONSTRAINT chk_media_assets_mime_type CHECK (
        mime_type IN ('image/jpeg', 'image/png', 'image/webp')
    );
