export type Theme = 'light' | 'dark'

export const themeStorageKey = 'hxy-blog-theme'

export function resolveTheme(saved: string | null, prefersDark: boolean): Theme {
  if (saved === 'light' || saved === 'dark') return saved
  return prefersDark ? 'dark' : 'light'
}

export function readSavedTheme(storage: Pick<Storage, 'getItem'>): Theme | null {
  try {
    const saved = storage.getItem(themeStorageKey)
    return saved === 'light' || saved === 'dark' ? saved : null
  } catch {
    // 隐私设置可能禁止本地存储；主题仍可按系统偏好运行。
    return null
  }
}

export function saveTheme(storage: Pick<Storage, 'setItem'>, theme: Theme) {
  try {
    storage.setItem(themeStorageKey, theme)
  } catch {
    // 写入失败只影响下次访问，不应阻断当前页面的切换。
  }
}
