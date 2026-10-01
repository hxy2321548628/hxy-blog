import type { OutlineItem } from '../../components/markdownOutline'

interface ArticleOutlineProps {
  items: OutlineItem[]
}

function ArticleOutline({ items }: ArticleOutlineProps) {
  if (items.length === 0) return null

  return (
    <aside className="article-outline">
      <p className="article-outline__title">文章大纲</p>
      <nav aria-label="文章大纲">
        <ol>
          {items.map((item) => (
            <li
              key={item.id}
              className={
                item.depth === 3 ? 'article-outline__item--nested' : undefined
              }
            >
              <a href={`#${item.id}`}>{item.text}</a>
            </li>
          ))}
        </ol>
      </nav>
    </aside>
  )
}

export default ArticleOutline
