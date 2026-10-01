import { Link } from 'react-router-dom'
import AdminPostList from '../posts/AdminPostList'
import { useListAdminPostsQuery } from '../posts/postApi'
import AdminHeader from './AdminHeader'

function AdminPage() {
  const { data, isLoading, isError } = useListAdminPostsQuery()

  return (
    <main className="admin-shell">
      <AdminHeader title="内容管理" />

      <section className="admin-content" aria-labelledby="admin-posts-title">
        <div className="admin-content__heading">
          <div>
            <p className="eyebrow">POSTS</p>
            <h2 id="admin-posts-title">文章</h2>
          </div>
          <Link className="button-primary" to="/admin/posts/new">
            新建草稿
          </Link>
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
        {data && <AdminPostList items={data.items} />}
      </section>
    </main>
  )
}

export default AdminPage
