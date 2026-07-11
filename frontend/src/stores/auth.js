import { defineStore } from 'pinia'
import client from '../api/client'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || null,
    user: null,
  }),
  getters: {
    isAuthenticated: (s) => !!s.token,
    role: (s) => s.user?.role || null,
    isGuru: (s) => s.user?.role === 'guru',
    isStaff: (s) => ['guru', 'secretary'].includes(s.user?.role),
    canEdit: (s) => ['guru', 'secretary', 'curator'].includes(s.user?.role),
  },
  actions: {
    async login(email, password) {
      const body = new URLSearchParams({ username: email, password })
      const { data } = await client.post('/auth/login', body)
      this.token = data.access_token
      localStorage.setItem('token', this.token)
      await this.fetchMe()
    },
    async fetchMe() {
      if (!this.token) return null
      const { data } = await client.get('/auth/me')
      this.user = data
      return data
    },
    logout() {
      this.token = null
      this.user = null
      localStorage.removeItem('token')
    },
  },
})
