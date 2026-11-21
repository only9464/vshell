// 与 4.9.3 一致的接口约定：JWT 放在请求头 Token，路径前缀 /api。
import axios from 'axios'
import { message } from 'ant-design-vue'

export const TOKEN_KEY = 'vshell.token'

export const http = axios.create({ baseURL: '/api', timeout: 30000 })

http.interceptors.request.use((cfg) => {
  const token = localStorage.getItem(TOKEN_KEY)
  if (token) cfg.headers.Token = token
  return cfg
})

http.interceptors.response.use(
  (resp) => {
    const data = resp.data
    if (data && typeof data === 'object' && 'code' in data && data.code !== 0) {
      message.error(data.message || '请求失败')
      return Promise.reject(new Error(data.message || 'request failed'))
    }
    return data
  },
  (err) => {
    if (err?.response?.status === 401) {
      localStorage.removeItem(TOKEN_KEY)
      if (!location.hash.startsWith('#/login')) location.hash = '#/login'
    }
    message.error(err?.response?.data?.message || err.message || '网络错误')
    return Promise.reject(err)
  },
)

export const get = <T = any>(url: string, params?: any) => http.get<unknown, T>(url, { params })
export const post = <T = any>(url: string, data?: any) => http.post<unknown, T>(url, data)

// 下载直链（浏览器直接导航，带 token）
export function downloadURL(path: string, params: Record<string, string | number>) {
  const q = new URLSearchParams(Object.entries(params).map(([k, v]) => [k, String(v)]))
  const token = localStorage.getItem(TOKEN_KEY) || ''
  if (token) q.set('token', token)
  return `/api${path}?${q.toString()}`
}
