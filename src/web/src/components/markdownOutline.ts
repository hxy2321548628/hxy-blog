import GithubSlugger from 'github-slugger'
import { toString } from 'mdast-util-to-string'
import type { Heading, Root } from 'mdast'
import remarkParse from 'remark-parse'
import { unified } from 'unified'
import { visit } from 'unist-util-visit'

export interface OutlineItem {
  id: string
  text: string
  depth: 2 | 3
}

const sanitizedIDPrefix = 'user-content-'

function visitOutlineHeadings(
  tree: Root,
  callback: (heading: Heading, id: string, text: string) => void,
) {
  const slugger = new GithubSlugger()
  visit(tree, 'heading', (heading) => {
    if (heading.depth !== 2 && heading.depth !== 3) return
    const text = toString(heading).trim()
    if (!text) return
    callback(heading, slugger.slug(text), text)
  })
}

// 渲染器与大纲共用同一套 slug 规则，重名标题也能精确跳转。
export function remarkHeadingIDs() {
  return (tree: Root) => {
    visitOutlineHeadings(tree, (heading, id) => {
      heading.data = {
        ...heading.data,
        hProperties: { ...heading.data?.hProperties, id },
      }
    })
  }
}

export function extractOutline(markdown: string): OutlineItem[] {
  const tree = unified().use(remarkParse).parse(markdown)
  const items: OutlineItem[] = []
  visitOutlineHeadings(tree, (heading, id, text) => {
    // rehype-sanitize 会加前缀防止 DOM clobbering，大纲链接必须与最终 DOM 保持一致。
    items.push({
      id: `${sanitizedIDPrefix}${id}`,
      text,
      depth: heading.depth as 2 | 3,
    })
  })
  return items
}
