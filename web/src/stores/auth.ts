import { defineStore } from 'pinia'
import { authApi } from '../api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('vshell.token') || '',
    username: localStorage.getItem('vshell.username') || '',
  }),
  getters: {
    loggedIn: (s) => !!s.token,
  },
  actions: {
    async login(username: string, password: string) {
      const res = await authApi.login(username, password)
      this.token = res.token
      this.username = res.username
      localStorage.setItem('vshell.token', res.token)
      localStorage.setItem('vshell.username', res.username)
      return res
    },
    logout() {
      this.token = ''
      this.username = ''
      localStorage.removeItem('vshell.token')
      localStorage.removeItem('vshell.username')
    },
  },
})
