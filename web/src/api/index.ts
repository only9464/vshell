// 各模块 API（严格按 docs/API.md 的端点表）。
import { get, post } from './http'

export { get, post }

export interface LoginResult {
  desc: string
  realName: string
  roles: { roleName: string; value: string }[]
  token: string
  userId: string
  username: string
}

export const authApi = {
  login: (username: string, password: string) =>
    post<LoginResult>('/login', { username, password }),
  logout: () => get('/logout'),
  userInfo: () => get('/getUserInfo'),
}

export interface ClientItem {
  Id: number
  VerifyKey: string
  Tp: string
  Addr: string
  LocalIP: string
  UserName: string
  HostName: string
  OsName: string
  ProcessName: string
  Arch: string
  Version: string
  Location: string
  IsConnect: boolean
  PingCheckTime: number
  Remark: string
  Port: number
  DnsPort: number
}

export const clientApi = {
  list: (page = 1, pageSize = 50, search = '') =>
    post<{ items: ClientItem[]; total: number; clientCount: number; clientOnlineCount: number }>(
      '/client/list',
      { page, pageSize, search },
    ),
  editRemark: (id: number, remark: string) => post('/client/editremark', { id, remark }),
  delList: (ids: number[]) => post('/client/dellist', { id: ids }),
  delFile: (id: number, paths: string[]) =>
    post('/client/delfile', { id, paths: paths.join('|<-') }),
  delProcess: (id: number, pid: number, name: string) =>
    post('/client/delprocess', { id, pid, name }),
}

export interface ListenerItem {
  Id: number
  Mode: string
  Status: boolean
  OssUrl: string
  Remark: string
  Vkey: string
  ListenAddr: string
  ConnectAddr: string
  EncryptSalt: string
  PingInterval: number
  DisconnectTimeout: number
}

export const listenerApi = {
  list: (page = 1, pageSize = 50) =>
    post<{ items: ListenerItem[] | null; total: number }>('/listener/list', { page, pageSize }),
  add: (data: Partial<ListenerItem> & { Mode: string; Vkey: string }) =>
    post('/listener/add', data),
  start: (id: number) => post('/listener/start', { id }),
  stop: (id: number) => post('/listener/stop', { id }),
  del: (id: number) => post('/listener/del', { id }),
  editRemark: (id: number, remark: string) => post('/listener/editremark', { id, remark }),
}

export const fileApi = {
  ls: (id: number, path = '') => post<{ items: any[]; total: number }>('/file/ls', { id, path }),
  cat: (id: number, path: string) => post('/file/cat', { id, path }),
  rm: (id: number, path: string) => post('/file/rm', { id, path }),
  mkdir: (id: number, path: string, target: string) =>
    post('/file/mkdir', { id, path, target }),
  edit: (id: number, path: string, target: string, content: string) =>
    post('/file/edit', { id, path, target, content }),
  wget: (id: number, path: string, url: string) => post('/file/wget', { id, path, target: url }),
  // 移动/重命名：后端约定 path 里用 "|<-" 分隔源与目标
  mv: (id: number, src: string, dst: string) =>
    post('/file/mv', { id, path: `${src}|<-${dst}` }),
  downloadToOSS: (id: number, path: string, target: string) =>
    post('/file/downloadtooss', { id, path, target }),
  upload: (id: number, path: string, file: File) => {
    const fd = new FormData()
    fd.append('id', String(id))
    fd.append('path', path)
    fd.append('file', file)
    return post('/file/upload', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
  },
}

export const downloadApi = {
  stage: (id: number, arch: string) => `/api/download/stage?id=${id}&arch=${arch}`,
  stageless: (id: number, arch: string) => `/api/download/stageless?id=${id}&arch=${arch}`,
  listen: (q: Record<string, string | number>) => q,
}

export const runnerApi = {
  list: () => get<{ id: number; name: string }[]>('/runner/list'),
  run: (id: number, pluginName: string, procArg: string) =>
    post<{ output: string }>('/runner/runplugin', { id, pluginName, procArg }),
}

export const dashboardApi = {
  info: () => get<Record<string, any>>('/dashboard/info'),
}

export const settingApi = {
  get: () => get<Record<string, string>>('/setting/get'),
  edit: (data: Record<string, string>) => post('/setting/edit', data),
}
