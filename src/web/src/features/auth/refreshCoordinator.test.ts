import { describe, expect, it, vi } from 'vitest'
import { createRefreshCoordinator } from './refreshCoordinator'

describe('createRefreshCoordinator', () => {
  it('把同时发生的刷新合并为一个请求', async () => {
    const request = vi.fn(async () => ({ accessToken: 'next-token' }))
    const refresh = createRefreshCoordinator(request)

    const [first, second] = await Promise.all([refresh(), refresh()])

    expect(request).toHaveBeenCalledTimes(1)
    expect(first).toEqual(second)
  })

  it('请求结束后允许下一次刷新', async () => {
    const request = vi.fn(async () => ({ accessToken: 'next-token' }))
    const refresh = createRefreshCoordinator(request)

    await refresh()
    await refresh()

    expect(request).toHaveBeenCalledTimes(2)
  })
})
