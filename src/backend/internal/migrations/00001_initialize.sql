-- +goose Up
-- Goose 通过 Up/Down 标记拆分两个方向；这些标记必须保持为独立注释行。
-- 首个迁移只初始化 Goose 的版本记录，不提前创建尚未确认的业务表。
SELECT 1;

-- +goose Down
-- 对应的回退同样无业务副作用，用来验证 CI 和生产迁移链路能够执行 down。
SELECT 1;
