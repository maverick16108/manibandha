export const STATUS_LABELS = {
  recommended: 'Зарегистрирован',
  aspirant: 'Кандидат',
  pranama: 'Пранама-мантра',
  harinama: 'Харинама',
  brahman: 'Брахман',
}

export const STATUS_ORDER = ['recommended', 'aspirant', 'pranama', 'harinama', 'brahman']

export const STATUS_BADGE = {
  recommended: 'bg-amber-100 text-amber-800',
  aspirant: 'bg-parchment-200 text-ink-700',
  pranama: 'bg-orange-100 text-orange-800',
  harinama: 'bg-saffron-500/15 text-saffron-700',
  brahman: 'bg-sage-500/20 text-sage-600',
}

export const GENDER_LABELS = {
  male: 'Мужской',
  female: 'Женский',
}

// Лёгкое превью загруженного изображения (генерится на бэке: /uploads/<hex>.thumb.webp).
// Для старых загрузок превью может не быть — используйте @error-фолбэк на оригинал.
export function thumbUrl(url) {
  if (!url || typeof url !== 'string') return url
  const m = url.match(/^(\/uploads\/[^.]+)\.(jpe?g|png|webp|gif)$/i)
  return m ? `${m[1]}.thumb.webp` : url
}

// @error-обработчик для превью: если .thumb.webp нет (старая загрузка) — грузим оригинал
export function imgFull(e, full) { const el = e.target; if (el.dataset.f || !full) return; el.dataset.f = '1'; el.src = full }

export const MARITAL_LABELS = {
  single: 'Не женат / не замужем',
  married: 'В браке',
  brahmachari: 'Брахмачари',
  sannyasi: 'Санньяси',
  widowed: 'Вдовец / вдова',
  other: 'Другое',
}

export const ROLE_LABELS = {
  guru: 'Гуру',
  secretary: 'Секретарь',
  curator: 'Куратор',
  student: 'Ученик',
}

// '+79048042771' -> '+7 904 804-27-71'
export function formatPhone(p) {
  const d = (p || '').replace(/\D/g, '')
  if (d.length === 11 && (d[0] === '7' || d[0] === '8')) {
    const n = d.slice(1)
    return `+7 ${n.slice(0, 3)} ${n.slice(3, 6)}-${n.slice(6, 8)}-${n.slice(8, 10)}`
  }
  return p || ''
}

// raw phone string (possibly several numbers) -> [{ tel, display }] for tel: links
export function phoneList(raw) {
  if (!raw) return []
  return raw
    .split(/[,;]+/)
    .map((t) => t.trim())
    .filter(Boolean)
    .map((t) => {
      const d = t.replace(/\D/g, '')
      const tel = d.length === 11 && (d[0] === '8' || d[0] === '7') ? '+7' + d.slice(1)
        : d.length === 10 ? '+7' + d : t
      return { tel, display: formatPhone(tel) }
    })
}

export function formatDate(value) {
  if (!value) return '—'
  const d = new Date(value)
  if (isNaN(d)) return value
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
}
