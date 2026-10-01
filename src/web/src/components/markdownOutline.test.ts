import { describe, expect, it } from 'vitest'
import { extractOutline } from './markdownOutline'

describe('extractOutline', () => {
  it('提取二三级标题并为重名标题生成唯一锚点', () => {
    const outline = extractOutline(`
# 文章标题
## Go **并发**
### 实现
## Go 并发

\`\`\`markdown
## 不应进入大纲
\`\`\`
`)

    expect(outline).toEqual([
      { id: 'user-content-go-并发', text: 'Go 并发', depth: 2 },
      { id: 'user-content-实现', text: '实现', depth: 3 },
      { id: 'user-content-go-并发-1', text: 'Go 并发', depth: 2 },
    ])
  })
})
