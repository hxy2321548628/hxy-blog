import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react'
import type { AxiosRequestConfig } from 'axios'
import { apiErrorFrom, httpClient, type ApiError } from '../../app/httpClient'
import { refreshSession } from '../auth/authClient'
import {
  sessionCleared,
  sessionReceived,
  type AuthState,
} from '../auth/authSlice'

interface RequestArgs {
  url: string
  method?: AxiosRequestConfig['method']
  data?: unknown
  params?: Record<string, number | string>
}

export interface PostSummary {
  slug: string
  title: string
  publishedAt: string
}

export interface PostListResponse {
  items: PostSummary[]
  page: number
  pageSize: number
  total: number
}

export interface PostDetailResponse {
  slug: string
  title: string
  contentMarkdown: string
  publishedAt: string
}

export interface AdminPostSummary {
  id: number
  slug: string
  title: string
  status: 'draft' | 'published'
  publishedAt: string | null
  updatedAt: string
}

export interface AdminPostDetail extends AdminPostSummary {
  contentMarkdown: string
  createdAt: string
}

export interface DraftInput {
  slug: string
  title: string
  contentMarkdown: string
}

interface AdminPostListResponse {
  items: AdminPostSummary[]
}

interface ListPostsParams {
  page: number
  pageSize: number
}

const axiosBaseQuery: BaseQueryFn<RequestArgs, unknown, ApiError> = async (
  request,
  api,
) => {
  const send = async (accessToken: string | null) => {
    return httpClient.request({
      url: request.url,
      method: request.method,
      data: request.data,
      params: request.params,
      headers: accessToken
        ? { Authorization: `Bearer ${accessToken}` }
        : undefined,
    })
  }

  const state = api.getState() as { auth: AuthState }
  const accessToken = state.auth.accessToken
  try {
    const response = await send(accessToken)
    return { data: response.data }
  } catch (error: unknown) {
    const apiError = apiErrorFrom(error)
    if (apiError.status === 401 && accessToken) {
      try {
        const session = await refreshSession()
        api.dispatch(sessionReceived(session))
        const response = await send(session.accessToken)
        return { data: response.data }
      } catch {
        api.dispatch(sessionCleared())
      }
    }
    // 前端只根据稳定错误码交互；非预期响应统一收敛为网络错误。
    return { error: apiError }
  }
}

export const postApi = createApi({
  reducerPath: 'postApi',
  baseQuery: axiosBaseQuery,
  tagTypes: ['AdminPosts', 'PublishedPosts'],
  endpoints: (builder) => ({
    listPosts: builder.query<PostListResponse, ListPostsParams>({
      query: ({ page, pageSize }) => ({
        url: '/posts',
        method: 'GET',
        params: { page, pageSize },
      }),
      providesTags: ['PublishedPosts'],
    }),
    getPost: builder.query<PostDetailResponse, string>({
      query: (slug) => ({
        url: `/posts/${encodeURIComponent(slug)}`,
        method: 'GET',
      }),
      providesTags: ['PublishedPosts'],
    }),
    listAdminPosts: builder.query<AdminPostListResponse, void>({
      query: () => ({ url: '/admin/posts', method: 'GET' }),
      providesTags: ['AdminPosts'],
    }),
    getAdminPost: builder.query<AdminPostDetail, number>({
      query: (id) => ({ url: `/admin/posts/${id}`, method: 'GET' }),
      providesTags: (_result, _error, id) => [{ type: 'AdminPosts', id }],
    }),
    createDraft: builder.mutation<AdminPostDetail, DraftInput>({
      query: (draft) => ({ url: '/admin/posts', method: 'POST', data: draft }),
      invalidatesTags: ['AdminPosts'],
    }),
    updateDraft: builder.mutation<
      AdminPostDetail,
      { id: number; draft: DraftInput }
    >({
      query: ({ id, draft }) => ({
        url: `/admin/posts/${id}`,
        method: 'PUT',
        data: draft,
      }),
      invalidatesTags: (_result, _error, { id }) => [
        'AdminPosts',
        { type: 'AdminPosts', id },
      ],
    }),
    publishPost: builder.mutation<AdminPostDetail, number>({
      query: (id) => ({ url: `/admin/posts/${id}/publish`, method: 'POST' }),
      invalidatesTags: (_result, _error, id) => [
        'AdminPosts',
        { type: 'AdminPosts', id },
        'PublishedPosts',
      ],
    }),
  }),
})

export const {
  useCreateDraftMutation,
  useGetAdminPostQuery,
  useGetPostQuery,
  useListAdminPostsQuery,
  useListPostsQuery,
  usePublishPostMutation,
  useUpdateDraftMutation,
} = postApi
