import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import PostDetail from './PostDetail'

describe('PostDetail', () => {
  it('安全渲染文章 Markdown 与发布时间', () => {
    const html = renderToStaticMarkup(
      <PostDetail
        post={{
          slug: 'hello-world',
          title: '第一篇文章',
          contentMarkdown:
            '## 小标题\n\n- 列表项\n\n![系统架构图](https://media.hxy2333.site/media/test.png)\n\n![危险图片](javascript:alert(1))\n\n[外部链接](https://example.com)\n\n<script>alert("xss")</script>',
          publishedAt: '2026-10-01T02:03:04Z',
        }}
      />,
    )

    expect(html).toContain('<h1>第一篇文章</h1>')
    expect(html).toContain('<h2>小标题</h2>')
    expect(html).toContain('<li>列表项</li>')
    expect(html).toContain('dateTime="2026-10-01T02:03:04Z"')
    expect(html).toContain('rel="noreferrer noopener"')
    expect(html).toContain(
      '<img src="https://media.hxy2333.site/media/test.png" alt="系统架构图"',
    )
    expect(html).not.toContain('javascript:')
    expect(html).not.toContain('<script')
    expect(html).not.toContain('alert(&quot;xss&quot;)')
  })
})
