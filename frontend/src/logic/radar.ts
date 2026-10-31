// Группировка радара по idea/plan/05-UI-AND-ACCEPTANCE.md §3. В отличие от
// JS макета, недоступный CLI и упавший CI не делают репозиторий «требующим
// внимания»: это состояние сервера, а не рабочего дерева. Репозиторий, статус
// которого ещё не прочитан, не попадает ни в одну группу: «в порядке» про
// него сказать нельзя.

import type { Repository } from '../data/model'
import { hasChanges, hasStatus, isBehind } from './status'

export type RadarGroup = 'attention' | 'progress' | 'ok'

export const radarGroups: RadarGroup[] = ['attention', 'progress', 'ok']

/** Группа радара; null — статус не прочитан. */
export function radarGroup(r: Repository): RadarGroup | null {
  if (r.error || (r.worktree?.conflicts ?? 0) > 0 || r.worktree?.operation) return 'attention'
  if (!hasStatus(r)) return null
  if (hasChanges(r) || isBehind(r)) return 'progress'
  return 'ok'
}

export function groupRadar(repos: Repository[]): Record<RadarGroup | 'unknown', Repository[]> {
  const groups: Record<RadarGroup | 'unknown', Repository[]> = { attention: [], progress: [], ok: [], unknown: [] }
  for (const r of repos) groups[radarGroup(r) ?? 'unknown'].push(r)
  return groups
}
