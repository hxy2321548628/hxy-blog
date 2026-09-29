# 代码规范

## 通用

- 代码和标识符使用英文，用户文案和项目文档使用中文。
- 函数只做一件事；优先早返回，避免深层嵌套。
- 不为一次性调用创建接口、工厂或通用工具层。
- 注释说明“为什么”，不复述代码本身。

## Go

- 必须通过 `gofmt`、`go vet ./...` 和 `go test ./...`。
- 包名简短、小写、单数；不使用 `util`/`common` 作为业务包。
- 在边界处包装错误并增加语境：`fmt.Errorf("load post: %w", err)`。
- `context.Context` 作为需要它的函数第一个参数，不存入结构体。
- HTTP 处理器只负责协议转换；业务规则不写在 handler 中。
- 数据库连接池必须显式配置最大连接数、空闲连接数和生命周期。

## React / TypeScript

- TypeScript 使用 strict 模式；不使用 `any`，边界未知值使用 `unknown` 并收窄。
- 优先函数组件和组合；不创建无业务意义的 wrapper 组件。
- 服务器状态与 UI 状态分离；不把可推导状态存入 state。
- 组件文件使用 `PascalCase.tsx`，普通模块使用 `camelCase.ts`。
- 必须通过 ESLint、TypeScript 类型检查和生产构建。

## API 与 MySQL

- API 前缀为 `/api`，JSON 字段使用 `camelCase`，时间使用 UTC RFC 3339。
- 列表接口显式分页并设置上限；不提供无界列表。
- MySQL 表和列使用 `snake_case`，字符集使用 `utf8mb4`。
- schema 变更用按时间或序号排序的 up/down 迁移。
