import { cloneElement, isValidElement, type ReactNode } from 'react'
import ReactMarkdown from 'react-markdown'
import rehypeHighlight from 'rehype-highlight'
import rehypeKatex from 'rehype-katex'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import 'highlight.js/styles/github.css'
import 'katex/dist/katex.min.css'
import MermaidDiagram from './MermaidDiagram'
import { remarkHeadingIDs } from './markdownOutline'

interface MarkdownContentProps {
  children: string
}

const sanitizeSchema = {
  ...defaultSchema,
  clobberPrefix: 'user-content-',
  attributes: {
    ...defaultSchema.attributes,
    code: [
      ...(defaultSchema.attributes?.code ?? []),
      ['className', /^language-./, 'math-inline', 'math-display'],
    ],
    h2: [...(defaultSchema.attributes?.h2 ?? []), 'id'],
    h3: [...(defaultSchema.attributes?.h3 ?? []), 'id'],
  },
}

function textContent(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(textContent).join('')
  if (isValidElement<{ children?: ReactNode }>(node)) {
    return textContent(node.props.children)
  }
  return ''
}

function highlightedLines(
  node: ReactNode,
  trimTerminalNewline = true,
): ReactNode[][] {
  const lines: ReactNode[][] = [[]]
  let key = 0

  const append = (current: ReactNode) => {
    if (current === null || current === undefined || typeof current === 'boolean') {
      return
    }
    if (typeof current === 'string' || typeof current === 'number') {
      const parts = String(current).split('\n')
      parts.forEach((part, index) => {
        if (part) lines[lines.length - 1].push(part)
        if (index < parts.length - 1) lines.push([])
      })
      return
    }
    if (Array.isArray(current)) {
      current.forEach(append)
      return
    }
    if (isValidElement<{ children?: ReactNode }>(current)) {
      const childLines = highlightedLines(current.props.children, false)
      childLines.forEach((children, index) => {
        if (children.length > 0) {
          lines[lines.length - 1].push(
            cloneElement(current, { key: key++ }, children),
          )
        }
        if (index < childLines.length - 1) lines.push([])
      })
    }
  }

  append(node)
  // Markdown 围栏正文固定以换行结尾；去掉它产生的额外空行，但保留作者写下的空白行。
  if (
    trimTerminalNewline &&
    lines.length > 1 &&
    lines[lines.length - 1].length === 0
  ) {
    lines.pop()
  }
  return lines
}

function MarkdownContent({ children }: MarkdownContentProps) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkMath, remarkHeadingIDs]}
      rehypePlugins={[
        [rehypeSanitize, sanitizeSchema],
        rehypeKatex,
        [rehypeHighlight, { detect: false, plainText: ['mermaid'] }],
      ]}
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
        pre: ({ children: preChildren }) => {
          if (
            isValidElement<{
              className?: string
              children?: ReactNode
            }>(preChildren) &&
            preChildren.props.className
              ?.split(/\s+/)
              .includes('language-mermaid')
          ) {
            return (
              <MermaidDiagram
                source={textContent(preChildren.props.children).replace(
                  /\n$/,
                  '',
                )}
              />
            )
          }
          if (
            isValidElement<{
              children?: ReactNode
            }>(preChildren)
          ) {
            const lines = highlightedLines(preChildren.props.children)
            return (
              <pre className="code-block">
                {cloneElement(
                  preChildren,
                  {},
                  <span className="code-lines">
                    {lines.map((line, index) => (
                      <span
                        className="code-line"
                        data-line-number={index + 1}
                        key={index}
                      >
                        {line}
                      </span>
                    ))}
                  </span>,
                )}
              </pre>
            )
          }
          return <pre className="code-block">{preChildren}</pre>
        },
        code: ({ className, children: codeChildren }) => {
          const language = className
            ?.split(/\s+/)
            .find((name) => name.startsWith('language-'))
            ?.slice('language-'.length)
          return (
            <code className={className} data-language={language}>
              {codeChildren}
            </code>
          )
        },
      }}
    >
      {children}
    </ReactMarkdown>
  )
}

export default MarkdownContent
