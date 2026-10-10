import axios, {
  type AxiosError,
  type AxiosRequestConfig,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'
import { message } from 'antd'
import type { MessageInstance } from 'antd/es/message/interface'
import { clearTokens, getAccessToken, getRefreshToken, setTokens } from '@/utils/token'
import { appConfig } from '@/config'
import type { ApiResponse, LoginResult } from '@/types'

/**
 * 由 main.tsx 中 <App> 内的桥接组件注入 App.useApp() 的 message 实例，
 * 使错误提示能消费 ConfigProvider 上下文（主题/暗色）；
 * 注入前（模块初始化极早期）回退到静态方法。
 */
let messageInstance: MessageInstance | null = null

export function bindMessageInstance(instance: MessageInstance): void {
  messageInstance = instance
}

function notifyError(content: string): void {
  if (messageInstance) {
    messageInstance.error(content)
  } else {
    message.error(content)
  }
}

/** 后端 Base URL（来自 .env 的 VITE_BASE_API，dev 经 vite proxy 转发） */
export const API_BASE_URL = appConfig.baseApi

/** 业务/网络统一错误对象 */
export class ApiError extends Error {
  code: number
  status?: number

  constructor(message: string, code: number, status?: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

/* ============================================================================
 * 拦截器链（数组式注册机制）
 * ----------------------------------------------------------------------------
 * 设计目的：把「认证、错误提示、埋点、loading、traceId」等横切关注点拆成
 * 独立的、可插拔的函数，方便后期按需追加而不改动核心请求逻辑。
 *
 * 使用方式（在任意模块的初始化处调用）：
 *   const off = registerRequestInterceptor((config) => {
 *     config.headers.set('X-Trace-Id', nanoid()) // 埋点示例
 *     return config
 *   })
 *   // 不再需要时调用 off() 注销
 *
 * - requestInterceptors：按注册顺序串行执行，可异步，返回值透传给下一个；
 * - responseInterceptors：同上，作用于成功响应（应答体已在默认拦截器里解包）；
 * - responseErrorInterceptors：作用于失败响应；若某个拦截器返回一个
 *   AxiosResponse（例如刷新 token 后重发原请求成功），则视为「已恢复」，
 *   不再向下传递错误，直接把该响应作为最终结果返回。
 * ========================================================================== */

export type RequestInterceptor = (
  config: InternalAxiosRequestConfig,
) => InternalAxiosRequestConfig | Promise<InternalAxiosRequestConfig>

export type ResponseInterceptor = (
  response: AxiosResponse,
) => AxiosResponse | Promise<AxiosResponse>

export type ResponseErrorInterceptorResult = AxiosError | AxiosResponse
export type ResponseErrorInterceptor = (
  error: AxiosError,
) => ResponseErrorInterceptorResult | Promise<ResponseErrorInterceptorResult>

export const requestInterceptors: RequestInterceptor[] = []
export const responseInterceptors: ResponseInterceptor[] = []
export const responseErrorInterceptors: ResponseErrorInterceptor[] = []

export function registerRequestInterceptor(fn: RequestInterceptor): () => void {
  requestInterceptors.push(fn)
  return () => {
    const i = requestInterceptors.indexOf(fn)
    if (i > -1) requestInterceptors.splice(i, 1)
  }
}

export function registerResponseInterceptor(fn: ResponseInterceptor): () => void {
  responseInterceptors.push(fn)
  return () => {
    const i = responseInterceptors.indexOf(fn)
    if (i > -1) responseInterceptors.splice(i, 1)
  }
}

export function registerResponseErrorInterceptor(fn: ResponseErrorInterceptor): () => void {
  responseErrorInterceptors.push(fn)
  return () => {
    const i = responseErrorInterceptors.indexOf(fn)
    if (i > -1) responseErrorInterceptors.splice(i, 1)
  }
}

/* ============================================================================
 * axios 实例
 * ========================================================================== */

const instance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 15000,
})

instance.interceptors.request.use(async (config) => {
  let next = config
  for (const fn of requestInterceptors) {
    next = await fn(next)
  }
  return next
})

instance.interceptors.response.use(
  async (response) => {
    let next = response
    for (const fn of responseInterceptors) {
      next = await fn(next)
    }
    return next
  },
  async (error: AxiosError) => {
    let current: ResponseErrorInterceptorResult = error
    for (const fn of responseErrorInterceptors) {
      const result = await fn(current as AxiosError)
      // 返回 AxiosResponse 代表该拦截器已经「恢复」了这次失败（如重发成功）
      if (result && (result as AxiosResponse).config) {
        return result
      }
      current = result
    }
    return Promise.reject(current)
  },
)

/* ============================================================================
 * 内置拦截器
 * ========================================================================== */

/** 内置请求拦截器 1：注入 Bearer Token */
registerRequestInterceptor((config) => {
  const token = getAccessToken()
  if (token) {
    config.headers.set('Authorization', `Bearer ${token}`)
  }
  return config
})

/** 内置请求拦截器 2：注入 traceId（示例，便于后期接日志/埋点系统） */
registerRequestInterceptor((config) => {
  if (!config.headers.get('X-Trace-Id')) {
    config.headers.set('X-Trace-Id', `${Date.now()}-${Math.random().toString(16).slice(2, 10)}`)
  }
  return config
})

/** 内置响应拦截器：解包统一响应体 { code, message, data } */
registerResponseInterceptor((response) => {
  const body = response.data as ApiResponse | undefined
  if (body && typeof body === 'object' && 'code' in body) {
    if (body.code !== 0) {
      const err = new ApiError(body.message || '请求失败', body.code, response.status)
      notifyError(err.message)
      throw err
    }
    response.data = body.data
  }
  return response
})

/* ------------------------------ token 刷新 ------------------------------ */

let refreshPromise: Promise<string> | null = null

async function refreshAccessToken(): Promise<string> {
  const refreshToken = getRefreshToken()
  if (!refreshToken) {
    throw new ApiError('登录已过期，请重新登录', -1, 401)
  }
  // 直接使用原生 axios，避免走本实例的拦截器造成递归
  const resp = await axios.post<ApiResponse<LoginResult>>(
    `${API_BASE_URL}/auth/refresh`,
    { refresh_token: refreshToken },
    { timeout: 15000 },
  )
  const body = resp.data
  if (body.code !== 0 || !body.data) {
    throw new ApiError(body.message || '登录已过期，请重新登录', body.code, 401)
  }
  setTokens(body.data.access_token, body.data.refresh_token)
  return body.data.access_token
}

/** 未授权时的兜底处理，可在 main.tsx 中用 setUnauthorizedHandler 覆盖为「清 store + 跳登录」 */
let unauthorizedHandler: () => void = () => {
  clearTokens()
  if (window.location.pathname !== '/login') {
    window.location.replace('/login')
  }
}

export function setUnauthorizedHandler(handler: () => void): void {
  unauthorizedHandler = handler
}

function extractErrorMessage(error: AxiosError): string {
  const body = error.response?.data as ApiResponse | undefined
  if (body && typeof body === 'object' && 'message' in body && body.message) {
    return body.message
  }
  if (error.response) {
    return `请求失败（HTTP ${error.response.status}）`
  }
  if (error.code === 'ECONNABORTED') {
    return '请求超时，请稍后重试'
  }
  return error.message || '网络异常，请稍后重试'
}

/** 内置错误拦截器：401 静默刷新 token 并重发；403 / 其他统一提示 */
registerResponseErrorInterceptor(async (error) => {
  const status = error.response?.status
  const original = error.config as (InternalAxiosRequestConfig & { _retry?: boolean }) | undefined

  if (status === 401 && original && !original._retry && !original.url?.includes('/auth/refresh')) {
    original._retry = true
    try {
      // 单飞：并发的 401 共用同一个刷新 Promise
      if (!refreshPromise) {
        refreshPromise = refreshAccessToken().finally(() => {
          refreshPromise = null
        })
      }
      const newToken = await refreshPromise
      original.headers.set('Authorization', `Bearer ${newToken}`)
      // 重发原请求，成功则作为「已恢复」返回
      return instance.request(original)
    } catch (refreshError) {
      unauthorizedHandler()
      return refreshError as AxiosError
    }
  }

  if (status === 401) {
    unauthorizedHandler()
    return error
  }

  if (status === 403) {
    notifyError('无权限访问该资源')
    return error
  }

  notifyError(extractErrorMessage(error))
  return error
})

/* ============================================================================
 * 对外请求封装：返回解包后的 data
 * ========================================================================== */

export async function request<T = unknown>(config: AxiosRequestConfig): Promise<T> {
  const response = await instance.request<unknown, AxiosResponse<T>>(config)
  return response.data
}

export const http = {
  get<T = unknown>(url: string, params?: unknown, config?: AxiosRequestConfig) {
    return request<T>({ method: 'GET', url, params, ...config })
  },
  post<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return request<T>({ method: 'POST', url, data, ...config })
  },
  put<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return request<T>({ method: 'PUT', url, data, ...config })
  },
  delete<T = unknown>(url: string, config?: AxiosRequestConfig) {
    return request<T>({ method: 'DELETE', url, ...config })
  },
}

export default instance
