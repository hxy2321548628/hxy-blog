import { Link, useParams } from 'react-router-dom'
import SiteFooter from '../../components/SiteFooter'
import SiteHeader from '../../components/SiteHeader'
import PostDetail from './PostDetail'
import { useGetPostQuery } from './postApi'

function PostDetailPage() {
  const { slug } = useParams()
  const { data, error, isLoading } = useGetPostQuery(slug ?? '', {
    skip: !slug,
  })

  return (
    <>
      <a className="skip-link" href="#main-content">
        跳到主要内容
      </a>
      <SiteHeader />
      <main className="article-shell" id="main-content">
        <div className="article-topline">
          <Link className="back-link" to="/">← 返回文章列表</Link>
          <span>02 / READING</span>
        </div>
        {isLoading && (
          <p className="request-state" role="status">
            正在读取文章…
          </p>
        )}
        {error?.code === 'POST_NOT_FOUND' && (
          <section className="article-state" aria-labelledby="not-found-title">
            <p className="eyebrow">404</p>
            <h1 id="not-found-title">文章不存在</h1>
            <p>这篇文章可能尚未发布，或者地址已经发生变化。</p>
          </section>
        )}
        {error && error.code !== 'POST_NOT_FOUND' && (
          <p className="request-state request-state--error" role="alert">
            文章暂时无法加载，请稍后重试。
          </p>
        )}
        {data && <PostDetail post={data} />}
      </main>
      <SiteFooter />
    </>
  )
}

export default PostDetailPage
