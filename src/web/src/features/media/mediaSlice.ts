import { createSlice, type PayloadAction } from '@reduxjs/toolkit'

export interface MediaAsset {
  id: number
  url: string
  mimeType: string
  sizeBytes: number
  width: number
  height: number
  createdAt: string
}

export type MediaUploadStatus =
  | 'selected'
  | 'invalid'
  | 'queued'
  | 'uploading'
  | 'uploaded'
  | 'failed'
  | 'cancelled'

export interface MediaUploadItem {
  id: string
  name: string
  sizeBytes: number
  mimeType: string
  altText: string
  status: MediaUploadStatus
  progress: number
  error: string | null
  retryable: boolean
  asset: MediaAsset | null
}

interface MediaState {
  items: MediaUploadItem[]
}

interface SelectedFile {
  id: string
  name: string
  sizeBytes: number
  mimeType: string
  validationError: string | null
}

const initialState: MediaState = { items: [] }

function findItem(state: MediaState, id: string) {
  return state.items.find((item) => item.id === id)
}

const mediaSlice = createSlice({
  name: 'media',
  initialState,
  reducers: {
    filesSelected(state, action: PayloadAction<SelectedFile[]>) {
      for (const file of action.payload) {
        state.items.push({
          id: file.id,
          name: file.name,
          sizeBytes: file.sizeBytes,
          mimeType: file.mimeType,
          altText: '',
          status: file.validationError ? 'invalid' : 'selected',
          progress: 0,
          error: file.validationError,
          retryable: false,
          asset: null,
        })
      }
    },
    altTextChanged(
      state,
      action: PayloadAction<{ id: string; altText: string }>,
    ) {
      const item = findItem(state, action.payload.id)
      if (item && ['selected', 'failed', 'cancelled'].includes(item.status)) {
        item.altText = action.payload.altText
      }
    },
    uploadQueued(state, action: PayloadAction<string>) {
      const item = findItem(state, action.payload)
      if (
        item &&
        item.altText.trim() &&
        ['selected', 'failed', 'cancelled'].includes(item.status)
      ) {
        item.status = 'queued'
        item.progress = 0
        item.error = null
        item.retryable = false
      }
    },
    uploadStarted(state, action: PayloadAction<string>) {
      const item = findItem(state, action.payload)
      if (item?.status === 'queued') {
        item.status = 'uploading'
      }
    },
    uploadProgressed(
      state,
      action: PayloadAction<{ id: string; progress: number }>,
    ) {
      const item = findItem(state, action.payload.id)
      if (item?.status === 'uploading') {
        item.progress = Math.max(0, Math.min(99, action.payload.progress))
      }
    },
    uploadSucceeded(
      state,
      action: PayloadAction<{ id: string; asset: MediaAsset }>,
    ) {
      const item = findItem(state, action.payload.id)
      if (item) {
        item.status = 'uploaded'
        item.progress = 100
        item.error = null
        item.retryable = false
        item.asset = action.payload.asset
      }
    },
    uploadFailed(
      state,
      action: PayloadAction<{
        id: string
        message: string
        retryable: boolean
      }>,
    ) {
      const item = findItem(state, action.payload.id)
      if (item) {
        item.status = 'failed'
        item.error = action.payload.message
        item.retryable = action.payload.retryable
      }
    },
    uploadCancelled(state, action: PayloadAction<string>) {
      const item = findItem(state, action.payload)
      if (item && ['queued', 'uploading'].includes(item.status)) {
        item.status = 'cancelled'
        item.error = '上传已取消，可以重新上传。'
        item.retryable = true
      }
    },
    uploadRemoved(state, action: PayloadAction<string>) {
      state.items = state.items.filter((item) => item.id !== action.payload)
    },
    uploadsCleared(state) {
      state.items = []
    },
  },
})

export const {
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
} = mediaSlice.actions

export default mediaSlice.reducer
