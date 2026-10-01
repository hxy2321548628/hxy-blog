# Sprint 2 验收记录

- 验收日期：2026-10-02
- 功能基线：`9f4d150724b9faeccc0579fb19b43e5336efa16b`
- 验收结论：通过，文章管理、内容组织与技术文章阅读链路已闭合

## 验收范围

| 能力 | 验收结果 | 证据 |
| --- | --- | --- |
| 文章编辑与删除 | 通过 | 管理端支持草稿和已发布文章编辑、二次确认后的物理删除；已发布文章编辑保持 slug 与首次发布时间，写接口继续要求管理员鉴权 |
| 分类与标签 | 通过 | 每篇文章关联一个分类和多个去重标签；历史文章迁移至“未分类”；公开列表支持 `?category=<slug>` 筛选且不暴露只有草稿的分类 |
| 技术文章阅读 | 通过 | Go、Python 围栏按语言高亮并显示连续行号；Mermaid 严格模式生成 SVG；KaTeX 渲染行内与块级公式；二、三级标题生成唯一锚点与可点击大纲 |
| 响应式文章布局 | 通过 | 桌面端大纲固定在文章侧边；390px 视口下大纲移至正文上方，代码块、图表和公式不造成页面横向溢出 |
| Header 收缩 | 通过 | 全局导航仅保留“文章”，组件测试确认不存在“关于”入口 |

## 数据与安全边界

- 分类、标签和关联表由可回滚的 `00005_create_post_taxonomy.sql` 管理；真实 MySQL 已验证 `up -> down -> up`，当前迁移版本为 5。
- Markdown 仍禁止原始 HTML，并通过 `rehype-sanitize` 清洗危险 URL；标题 ID 采用防 DOM clobbering 前缀。
- Mermaid 仅在文章含图表时动态加载，使用 `securityLevel: 'strict'`；渲染失败会显示源码与可理解的错误状态，不阻塞正文。
- 新增依赖许可证均适合当前项目：Highlight.js（BSD-3-Clause）、Mermaid/KaTeX/remark-math/rehype-katex（MIT）、github-slugger（ISC）。锁文件通过覆盖将 Mermaid 的间接 `lodash-es` 固定到已修复版本，安装审计为 0 个已知漏洞。

## 验证记录

```text
make check
Go vet/单元测试：通过
Web：9 个测试文件、15 个测试通过
ESLint、TypeScript、Vite 生产构建：通过
开发/生产 Compose 配置与部署脚本检查：通过

go test -tags=integration ./internal/migrations ./internal/post
ok hxy-blog/backend/internal/migrations
ok hxy-blog/backend/internal/post

浏览器验收：
Go/Python 高亮与连续行号：通过
Mermaid SVG：1 个
KaTeX：2 处
大纲锚点点击：通过
390px 页面横向溢出：无
控制台 error/warn：0
```

## 已知边界

- Vite 生产构建仍提示部分技术文章依赖分块超过 500 kB；文章详情路由和 Mermaid 功能均保持延迟加载，当前不影响首页与验收功能。后续仅在真实性能数据表明有必要时再拆分语法高亮语言或公式资源。
- 本 Sprint 不包含“关于”页、搜索、标签筛选、评论、回收站和多用户权限。

## Definition of Done

- Sprint 2 四项 Backlog 均完成，并有自动化测试、真实 MySQL 集成测试或浏览器验收证据。
- Schema 变更可回滚，公开接口不泄露草稿分类，Markdown 扩展未绕过既有安全清洗。
- `make check` 通过，开发计划、Backlog、验收记录与实现保持同步。
