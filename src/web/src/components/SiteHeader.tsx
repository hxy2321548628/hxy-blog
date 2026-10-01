import { Link } from 'react-router-dom'

function SiteHeader() {
  return (
    <header className="site-header">
      <div className="site-header__inner">
        <Link className="site-brand" to="/" aria-label="hxy.blog 首页">
          hxy.blog
        </Link>
        <nav className="site-nav" aria-label="全局导航">
          <Link
            className="site-nav__link site-nav__link--active"
            to="/"
            aria-current="page"
          >
            文章
          </Link>
        </nav>
      </div>
    </header>
  )
}

export default SiteHeader
