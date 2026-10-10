import { type FormEvent, useState } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { useAppDispatch, useAppSelector } from '../../app/hooks'
import { login } from './authSlice'
import ThemeToggle from '../../components/ThemeToggle'

function LoginPage() {
  const dispatch = useAppDispatch()
  const { status, loginPending, loginError } = useAppSelector(
    (state) => state.auth,
  )
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  if (status === 'authenticated') {
    return <Navigate to="/admin" replace />
  }

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    void dispatch(login({ username: username.trim(), password }))
  }

  return (
    <main className="auth-shell">
      <div className="auth-shell__topline">
        <Link className="back-link" to="/">
          ← 返回博客
        </Link>
        <ThemeToggle />
      </div>

      <section className="auth-panel" aria-labelledby="login-title">
        <p className="eyebrow">ADMIN</p>
        <h1 id="login-title">登录管理后台</h1>
        <p className="auth-panel__intro">使用唯一管理员账号继续。</p>

        <form className="auth-form" onSubmit={submit}>
          <label htmlFor="username">用户名</label>
          <input
            id="username"
            name="username"
            type="text"
            autoComplete="username"
            autoFocus
            required
            value={username}
            onChange={(event) => setUsername(event.target.value)}
          />

          <label htmlFor="password">密码</label>
          <input
            id="password"
            name="password"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />

          {loginError && (
            <p className="auth-form__error" role="alert">
              {loginError}
            </p>
          )}

          <button type="submit" disabled={loginPending}>
            {loginPending ? '正在登录…' : '登录'}
          </button>
        </form>
      </section>
    </main>
  )
}

export default LoginPage
