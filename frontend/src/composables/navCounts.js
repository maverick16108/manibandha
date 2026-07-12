import { reactive } from 'vue'
import client from '../api/client'

// Счётчики для меню: непросмотренные вопросы/отчёты (общие) и неодобренные заявки.
export const navCounts = reactive({ questions: 0, reports: 0, approvals: 0 })

export async function refreshNavCounts() {
  try {
    const { data } = await client.get('/threads/nav-counts')
    navCounts.questions = data.questions || 0
    navCounts.reports = data.reports || 0
    navCounts.approvals = data.approvals || 0
  } catch { /* не залогинен / нет прав — игнор */ }
}
