import { describe, expect, it, vi } from 'vitest'
import { readSavedTheme, resolveTheme, saveTheme } from './theme'

describe('theme preference', () => {
  it('没有有效的手动选择时跟随系统偏好', () => {
    expect(resolveTheme(null, true)).toBe('dark')
    expect(resolveTheme(null, false)).toBe('light')
    expect(resolveTheme('unexpected', true)).toBe('dark')
  })

  it('手动选择优先于系统偏好', () => {
    expect(resolveTheme('light', true)).toBe('light')
    expect(resolveTheme('dark', false)).toBe('dark')
  })

  it('只读取有效的主题值，存储不可用时仍可使用系统偏好', () => {
    expect(readSavedTheme({ getItem: () => 'dark' })).toBe('dark')
    expect(readSavedTheme({ getItem: () => 'unexpected' })).toBeNull()
    expect(readSavedTheme({ getItem: () => { throw new Error('blocked') } })).toBeNull()
  })

  it('写入失败不会阻止当前页面切换', () => {
    const setItem = vi.fn(() => { throw new Error('blocked') })
    expect(() => saveTheme({ setItem }, 'dark')).not.toThrow()
    expect(setItem).toHaveBeenCalledWith('hxy-blog-theme', 'dark')
  })
})
