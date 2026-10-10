import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { readSavedTheme, resolveTheme, saveTheme, type Theme } from './theme'

const ThemeContext = createContext<{ theme: Theme; toggleTheme: () => void } | null>(null)

function savedTheme(): Theme | null {
  if (typeof window === 'undefined') return null
  try {
    return readSavedTheme(window.localStorage)
  } catch {
    return null
  }
}

function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => {
    if (typeof document === 'undefined') return 'light'
    const initial = document.documentElement.dataset.theme
    if (initial === 'light' || initial === 'dark') return initial
    return resolveTheme(savedTheme(), window.matchMedia('(prefers-color-scheme: dark)').matches)
  })
  const manuallySelected = useRef(savedTheme() !== null)

  useEffect(() => {
    // 只在尚未手动选择时跟随系统，防止系统切换覆盖用户偏好。
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const followSystem = (event: MediaQueryListEvent) => {
      if (!manuallySelected.current) setTheme(event.matches ? 'dark' : 'light')
    }
    media.addEventListener('change', followSystem)
    return () => media.removeEventListener('change', followSystem)
  }, [])

  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  const toggleTheme = () => {
    const next = theme === 'dark' ? 'light' : 'dark'
    manuallySelected.current = true
    try {
      saveTheme(window.localStorage, next)
    } catch {
      // localStorage 属性本身也可能被浏览器策略禁止访问。
    }
    setTheme(next)
  }

  return (
    <ThemeContext.Provider value={{ theme, toggleTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  const theme = useContext(ThemeContext)
  if (!theme) throw new Error('ThemeProvider is missing')
  return theme
}

export default ThemeProvider
