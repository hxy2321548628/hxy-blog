import { useTheme } from '../app/ThemeProvider'

function ThemeToggle() {
  const { theme, toggleTheme } = useTheme()
  const nextTheme = theme === 'dark' ? '浅色' : '暗色'

  return (
    <button
      className="theme-toggle"
      type="button"
      onClick={toggleTheme}
      aria-label={`切换为${nextTheme}模式`}
    >
      <span aria-hidden="true">{theme === 'dark' ? '☀' : '◐'}</span>
      <span>{nextTheme}模式</span>
    </button>
  )
}

export default ThemeToggle
