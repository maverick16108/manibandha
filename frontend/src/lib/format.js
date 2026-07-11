export const STATUS_LABELS = {
  aspirant: 'Кандидат',
  pranama: 'Пранама-мантра',
  recommended: 'Рекомендован',
  harinama: 'Харинама',
  brahman: 'Брахман',
}

export const STATUS_ORDER = ['aspirant', 'pranama', 'recommended', 'harinama', 'brahman']

export const STATUS_BADGE = {
  aspirant: 'bg-parchment-200 text-ink-700',
  pranama: 'bg-orange-100 text-orange-800',
  recommended: 'bg-amber-100 text-amber-800',
  harinama: 'bg-saffron-500/15 text-saffron-700',
  brahman: 'bg-sage-500/20 text-sage-600',
}

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
  curator: 'Наставник',
  student: 'Ученик',
}

export function formatDate(value) {
  if (!value) return '—'
  const d = new Date(value)
  if (isNaN(d)) return value
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
}
