import ReactMarkdown from 'react-markdown'
import rehypeSanitize from 'rehype-sanitize'
import remarkGfm from 'remark-gfm'
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
      </header>
      <div className="article-content">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          rehypePlugins={[rehypeSanitize]}
          skipHtml
          components={{
            a: ({ href, children }) => {
              const external = href?.startsWith('http://') || href?.startsWith('https://')
              return (
                <a href={href} rel={external ? 'noreferrer noopener' : undefined}>
                  {children}
                </a>
              )
            },
          }}
        >
          {post.contentMarkdown}
        </ReactMarkdown>
      </div>
    </article>
  )
}

export default PostDetail
