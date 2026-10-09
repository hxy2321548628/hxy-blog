import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import PostList from './PostList'

describe('PostList', () => {
  it('展示已发布文章的标题和时间', () => {
    const html = renderToStaticMarkup(
      <MemoryRouter>
        <PostList
          startIndex={10}
          items={[
            {
              slug: 'hello-world',
              title: '第一篇文章',
              publishedAt: '2026-10-01T02:03:04Z',
              category: { slug: 'engineering', name: '工程' },
              tags: ['Go', 'React'],
            },
          ]}
        />
      </MemoryRouter>,
    )

    expect(html).toContain('第一篇文章')
    expect(html).toContain('dateTime="2026-10-01T02:03:04Z"')
    expect(html).toContain('href="/posts/hello-world"')
    expect(html).toContain('post-list__number">11</span>')
    expect(html).toContain('工程')
    expect(html).toContain('Go')
  })

  it('在没有已发布文章时显示空状态', () => {
    const html = renderToStaticMarkup(
      <MemoryRouter>
        <PostList items={[]} />
      </MemoryRouter>,
    )

    expect(html).toContain('还没有已发布的文章')
  })
})
