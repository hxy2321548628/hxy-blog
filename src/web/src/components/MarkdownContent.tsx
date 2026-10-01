import ReactMarkdown from 'react-markdown'
import rehypeSanitize from 'rehype-sanitize'
import remarkGfm from 'remark-gfm'

interface MarkdownContentProps {
  children: string
}

function MarkdownContent({ children }: MarkdownContentProps) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[rehypeSanitize]}
      skipHtml
      components={{
        a: ({ href, children: linkChildren }) => {
          const external =
            href?.startsWith('http://') || href?.startsWith('https://')
          return (
            <a
              href={href}
              rel={external ? 'noreferrer noopener' : undefined}
            >
              {linkChildren}
            </a>
          )
        },
      }}
    >
      {children}
    </ReactMarkdown>
  )
}

export default MarkdownContent
