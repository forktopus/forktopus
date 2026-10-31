// Группировка радара по idea/plan/05-UI-AND-ACCEPTANCE.md §3. В отличие от
// JS макета, недоступный CLI и упавший CI не делают репозиторий «требующим
// внимания»: это состояние сервера, а не рабочего дерева.

import type { Repository } from '../data/model'
import { hasChanges, isBehind } from './status'

export type RadarGroup = 'attention' | 'progress' | 'ok'

export const radarGroups: RadarGroup[] = ['attention', 'progress', 'ok']

export function radarGroup(r: Repository): RadarGroup {
  if (r.error || r.worktree.conflicts > 0 || r.worktree.operation) return 'attention'
  if (hasChanges(r) || isBehind(r)) return 'progress'
  return 'ok'
}

export function groupRadar(repos: Repository[]): Record<RadarGroup, Repository[]> {
  const groups: Record<RadarGroup, Repository[]> = { attention: [], progress: [], ok: [] }
  for (const r of repos) groups[radarGroup(r)].push(r)
  return groups
}
