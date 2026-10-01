import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import AdminPostList from './AdminPostList'

describe('AdminPostList', () => {
  it('展示草稿状态并链接到编辑器', () => {
    const html = renderToStaticMarkup(
      <MemoryRouter>
        <AdminPostList
          items={[
            {
              id: 7,
              slug: 'first-draft',
              title: '第一篇草稿',
              status: 'draft',
              publishedAt: null,
              updatedAt: '2026-10-01T02:03:04Z',
              category: { slug: 'engineering', name: '工程' },
              tags: ['Go'],
            },
          ]}
        />
      </MemoryRouter>,
    )

    expect(html).toContain('第一篇草稿')
    expect(html).toContain('草稿')
    expect(html).toContain('href="/admin/posts/7"')
    expect(html).toContain('工程')
  })
})
