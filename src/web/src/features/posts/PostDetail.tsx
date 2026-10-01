import MarkdownContent from '../../components/MarkdownContent'
import type { PostDetailResponse } from './postApi'

interface PostDetailProps {
  post: PostDetailResponse
}

const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
  timeZone: 'Asia/Shanghai',
})

function PostDetail({ post }: PostDetailProps) {
  return (
    <article className="article">
      <header className="article__header">
        <p className="eyebrow">ARTICLE</p>
        <h1>{post.title}</h1>
        <time className="article__date" dateTime={post.publishedAt}>
          发布于 {dateFormatter.format(new Date(post.publishedAt))}
        </time>
        <div className="article-taxonomy">
          <span className="post-category">{post.category.name}</span>
          {post.tags.length > 0 && (
            <ul className="tag-list" aria-label="文章标签">
              {post.tags.map((tag) => (
                <li key={tag}>{tag}</li>
              ))}
            </ul>
          )}
        </div>
      </header>
      <div className="article-content">
        <MarkdownContent>{post.contentMarkdown}</MarkdownContent>
      </div>
    </article>
  )
}

export default PostDetail
