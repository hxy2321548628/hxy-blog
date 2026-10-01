# Sprint 2 开发计划

- 日期：2026-10-02
- 状态：范围已确认，开发中
- Sprint 目标：完善文章管理、内容组织和技术文章阅读体验。

## 已确认的产品决策

- 每篇文章只属于一个分类，可有多个标签。
- 分类和标签由管理员在文章编辑器中填写，相同值自动复用。
- 删除为物理删除，管理端必须在提交前二次确认并明示不可撤销。
- 已发布文章可修改标题、正文、分类和标签，但保持 slug 与首次发布时间不变。
- 前台使用 `/?category=<分类 slug>` 筛选已发布文章。
- Markdown 围栏代码块根据 `go`、`python` 等语言标识进行语法高亮，并显示从 1 开始的行号。
- Mermaid 使用 `mermaid` 代码块，数学公式使用 `$...$` 和 `$$...$$`。
- 大纲收集 Markdown 二、三级标题；桌面端显示在文章侧边，窄屏仍保留可点击导航。

## 非目标

- 不新增“关于”页。
- 不新增多用户、RBAC、评论、搜索、回收站或标签筛选。
- 不更改部署镜像选型，不引入 SSR、微服务或缓存。
- 不把 Markdown 预渲染 HTML 写入数据库。

## 故事与验收标准

### 故事 1：文章编辑与删除

- Given 管理员打开草稿，When 修改并保存，Then 标题、slug 和正文按输入更新。
- Given 管理员打开已发布文章，When 修改并保存，Then 内容更新，slug 和首次发布时间不变。
- Given 管理员点击删除，When 未确认，Then 不发送删除请求；When 确认，Then 文章被物理删除并返回列表。
- Given 请求未携带有效 Access Token，When 调用写接口，Then API 拒绝操作。

### 故事 2：分类与标签

- Given 管理员保存文章，When 填写分类和标签，Then 文章与单一分类及去重标签在同一事务内更新。
- Given 历史文章已存在，When 执行迁移，Then 文章自动归入“未分类”，迁移可 down。
- Given 访客查看文章，Then 列表和详情显示分类/标签；When 选择分类，Then 只显示该分类已发布文章。
- Given 分类中只有草稿，Then 公开分类列表不泄露该分类。

### 故事 3：技术文章阅读

- Given Markdown 包含 `go`、`python` 等语言围栏，Then 代码按语言语法高亮、逐行显示连续行号并保留横向滚动。
- Given Markdown 包含合法 Mermaid，Then 浏览器展示 SVG 图；Given 语法错误，Then 显示可理解的失败状态而不影响其余文章。
- Given Markdown 包含行内或块级公式，Then 使用 KaTeX 渲染且窄屏不撑破页面。
- Given 文章包含二、三级标题，Then 大纲生成唯一锚点，点击可跳转到对应标题。
- Given Markdown 包含原始 HTML 或危险 URL，Then 仍被清洗或拒绝。

### 故事 4：导航收缩

- Given Header 当前只有文章页，Then 导航不展示“关于”或 `#about` 入口。

## 依赖与风险

- 语法高亮：`rehype-highlight` / `highlight.js`（BSD-3-Clause）。
- Mermaid：`mermaid`（MIT），仅在文章存在 Mermaid 代码块时动态加载，并启用严格安全模式。
- 数学公式：`remark-math` / `rehype-katex` / `katex`（MIT）。
- 标题锚点：`github-slugger`（ISC），大纲与渲染器必须共用相同规则。
- Mermaid 库体积较大；必须保持路由级和功能级延迟加载，并检查生产构建产物。
- Markdown 扩展不得绕过 `rehype-sanitize` 或启用原始 HTML。

## 验证命令

- `make check`
- `bash -c 'set -a; source .env; set +a; exec go -C src/backend test -tags=integration ./internal/migrations ./internal/post'`
- 在本地浏览器验证桌面/窄屏布局、Mermaid 真实渲染和大纲跳转。
