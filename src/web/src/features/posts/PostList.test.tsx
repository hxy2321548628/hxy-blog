import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import PostList from './PostList'

describe('PostList', () => {
  it('展示已发布文章的标题和时间', () => {
    const html = renderToStaticMarkup(
      <PostList
        items={[
          {
            slug: 'hello-world',
            title: '第一篇文章',
            publishedAt: '2026-10-01T02:03:04Z',
          },
        ]}
      />,
    )

    expect(html).toContain('第一篇文章')
    expect(html).toContain('dateTime="2026-10-01T02:03:04Z"')
    expect(html).not.toContain('hello-world')
  })

  it('在没有已发布文章时显示空状态', () => {
    const html = renderToStaticMarkup(<PostList items={[]} />)

    expect(html).toContain('还没有已发布的文章')
  })
})
