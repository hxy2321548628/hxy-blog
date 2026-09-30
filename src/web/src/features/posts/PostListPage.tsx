import { useSearchParams } from 'react-router-dom'
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
    <main className="shell">
      <header className="hero">
        <p className="eyebrow">HXY BLOG</p>
        <h1>写下值得留住的事</h1>
        <p className="summary">关于软件工程、学习和日常思考的个人记录。</p>
      </header>

      <section aria-labelledby="latest-posts">
        <div className="section-heading">
          <h2 id="latest-posts">最新文章</h2>
          {data && <span>{data.total} 篇</span>}
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
    </main>
  )
}

export default PostListPage
