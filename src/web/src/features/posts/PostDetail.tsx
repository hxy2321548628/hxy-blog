import { Link } from 'react-router-dom'
import MarkdownContent from '../../components/MarkdownContent'
import { extractOutline } from '../../components/markdownOutline'
import type { PostDetailResponse } from './postApi'
import ArticleOutline from './ArticleOutline'

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
  const outline = extractOutline(post.contentMarkdown)
  return (
    <article className="article-layout">
      <header className="article__header">
        <p className="eyebrow">
          <span className="red-square" aria-hidden="true" />
          {post.category.name} / ARTICLE
        </p>
        <h1>{post.title}</h1>
        <div className="article__meta">
          <time className="article__date" dateTime={post.publishedAt}>
            {dateFormatter.format(new Date(post.publishedAt))}
          </time>
          <span>{post.category.name}</span>
          {post.tags.length > 0 && (
            <ul className="tag-list" aria-label="文章标签">
              {post.tags.map((tag) => (
                <li key={tag}>{tag}</li>
              ))}
            </ul>
          )}
        </div>
      </header>
      <ArticleOutline items={outline} />
      <div className="article-content">
        <MarkdownContent>{post.contentMarkdown}</MarkdownContent>
        <div className="article-end">
          <span>— END OF ARTICLE —</span>
          <Link to="/">返回全部文章 ↗</Link>
        </div>
      </div>
    </article>
  )
}

export default PostDetail
