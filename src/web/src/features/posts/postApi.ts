import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react'
import axios, { type AxiosRequestConfig } from 'axios'

interface RequestArgs {
  url: string
  method?: AxiosRequestConfig['method']
  data?: unknown
  params?: Record<string, number | string>
}

interface ApiError {
  status: number
  code: string
  message: string
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

interface ListPostsParams {
  page: number
  pageSize: number
}

const client = axios.create({
  baseURL: '/api',
  timeout: 10_000,
})

const axiosBaseQuery: BaseQueryFn<RequestArgs, unknown, ApiError> = async (
  request,
) => {
  try {
    const response = await client.request({
      url: request.url,
      method: request.method,
      data: request.data,
      params: request.params,
    })
    return { data: response.data }
  } catch (error: unknown) {
    // 前端只根据稳定错误码交互；非预期响应统一收敛为网络错误。
    if (axios.isAxiosError(error)) {
      const payload: unknown = error.response?.data
      if (
        typeof payload === 'object' &&
        payload !== null &&
        'code' in payload &&
        'message' in payload &&
        typeof payload.code === 'string' &&
        typeof payload.message === 'string'
      ) {
        return {
          error: {
            status: error.response?.status ?? 0,
            code: payload.code,
            message: payload.message,
          },
        }
      }
    }
    return {
      error: { status: 0, code: 'NETWORK_ERROR', message: '无法连接服务器' },
    }
  }
}

export const postApi = createApi({
  reducerPath: 'postApi',
  baseQuery: axiosBaseQuery,
  endpoints: (builder) => ({
    listPosts: builder.query<PostListResponse, ListPostsParams>({
      query: ({ page, pageSize }) => ({
        url: '/posts',
        method: 'GET',
        params: { page, pageSize },
      }),
    }),
  }),
})

export const { useListPostsQuery } = postApi
