import { Link, useSearchParams } from 'react-router-dom'
import SiteFooter from '../../components/SiteFooter'
import SiteHeader from '../../components/SiteHeader'
import PostList from './PostList'
import { useListCategoriesQuery, useListPostsQuery } from './postApi'

const pageSize = 10

function pageFromSearchParams(value: string | null) {
  const parsed = Number(value ?? '1')
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : 1
}

function PostListPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const page = pageFromSearchParams(searchParams.get('page'))
  const category = searchParams.get('category') ?? undefined
  const { data, isLoading, isError } = useListPostsQuery({
    page,
    pageSize,
    category,
  })
  const { data: categoryData } = useListCategoriesQuery()
  const activeCategory = categoryData?.items.find(
    (item) => item.slug === category,
  )

  const changePage = (nextPage: number) => {
    // 切换分页时保留分类；第一页则不保留冗余 page 参数。
    const next = new URLSearchParams()
    if (category) next.set('category', category)
    if (nextPage !== 1) next.set('page', String(nextPage))
    setSearchParams(next)
  }

  return (
    <>
      <a className="skip-link" href="#main-content">
        跳到主要内容
      </a>
      <SiteHeader />

      <main className="page-shell" id="main-content">
        <div className="journal-index">
          <span>01 / JOURNAL</span>
          <span>独立写作 · 持续更新</span>
        </div>
        <header className="journal-hero">
          <div className="journal-hero__main">
            <p className="eyebrow">
              <span className="red-square" aria-hidden="true" />
              HXY / NOTES ON BUILDING
            </p>
            <h1>
              记录问题。<br />
              <span>整理答案。</span>
            </h1>
          </div>
          <div className="journal-hero__aside">
            <span className="journal-hero__mark" aria-hidden="true">✳</span>
            <p>你好，我是 hxy。这里记录技术、工程实践与长期思考。</p>
            <span>文字，是思考的另一种结构。</span>
          </div>
        </header>

        <div className="list-heading" id="latest-posts">
          <div>
            <span className="list-heading__number">01</span>
            <h2>{activeCategory ? activeCategory.name : '全部文章'}</h2>
          </div>
          <span className="list-heading__note">
            SELECTED WRITING / {new Date().getFullYear()}
          </span>
        </div>
        <div className="page-grid">
          <aside className="profile" aria-label="文章分类">
            <p className="profile__label">BROWSE BY TOPIC</p>
            <nav className="category-nav" aria-label="文章分类">
              <Link
                className={!category ? 'category-nav__active' : undefined}
                aria-current={!category ? 'page' : undefined}
                to="/"
              >
                <span>全部</span>
                <span>
                  {categoryData?.items.reduce(
                    (total, item) => total + item.postCount,
                    0,
                  ) ?? '—'}
                </span>
              </Link>
              {categoryData?.items.map((item) => (
                <Link
                  key={item.slug}
                  className={
                    category === item.slug ? 'category-nav__active' : undefined
                  }
                  aria-current={category === item.slug ? 'page' : undefined}
                  to={`/?category=${encodeURIComponent(item.slug)}`}
                >
                  <span>{item.name}</span>
                  <span>{item.postCount}</span>
                </Link>
              ))}
            </nav>
            <p className="profile__note">
              以分类找到主题，<br />以标题决定下一次阅读。
            </p>
          </aside>

          <section className="post-column" aria-label="文章列表">
            <div className="post-column__heading">
              <span>文章 / ARTICLE</span>
              {data && <span className="post-count">共 {data.total} 篇</span>}
            </div>

            {isLoading && (
              <p className="request-state" role="status">
                正在读取文章…
              </p>
            )}
            {isError && (
              <p className="request-state request-state--error" role="alert">
                文章暂时无法加载，请稍后重试。
              </p>
            )}
            {data && (
              <PostList
                items={data.items}
                startIndex={(page - 1) * pageSize}
              />
            )}

            {data && data.total > pageSize && (
              <nav className="pagination" aria-label="文章分页">
                <button
                  type="button"
                  disabled={page === 1}
                  onClick={() => changePage(page - 1)}
                >
                  上一页
                </button>
                <span>第 {page} 页</span>
                <button
                  type="button"
                  disabled={page * pageSize >= data.total}
                  onClick={() => changePage(page + 1)}
                >
                  下一页
                </button>
              </nav>
            )}
          </section>
        </div>
      </main>
      <SiteFooter />
    </>
  )
}

export default PostListPage
