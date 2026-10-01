import { describe, expect, it } from 'vitest'
import { insertMarkdownImage } from './markdownImage'

describe('insertMarkdownImage', () => {
  it('在光标位置以独立段落插入图片', () => {
    const result = insertMarkdownImage(
      '前文后文',
      2,
      2,
      '系统架构图',
      'https://media.example.com/media/architecture.png',
    )

    expect(result.value).toBe(
      '前文\n\n![系统架构图](https://media.example.com/media/architecture.png)\n\n后文',
    )
    expect(result.cursor).toBe(
      '前文\n\n![系统架构图](https://media.example.com/media/architecture.png)'.length,
    )
  })

  it('替换选区并转义替代文本中的 Markdown 字符', () => {
    const result = insertMarkdownImage(
      'before selected after',
      7,
      15,
      '流程\\图]',
      'https://media.example.com/image.webp',
    )

    expect(result.value).toBe(
      'before \n\n![流程\\\\图\\]](https://media.example.com/image.webp)\n\n after',
    )
  })
})
