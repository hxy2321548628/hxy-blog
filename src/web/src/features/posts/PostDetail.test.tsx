import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import PostDetail from './PostDetail'

describe('PostDetail', () => {
  it('安全渲染文章 Markdown 与发布时间', () => {
    const html = renderToStaticMarkup(
      <MemoryRouter>
        <PostDetail
          post={{
          slug: 'hello-world',
          title: '第一篇文章',
          contentMarkdown:
            '## 小标题\n\n### 子章节\n\n- 列表项\n\n```go\n/* 第一行\n第二行 */\nfmt.Println("hello")\n```\n\n```python\nprint("hello")\n```\n\n```mermaid\ngraph LR\n  A --> B\n```\n\n公式 $E = mc^2$\n\n![系统架构图](https://media.hxy2333.site/media/test.png)\n\n![危险图片](javascript:alert(1))\n\n[外部链接](https://example.com)\n\n<script>alert("xss")</script>',
          publishedAt: '2026-10-01T02:03:04Z',
          category: { slug: 'engineering', name: '工程' },
          tags: ['Go'],
          }}
        />
      </MemoryRouter>,
    )

    expect(html).toContain('<h1>第一篇文章</h1>')
    expect(html).toContain('<h2 id="user-content-小标题">小标题</h2>')
    expect(html).toContain('href="#user-content-小标题"')
    expect(html).toContain('href="#user-content-子章节"')
    expect(html).toContain('<li>列表项</li>')
    expect(html).toContain('language-go')
    expect(html).toContain('hljs-built_in')
    expect(html).toContain('language-python')
    expect(html).toContain('data-line-number="1"')
    expect(html).toContain('data-line-number="2"')
    expect(html).toContain('data-line-number="3"')
    expect(html).not.toContain('data-line-number="4"')
    expect(html.match(/data-line-number=/g)).toHaveLength(4)
    expect(html).toContain('正在渲染图表')
    expect(html).toContain('class="katex"')
    expect(html).toContain('dateTime="2026-10-01T02:03:04Z"')
    expect(html).toContain('rel="noreferrer noopener"')
    expect(html).toContain('工程')
    expect(html).toContain('Go')
    expect(html).toContain(
      '<img src="https://media.hxy2333.site/media/test.png" alt="系统架构图"',
    )
    expect(html).not.toContain('javascript:')
    expect(html).not.toContain('<script')
    expect(html).not.toContain('alert(&quot;xss&quot;)')
  })
})
