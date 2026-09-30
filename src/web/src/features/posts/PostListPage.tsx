import { Link, useSearchParams } from 'react-router-dom'
import PostList from './PostList'
import { useListPostsQuery } from './postApi'

const pageSize = 10

function pageFromSearchParams(value: string | null) {
  const parsed = Number(value ?? '1')
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : 1
}

function PostListPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const page = pageFromSearchParams(searchParams.get('page'))
  const { data, isLoading, isError } = useListPostsQuery({ page, pageSize })

  const changePage = (nextPage: number) => {
    // 第一页不保留冗余查询参数，复制首页 URL 时更简洁。
    setSearchParams(nextPage === 1 ? {} : { page: String(nextPage) })
  }

  return (
    <>
      <a className="skip-link" href="#main-content">
        跳到主要内容
      </a>
      <header className="site-header">
        <div className="site-header__inner">
          <Link className="site-brand" to="/" aria-label="hxy.blog 首页">
            hxy.blog
          </Link>
          <nav className="site-nav" aria-label="全局导航">
            <Link className="site-nav__link site-nav__link--active" to="/" aria-current="page">
              文章
            </Link>
            <a className="site-nav__link" href="#about">
              关于
            </a>
          </nav>
        </div>
      </header>

      <main className="page-shell" id="main-content">
        <div className="page-grid">
          <section className="post-column" aria-labelledby="latest-posts">
            <p className="intro">你好，我是 hxy。这里记录技术、工程实践与长期思考。</p>
            <div className="list-heading">
              <div>
                <p className="eyebrow">WRITING</p>
                <h1 id="latest-posts">最近写作</h1>
              </div>
              {data && <span className="post-count">{data.total} 篇</span>}
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
            {data && <PostList items={data.items} />}

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

          <aside className="profile" id="about" aria-label="关于博主">
            <section className="profile__section">
              <h2>关于</h2>
              <p>独立开发者，关注 Go、React、AI 工程化，以及如何把复杂系统讲清楚。</p>
            </section>
            <section className="profile__section">
              <h2>正在做</h2>
              <p>构建这个简单、可维护的个人博客。</p>
            </section>
          </aside>
        </div>
      </main>
    </>
  )
}

export default PostListPage
