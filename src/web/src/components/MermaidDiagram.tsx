import { useEffect, useId, useState } from 'react'
import { useTheme } from '../app/ThemeProvider'

let mermaidLoader: Promise<typeof import('mermaid').default> | undefined

function loadMermaid() {
  if (!mermaidLoader) {
    mermaidLoader = import('mermaid').then(({ default: mermaid }) => mermaid)
  }
  return mermaidLoader
}

interface MermaidDiagramProps {
  source: string
}

function MermaidDiagram({ source }: MermaidDiagramProps) {
  const { theme } = useTheme()
  const reactID = useId()
  const [result, setResult] = useState<{
    source: string
    theme: string
    svg: string | null
    failed: boolean
  } | null>(null)

  useEffect(() => {
    let active = true
    const diagramID = `mermaid-${reactID.replace(/[^a-zA-Z0-9_-]/g, '')}`
    void loadMermaid()
      .then((mermaid) => {
        // 严格模式禁止图表源码注入脚本或任意链接；配色写入 SVG，切换时重新渲染。
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: 'strict',
          theme: theme === 'dark' ? 'dark' : 'neutral',
        })
        return mermaid.render(diagramID, source)
      })
      .then(({ svg: rendered }) => {
        if (active) setResult({ source, theme, svg: rendered, failed: false })
      })
      .catch(() => {
        if (active) setResult({ source, theme, svg: null, failed: true })
      })
    return () => {
      active = false
    }
  }, [reactID, source, theme])

  if (result?.source === source && result.theme === theme && result.failed) {
    return (
      <pre className="mermaid-error">
        <code>{source}</code>
        <span>图表语法无法渲染</span>
      </pre>
    )
  }
  if (result?.source !== source || result.theme !== theme || !result.svg) {
    return (
      <div className="mermaid-loading" role="status">
        正在渲染图表…
      </div>
    )
  }
  return (
    <div
      className="mermaid-diagram"
      // SVG 只由严格安全模式下的 Mermaid 生成，不接受文章中的原始 HTML。
      dangerouslySetInnerHTML={{ __html: result.svg }}
    />
  )
}

export default MermaidDiagram
