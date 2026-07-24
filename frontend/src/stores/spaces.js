import { defineStore } from 'pinia'
import client from '../api/client'

// Пространства (svistok.io-платформа). Каталог, создание, вступление/выход.
// Активное пространство пока = домашнее (Манибандха); переключение контента появится в Ф4b.
export const useSpacesStore = defineStore('spaces', {
  state: () => ({
    list: [],
    loading: false,
    loaded: false,
    activeId: Number(localStorage.getItem('activeSpaceId')) || 1, // 1 = домашнее (Манибандха)
  }),
  getters: {
    mine: (s) => s.list.filter((x) => x.my_status === 'active'),
    joinable: (s) => s.list.filter((x) => x.my_status !== 'active'),
    isHome: (s) => s.activeId === 1,
    active: (s) => s.list.find((x) => x.id === s.activeId) || null,
  },
  actions: {
    async load(force = false) {
      if (this.loaded && !force) return this.list
      this.loading = true
      try {
        const { data } = await client.get('/spaces')
        this.list = data || []
        this.loaded = true
      } finally {
        this.loading = false
      }
      return this.list
    },
    async create(payload) {
      const { data } = await client.post('/spaces', payload)
      await this.load(true)
      return data
    },
    async join(id) {
      const { data } = await client.post(`/spaces/${id}/join`)
      await this.load(true)
      return data
    },
    async leave(id) {
      await client.delete(`/spaces/${id}/join`)
      await this.load(true)
    },
    // Вход в пространство: меняем активное и полностью перезагружаем приложение —
    // права, модули, кэши разделов и keep-alive нужно пересобрать под новый контекст.
    enter(id) {
      if (id === 1) localStorage.removeItem('activeSpaceId')
      else localStorage.setItem('activeSpaceId', String(id))
      this.activeId = id
      window.location.assign('/app')
    },
    exitToHome() { this.enter(1) },
  },
})
