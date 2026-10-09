import { Link } from 'react-router-dom'
import type { PostSummary } from './postApi'

interface PostListProps {
  items: PostSummary[]
  startIndex?: number
}

const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  timeZone: 'Asia/Shanghai',
})

function PostList({ items, startIndex = 0 }: PostListProps) {
  if (items.length === 0) {
    return <p className="empty-state">还没有已发布的文章。</p>
  }

  return (
    <ol className="post-list">
      {items.map((item, index) => (
        <li key={item.slug} className="post-list__item">
          <Link
            className="post-list__link"
            to={`/posts/${encodeURIComponent(item.slug)}`}
          >
            <article>
              <span className="post-list__number">
                {String(startIndex + index + 1).padStart(2, '0')}
              </span>
              <div className="post-list__body">
                <div className="post-list__meta">
                  <span className="post-category">{item.category.name}</span>
                  <span aria-hidden="true">·</span>
                  <time className="post-meta" dateTime={item.publishedAt}>
                    {dateFormatter
                      .format(new Date(item.publishedAt))
                      .replaceAll('/', '.')}
                  </time>
                </div>
                <h3>{item.title}</h3>
                {item.tags.length > 0 && (
                  <ul className="tag-list" aria-label="文章标签">
                    {item.tags.map((tag) => (
                      <li key={tag}># {tag}</li>
                    ))}
                  </ul>
                )}
              </div>
              <span className="post-list__arrow" aria-hidden="true">↗</span>
            </article>
          </Link>
        </li>
      ))}
    </ol>
  )
}

export default PostList
