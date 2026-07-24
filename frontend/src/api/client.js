import axios from 'axios'

// Same-origin: dev proxies /api to backend (vite), prod nginx proxies /api.
const client = axios.create({ baseURL: '/api' })

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  // активное пространство (Ф4b): бэкенд считает права/модули/контент для него.
  // 1 (домашнее, Манибандха) не шлём — резолвится по домену/умолчанию.
  const sp = localStorage.getItem('activeSpaceId')
  if (sp && sp !== '1') config.headers['X-Space-Id'] = sp
  return config
})

client.interceptors.response.use(
  (r) => r,
  (error) => {
    if (error.response && error.response.status === 401) {
      localStorage.removeItem('token')
      if (!window.location.pathname.startsWith('/login')) {
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  },
)

export default client
