import {
  type ChangeEvent,
  type DragEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useBlocker } from 'react-router-dom'
import { useAppDispatch, useAppSelector } from '../../app/hooks'
import { refreshSession, isApiError } from '../auth/authClient'
import { sessionCleared, sessionReceived } from '../auth/authSlice'
import { uploadMedia } from './mediaClient'
import {
  altTextChanged,
  filesSelected,
  uploadCancelled,
  uploadFailed,
  uploadProgressed,
  uploadQueued,
  uploadRemoved,
  uploadStarted,
  uploadSucceeded,
  uploadsCleared,
  type MediaUploadItem,
} from './mediaSlice'

const maxFileSize = 10 * 1024 * 1024
const supportedTypes = new Set(['image/jpeg', 'image/png', 'image/webp'])

interface MediaUploadPanelProps {
  onInsertMarkdown: (altText: string, url: string) => void
}

function validationError(file: File) {
  if (file.size > maxFileSize) {
    return '图片超过 10 MiB，请压缩后重新选择。'
  }
  if (file.type && !supportedTypes.has(file.type)) {
    return '仅支持 JPG、PNG 和 WebP 图片。'
  }
  return null
}

function formatBytes(bytes: number) {
  if (bytes < 1024 * 1024) {
    return `${Math.max(1, Math.round(bytes / 1024))} KiB`
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
}

function statusText(item: MediaUploadItem) {
  switch (item.status) {
    case 'selected':
      return '填写替代文本后即可上传'
    case 'queued':
      return '等待上传'
    case 'uploading':
      return `正在上传 ${item.progress}%`
    case 'uploaded':
      return '上传完成 · 已插入正文'
    case 'cancelled':
      return '上传已取消'
    case 'invalid':
    case 'failed':
      return '上传失败'
  }
}

function canRetry(status: number) {
  return status === 0 || status === 429 || status === 503 || status >= 500
}

function MediaUploadPanel({ onInsertMarkdown }: MediaUploadPanelProps) {
  const dispatch = useAppDispatch()
  const items = useAppSelector((state) => state.media.items)
  const accessToken = useAppSelector((state) => state.auth.accessToken)
  const files = useRef(new Map<string, File>())
  const controllers = useRef(new Map<string, AbortController>())
  const fileInput = useRef<HTMLInputElement>(null)
  const [dragActive, setDragActive] = useState(false)
  const [copiedID, setCopiedID] = useState<string | null>(null)
  const [copyErrorID, setCopyErrorID] = useState<string | null>(null)
  const nextQueued = useMemo(
    () => items.find((item) => item.status === 'queued'),
    [items],
  )
  const hasPendingUpload = items.some((item) =>
    ['queued', 'uploading'].includes(item.status),
  )
  const blocker = useBlocker(hasPendingUpload)

  useEffect(() => {
    if (blocker.state !== 'blocked') {
      return
    }
    if (window.confirm('图片仍在上传，离开页面会取消上传。确定离开吗？')) {
      blocker.proceed()
    } else {
      blocker.reset()
    }
  }, [blocker])

  useEffect(() => {
    if (!hasPendingUpload) {
      return
    }
    const warnBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', warnBeforeUnload)
    return () => window.removeEventListener('beforeunload', warnBeforeUnload)
  }, [hasPendingUpload])

  useEffect(() => {
    const activeControllers = controllers.current
    const selectedFiles = files.current
    return () => {
      for (const controller of activeControllers.values()) {
        controller.abort()
      }
      activeControllers.clear()
      selectedFiles.clear()
      dispatch(uploadsCleared())
    }
  }, [dispatch])

  useEffect(() => {
    if (!nextQueued || controllers.current.has(nextQueued.id)) {
      return
    }
    const file = files.current.get(nextQueued.id)
    if (!file || !accessToken) {
      dispatch(
        uploadFailed({
          id: nextQueued.id,
          message: file ? '登录已失效，请重新登录。' : '原始文件已不可用，请重新选择。',
          retryable: false,
        }),
      )
      return
    }

    const controller = new AbortController()
    controllers.current.set(nextQueued.id, controller)
    dispatch(uploadStarted(nextQueued.id))

    const reportProgress = (progress: number) => {
      dispatch(uploadProgressed({ id: nextQueued.id, progress }))
    }
    const run = async () => {
      try {
        let asset
        try {
          asset = await uploadMedia({
            file,
            accessToken,
            signal: controller.signal,
            onProgress: reportProgress,
          })
        } catch (error: unknown) {
          if (!isApiError(error) || error.status !== 401) {
            throw error
          }
          let session
          try {
            session = await refreshSession()
          } catch (refreshError: unknown) {
            dispatch(sessionCleared())
            throw refreshError
          }
          dispatch(sessionReceived(session))
          asset = await uploadMedia({
            file,
            accessToken: session.accessToken,
            signal: controller.signal,
            onProgress: reportProgress,
          })
        }
        dispatch(uploadSucceeded({ id: nextQueued.id, asset }))
        onInsertMarkdown(nextQueued.altText.trim(), asset.url)
      } catch (error: unknown) {
        if (controller.signal.aborted) {
          dispatch(uploadCancelled(nextQueued.id))
          return
        }
        const apiError = isApiError(error)
          ? error
          : { status: 0, message: '图片上传失败，请稍后重试。' }
        dispatch(
          uploadFailed({
            id: nextQueued.id,
            message: apiError.message,
            retryable: canRetry(apiError.status),
          }),
        )
      } finally {
        controllers.current.delete(nextQueued.id)
      }
    }
    void run()
  }, [accessToken, dispatch, nextQueued, onInsertMarkdown])

  const addFiles = (selected: File[]) => {
    const descriptors = selected.map((file) => {
      const id = crypto.randomUUID()
      files.current.set(id, file)
      return {
        id,
        name: file.name,
        sizeBytes: file.size,
        mimeType: file.type,
        validationError: validationError(file),
      }
    })
    if (descriptors.length > 0) {
      dispatch(filesSelected(descriptors))
    }
    if (fileInput.current) {
      fileInput.current.value = ''
    }
  }

  const selectFiles = (event: ChangeEvent<HTMLInputElement>) => {
    addFiles(Array.from(event.target.files ?? []))
  }

  const dropFiles = (event: DragEvent<HTMLLabelElement>) => {
    event.preventDefault()
    setDragActive(false)
    addFiles(Array.from(event.dataTransfer.files))
  }

  const cancelUpload = (id: string) => {
    const controller = controllers.current.get(id)
    if (controller) {
      controller.abort()
    } else {
      dispatch(uploadCancelled(id))
    }
  }

  const removeUpload = (id: string) => {
    controllers.current.get(id)?.abort()
    controllers.current.delete(id)
    files.current.delete(id)
    dispatch(uploadRemoved(id))
  }

  const copyURL = async (item: MediaUploadItem) => {
    if (!item.asset) {
      return
    }
    try {
      await navigator.clipboard.writeText(item.asset.url)
      setCopiedID(item.id)
      setCopyErrorID(null)
    } catch {
      setCopiedID(null)
      setCopyErrorID(item.id)
    }
  }

  return (
    <section className="media-uploader" aria-labelledby="media-upload-title">
      <div className="media-uploader__heading">
        <div>
          <h3 id="media-upload-title">插入图片</h3>
          <p>JPG、PNG、WebP · 单张不超过 10 MiB</p>
        </div>
      </div>

      <label
        className={`media-drop-zone${dragActive ? ' media-drop-zone--active' : ''}`}
        onDragEnter={() => setDragActive(true)}
        onDragLeave={() => setDragActive(false)}
        onDragOver={(event) => event.preventDefault()}
        onDrop={dropFiles}
      >
        <input
          ref={fileInput}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          multiple
          onChange={selectFiles}
        />
        <span className="media-drop-zone__icon" aria-hidden="true">
          ↑
        </span>
        <strong>拖放图片，或点击选择</strong>
        <span>可一次选择多张；文件会逐张上传，不阻塞正文编辑。</span>
      </label>

      {items.length > 0 && (
        <ul className="media-upload-list" aria-live="polite">
          {items.map((item) => {
            const editable = ['selected', 'failed', 'cancelled'].includes(
              item.status,
            )
            return (
              <li
                className={`media-upload-item media-upload-item--${item.status}`}
                key={item.id}
              >
                <div className="media-upload-item__summary">
                  {item.asset ? (
                    <img src={item.asset.url} alt={item.altText} />
                  ) : (
                    <span className="media-upload-item__placeholder" aria-hidden="true">
                      IMG
                    </span>
                  )}
                  <div>
                    <strong>{item.name}</strong>
                    <span>
                      {formatBytes(item.sizeBytes)} · {statusText(item)}
                    </span>
                  </div>
                </div>

                <label htmlFor={`media-alt-${item.id}`}>替代文本</label>
                <input
                  id={`media-alt-${item.id}`}
                  aria-required="true"
                  disabled={!editable || item.status === 'invalid'}
                  value={item.altText}
                  placeholder="描述图片传达的信息"
                  onChange={(event) =>
                    dispatch(
                      altTextChanged({
                        id: item.id,
                        altText: event.target.value,
                      }),
                    )
                  }
                />

                {['queued', 'uploading'].includes(item.status) && (
                  <progress
                    max="100"
                    value={item.progress}
                    aria-label={`${item.name} 上传进度 ${item.progress}%`}
                  />
                )}
                {item.error && (
                  <p className="media-upload-item__error" role="alert">
                    {item.error}
                  </p>
                )}
                {item.asset && (
                  <output className="media-upload-item__url">
                    {item.asset.url}
                  </output>
                )}

                <div className="media-upload-item__actions">
                  {item.status === 'selected' && (
                    <button
                      className="button-primary"
                      type="button"
                      disabled={!item.altText.trim()}
                      onClick={() => dispatch(uploadQueued(item.id))}
                    >
                      上传并插入
                    </button>
                  )}
                  {['queued', 'uploading'].includes(item.status) && (
                    <button
                      className="button-secondary"
                      type="button"
                      onClick={() => cancelUpload(item.id)}
                    >
                      取消上传
                    </button>
                  )}
                  {item.status === 'failed' && item.retryable && (
                    <button
                      className="button-primary"
                      type="button"
                      disabled={!item.altText.trim()}
                      onClick={() => dispatch(uploadQueued(item.id))}
                    >
                      重试上传
                    </button>
                  )}
                  {item.status === 'cancelled' && (
                    <button
                      className="button-primary"
                      type="button"
                      disabled={!item.altText.trim()}
                      onClick={() => dispatch(uploadQueued(item.id))}
                    >
                      重新上传
                    </button>
                  )}
                  {item.asset && (
                    <>
                      <button
                        className="button-secondary"
                        type="button"
                        onClick={() => void copyURL(item)}
                      >
                        复制 URL
                      </button>
                      <button
                        className="button-secondary"
                        type="button"
                        onClick={() =>
                          onInsertMarkdown(item.altText, item.asset!.url)
                        }
                      >
                        再次插入
                      </button>
                    </>
                  )}
                  {!['queued', 'uploading'].includes(item.status) && (
                    <button
                      className="button-secondary"
                      type="button"
                      onClick={() => removeUpload(item.id)}
                    >
                      移除
                    </button>
                  )}
                </div>
                {copiedID === item.id && (
                  <p className="media-upload-item__feedback" role="status">
                    URL 已复制。
                  </p>
                )}
                {copyErrorID === item.id && (
                  <p className="media-upload-item__error" role="alert">
                    自动复制失败，请手动选择上方 URL。
                  </p>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}

export default MediaUploadPanel
