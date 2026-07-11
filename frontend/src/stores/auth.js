import { defineStore } from 'pinia'
import client from '../api/client'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || null,
    user: null,
    caps: [],
    roles: [],
  }),
  getters: {
    isAuthenticated: (s) => !!s.token,
    role: (s) => s.user?.role || null,
    // право-действие
    can: (s) => (cap) => s.caps.includes(cap),
    // производные (для совместимости с существующими проверками)
    isGuru: (s) => s.roles.includes('guru'),
    isStaff: (s) => s.caps.includes('users.manage'),
    canEdit: (s) => s.caps.includes('disciples.edit'),
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
      try {
        const { data: perm } = await client.get('/me/capabilities')
        this.caps = perm.capabilities || []
        this.roles = perm.roles || []
      } catch {
        this.caps = []
        this.roles = []
      }
      return data
    },
    async updateProfile(payload) {
      const { data } = await client.patch('/auth/me', payload)
      this.user = { ...this.user, ...data }
      return data
    },
    logout() {
      this.token = null
      this.user = null
      this.caps = []
      this.roles = []
      localStorage.removeItem('token')
    },
  },
})
