import { describe, expect, it } from 'vitest'
import reducer, {
  altTextChanged,
  filesSelected,
  uploadFailed,
  uploadProgressed,
  uploadQueued,
  uploadStarted,
  uploadSucceeded,
} from './mediaSlice'

describe('mediaSlice', () => {
  it('只在填写替代文本后进入上传队列，并记录成功结果', () => {
    let state = reducer(
      undefined,
      filesSelected([
        {
          id: 'file-1',
          name: 'architecture.png',
          sizeBytes: 2048,
          mimeType: 'image/png',
          validationError: null,
        },
      ]),
    )

    state = reducer(state, uploadQueued('file-1'))
    expect(state.items[0].status).toBe('selected')

    state = reducer(
      state,
      altTextChanged({ id: 'file-1', altText: '系统架构图' }),
    )
    state = reducer(state, uploadQueued('file-1'))
    state = reducer(state, uploadStarted('file-1'))
    state = reducer(
      state,
      uploadProgressed({ id: 'file-1', progress: 100 }),
    )
    expect(state.items[0].progress).toBe(99)

    state = reducer(
      state,
      uploadSucceeded({
        id: 'file-1',
        asset: {
          id: 7,
          url: 'https://media.example.com/media/image.png',
          mimeType: 'image/png',
          sizeBytes: 2048,
          width: 800,
          height: 600,
          createdAt: '2026-10-01T00:00:00Z',
        },
      }),
    )
    expect(state.items[0]).toMatchObject({
      status: 'uploaded',
      progress: 100,
      retryable: false,
    })
  })

  it('保留文件级校验错误和可重试失败状态', () => {
    let state = reducer(
      undefined,
      filesSelected([
        {
          id: 'invalid',
          name: 'script.svg',
          sizeBytes: 32,
          mimeType: 'image/svg+xml',
          validationError: '仅支持 JPG、PNG 和 WebP 图片。',
        },
        {
          id: 'retry',
          name: 'photo.jpg',
          sizeBytes: 1024,
          mimeType: 'image/jpeg',
          validationError: null,
        },
      ]),
    )
    state = reducer(
      state,
      uploadFailed({
        id: 'retry',
        message: '图片存储暂时不可用，请稍后重试',
        retryable: true,
      }),
    )

    expect(state.items[0]).toMatchObject({
      status: 'invalid',
      retryable: false,
    })
    expect(state.items[1]).toMatchObject({
      status: 'failed',
      retryable: true,
    })
  })
})
