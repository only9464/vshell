import { defineStore } from 'pinia'
import { post, TOKEN_KEY } from '../api/http'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem(TOKEN_KEY) || '',
    username: localStorage.getItem('vshell.username') || '',
  }),
  actions: {
    async login(username: string, password: string) {
      const res: any = await post('/login', { username, password })
      const token = res?.result?.token || res?.token
      if (!token) throw new Error('登录失败')
      this.token = token
      this.username = res?.result?.username || username
      localStorage.setItem(TOKEN_KEY, token)
      localStorage.setItem('vshell.username', this.username)
    },
    logout() {
      this.token = ''
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem('vshell.username')
    },
  },
})
