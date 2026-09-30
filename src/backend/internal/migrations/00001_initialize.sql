-- +goose Up
-- 首个迁移只初始化 Goose 的版本记录，不提前创建尚未确认的业务表。
SELECT 1;

-- +goose Down
SELECT 1;
