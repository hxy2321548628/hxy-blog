import { describe, expect, it } from 'vitest'
import { validationError } from './MediaUploadPanel'

describe('图片上传格式预检', () => {
  it.each(['image/bmp', 'image/x-ms-bmp', 'image/gif'])(
    '允许 %s 文件进入服务端内容校验',
    (mimeType) => {
      const file = new File(['image'], 'picture', { type: mimeType })
      expect(validationError(file)).toBeNull()
    },
  )

  it('拒绝 SVG，保留原有文件大小限制', () => {
    const svg = new File(['<svg/>'], 'picture.svg', { type: 'image/svg+xml' })
    expect(validationError(svg)).toContain('仅支持')
    const largeGIF = new File([new Uint8Array(10 * 1024 * 1024 + 1)], 'large.gif', {
      type: 'image/gif',
    })
    expect(validationError(largeGIF)).toContain('10 MiB')
  })
})
