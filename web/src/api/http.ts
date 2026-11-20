// 统一 HTTP 封装：与 docs/API.md §0 的信封约定一致。
// - 请求头带 Token
// - code !== 0 视为业务错误
// - 401 清凭据并回登录页
import axios, { type AxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'

export interface Envelope<T = unknown> {
  code: number
  message?: string
  type?: string
  result?: T
}

const http = axios.create({ baseURL: '/api', timeout: 60_000 })

http.interceptors.request.use((cfg) => {
  const token = localStorage.getItem('vshell.token')
  if (token) cfg.headers.Token = token
  return cfg
})

http.interceptors.response.use(
  // 拦截器把信封拆成 result 直接返回，因此这里放宽类型（axios 的类型签名要求
  // 返回 AxiosResponse，运行时并不检查）。
  ((resp: any) => {
    const data = resp.data as Envelope
    if (data && typeof data.code === 'number' && data.code !== 0) {
      const msg = data.message || '请求失败'
      ElMessage.error(msg)
      return Promise.reject(new Error(msg))
    }
    return data?.result ?? data
  }) as any,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('vshell.token')
      if (!location.hash.includes('/login')) location.hash = '#/login'
    } else {
      ElMessage.error(err.message || '网络错误')
    }
    return Promise.reject(err)
  },
)

export function post<T = any>(url: string, data: unknown = {}, config?: AxiosRequestConfig) {
  return http.post<unknown, T>(url, data, config)
}
export function get<T = any>(url: string, config?: AxiosRequestConfig) {
  return http.get<unknown, T>(url, config)
}

// downloadURL 拼出带 token 的下载地址（浏览器直接导航下载，见 API.md §6）。
export function downloadURL(path: string, params: Record<string, string | number | undefined>) {
  const search = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') search.set(k, String(v))
  }
  const token = localStorage.getItem('vshell.token')
  if (token) search.set('token', token)
  return `/api${path}?${search.toString()}`
}

export default http
