# hxy.blog 瑞士风格前台原型

打开 [`index.html`](./index.html) 查看文章列表；点击“阅读示例”或首篇文章查看详情。分类按钮可筛选原型中的示例文章。所有标题、日期和正文均为示例内容。

## 设计取舍

- 用明确的网格、编号、规则线和红色强调建立瑞士风格的视觉秩序。红色只标记当前页、重点和可交互反馈。
- 列表页突出标题与分类，让读者快速扫描；详情页将正文限制在较短行长，桌面大纲位于侧栏，手机大纲位于正文之前。
- 使用系统字体与原生 HTML/CSS，不增加字体下载或前端依赖；代码块在窄屏内独立横向滚动。
- 原型不修改现有生产页面。正式实施时需要保留文章分页、Markdown 渲染、背景音乐控制以及已有的加载、失败和空状态。

## 原型图

| 页面 | 桌面 | 手机 |
| --- | --- | --- |
| 文章列表 | [`list-desktop.png`](./list-desktop.png) | [`list-mobile.png`](./list-mobile.png) |
| 文章详情 | [`article-desktop.png`](./article-desktop.png) | [`article-mobile.png`](./article-mobile.png) |
