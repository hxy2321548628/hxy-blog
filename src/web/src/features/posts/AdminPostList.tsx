import { Link } from 'react-router-dom'
import type { AdminPostSummary } from './postApi'

interface AdminPostListProps {
  items: AdminPostSummary[]
}

const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
  timeZone: 'Asia/Shanghai',
})

function AdminPostList({ items }: AdminPostListProps) {
  if (items.length === 0) {
    return <p className="empty-state">还没有文章，从一篇新草稿开始吧。</p>
  }

  return (
    <ol className="admin-post-list">
      {items.map((item) => (
        <li key={item.id}>
          <Link to={`/admin/posts/${item.id}`}>
            <div>
              <span className={`status-badge status-badge--${item.status}`}>
                {item.status === 'draft' ? '草稿' : '已发布'}
              </span>
              <h2>{item.title}</h2>
              <p>/{item.slug}</p>
              <p>{item.category.name}</p>
            </div>
            <time dateTime={item.updatedAt}>
              {dateFormatter.format(new Date(item.updatedAt))}
            </time>
          </Link>
        </li>
      ))}
    </ol>
  )
}

export default AdminPostList
