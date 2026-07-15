import { defineStore } from 'pinia'
import client from '../api/client'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || null,
    user: null,
    caps: [],
    roles: [],
    pending: false,
  }),
  getters: {
    isAuthenticated: (s) => !!s.token,
    isPending: (s) => s.pending,
    role: (s) => s.user?.role || null,
    // право-действие
    can: (s) => (cap) => s.caps.includes(cap),
    // производные (для совместимости с существующими проверками)
    isGuru: (s) => s.roles.includes('guru') || s.user?.role === 'guru',
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
      this.refreshToken() // скользящее продление сессии: каждый заход обновляет срок токена
      try {
        const { data: perm } = await client.get('/me/capabilities')
        this.caps = perm.capabilities || []
        this.roles = perm.roles || []
        this.pending = !!perm.pending
      } catch {
        this.caps = []
        this.roles = []
        this.pending = false
      }
      return data
    },
    async requestPhoneCode(phone, purpose = 'auto') {
      const { data } = await client.post('/auth/phone/request', { phone, purpose })
      return data
    },
    async loginByPhone(phone, code) {
      const { data } = await client.post('/auth/phone/verify', { phone, code })
      this.token = data.access_token
      localStorage.setItem('token', this.token)
      await this.fetchMe()
    },
    async refreshToken() {
      // получить свежий токен на полный срок; молча игнорируем ошибки
      try {
        const { data } = await client.post('/auth/refresh')
        if (data?.access_token) {
          this.token = data.access_token
          localStorage.setItem('token', this.token)
        }
      } catch { /* не критично */ }
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
      this.pending = false
      localStorage.removeItem('token')
    },
  },
})
