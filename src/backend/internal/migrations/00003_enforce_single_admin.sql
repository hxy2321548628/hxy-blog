-- +goose Up
-- 固定值列配合唯一索引把“单管理员”变成数据库不变量，避免两个初始化进程并发创建账户。
ALTER TABLE admins
    ADD COLUMN singleton TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER id,
    ADD CONSTRAINT chk_admins_singleton CHECK (singleton = 1),
    ADD UNIQUE KEY uk_admins_singleton (singleton);

-- +goose Down
ALTER TABLE admins
    DROP INDEX uk_admins_singleton,
    DROP CHECK chk_admins_singleton,
    DROP COLUMN singleton;
