import { Link } from 'react-router-dom'

function SiteFooter() {
  return (
    <footer className="site-footer">
      <div className="site-footer__inner">
        <Link to="/">hxy.blog</Link>
        <span>思考有迹，文字有形。</span>
        <span>© {new Date().getFullYear()} HXY</span>
      </div>
    </footer>
  )
}

export default SiteFooter
