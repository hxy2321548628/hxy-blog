import { apiErrorFrom, httpClient } from '../../app/httpClient'
import type { MediaAsset } from './mediaSlice'

interface UploadMediaOptions {
  file: File
  accessToken: string
  signal: AbortSignal
  onProgress: (progress: number) => void
}

export async function uploadMedia({
  file,
  accessToken,
  signal,
  onProgress,
}: UploadMediaOptions): Promise<MediaAsset> {
  const formData = new FormData()
  formData.append('file', file)
  try {
    const response = await httpClient.post<MediaAsset>('/admin/media', formData, {
      headers: { Authorization: `Bearer ${accessToken}` },
      onUploadProgress: (event) => {
        const total = event.total ?? file.size
        if (total > 0) {
          // 100% 只在服务端确认 COS 与元数据均成功后显示。
          onProgress(Math.min(99, Math.round((event.loaded / total) * 100)))
        }
      },
      signal,
      timeout: 40_000,
    })
    return response.data
  } catch (error: unknown) {
    throw apiErrorFrom(error)
  }
}
