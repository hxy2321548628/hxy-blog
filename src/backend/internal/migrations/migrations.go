package migrations

import "embed"

// Files 把 SQL 迁移编译进发布镜像，确保迁移内容与应用 Commit 完全一致。
//
//go:embed *.sql
var Files embed.FS
