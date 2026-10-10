import { Link, useLocation } from 'react-router-dom'
import ThemeToggle from './ThemeToggle'

function SiteHeader() {
  const isListPage = useLocation().pathname === '/'

  return (
    <header className="site-header">
      <div className="site-header__inner">
        <Link className="site-brand" to="/" aria-label="hxy.blog 首页">
          <span className="site-brand__mark" aria-hidden="true">h.</span>
          <span className="site-brand__name">hxy.blog</span>
        </Link>
        <span className="site-header__note">技术、工程与长期思考的个人记录</span>
        <nav className="site-nav" aria-label="全局导航">
          <Link
            className="site-nav__link"
            to="/"
            aria-current={isListPage ? 'page' : undefined}
          >
            文章
          </Link>
        </nav>
        <ThemeToggle />
      </div>
    </header>
  )
}

export default SiteHeader
