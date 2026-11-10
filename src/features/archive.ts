import type { LocationQuery } from 'vue-router'
import { ApiError } from '../api'
import type { ArchiveFilters } from '../api/types'
export function archiveFilters(query: LocationQuery): ArchiveFilters {
  const one = (key: string) => (typeof query[key] === 'string' ? (query[key] as string) : '')
  const type = one('type')
  return {
    type: type === 'card' || type === 'project' ? type : '',
    q: one('q'),
    project_id: one('project_id'),
    history: one('history') === 'true',
    from: one('from'),
    to: one('to'),
  }
}
export function archiveError(cause: unknown): string {
  if (cause instanceof ApiError) {
    if (cause.code === 'project_archived') return 'Сначала восстановите проект этой карточки.'
    if (cause.code === 'project_name_conflict')
      return 'Уже существует активный проект с таким именем. Измените его имя и повторите восстановление.'
    if (cause.status === 409) return 'Объект уже изменён. Обновите данные и повторите действие.'
    if (cause.status === 403) return 'Для этого действия нужны права администратора.'
  }
  return 'Не удалось выполнить действие. Попробуйте ещё раз.'
}
