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
          <article>
            <time className="post-meta" dateTime={item.publishedAt}>
              {dateFormatter.format(new Date(item.publishedAt))}
            </time>
            <h2>{item.title}</h2>
          </article>
        </li>
      ))}
    </ol>
  )
}

export default PostList
