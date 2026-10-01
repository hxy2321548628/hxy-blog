import { Link } from 'react-router-dom'
import type { PostSummary } from './postApi'

interface PostListProps {
  items: PostSummary[]
}

const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
  timeZone: 'Asia/Shanghai',
})

function PostList({ items }: PostListProps) {
  if (items.length === 0) {
    return <p className="empty-state">还没有已发布的文章。</p>
  }

  return (
    <ol className="post-list">
      {items.map((item) => (
        <li key={item.slug} className="post-list__item">
          <Link
            className="post-list__link"
            to={`/posts/${encodeURIComponent(item.slug)}`}
          >
            <article>
              <div className="post-list__meta">
                <time className="post-meta" dateTime={item.publishedAt}>
                  {dateFormatter.format(new Date(item.publishedAt))}
                </time>
                <span className="post-category">{item.category.name}</span>
              </div>
              <h2>{item.title}</h2>
              {item.tags.length > 0 && (
                <ul className="tag-list" aria-label="文章标签">
                  {item.tags.map((tag) => (
                    <li key={tag}>{tag}</li>
                  ))}
                </ul>
              )}
            </article>
          </Link>
        </li>
      ))}
    </ol>
  )
}

export default PostList
