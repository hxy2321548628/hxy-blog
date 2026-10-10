import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import SiteHeader from './SiteHeader'
import ThemeProvider from '../app/ThemeProvider'

describe('SiteHeader', () => {
  it('只展示已有页面的导航入口', () => {
    const html = renderToStaticMarkup(
      <MemoryRouter>
        <ThemeProvider>
          <SiteHeader />
        </ThemeProvider>
      </MemoryRouter>,
    )

    expect(html).toContain('href="/"')
    expect(html).toContain('文章')
    expect(html).not.toContain('关于')
    expect(html).not.toContain('#about')
  })
})
