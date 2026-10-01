// 刷新令牌采用一次性轮换；合并并发请求可避免第二个请求被服务端判定为重放攻击。
export function createRefreshCoordinator<Result>(request: () => Promise<Result>) {
  let pending: Promise<Result> | null = null

  return () => {
    if (!pending) {
      pending = request().finally(() => {
        pending = null
      })
    }
    return pending
  }
}
