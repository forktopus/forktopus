// Группировка радара по idea/plan/05-UI-AND-ACCEPTANCE.md §3. В отличие от
// JS макета, недоступный CLI и упавший CI не делают репозиторий «требующим
// внимания»: это состояние сервера, а не рабочего дерева. Репозиторий, статус
// которого ещё не прочитан, не попадает ни в одну группу: «в порядке» про
// него сказать нельзя.

import { Availability, type Repository } from '../data/model'
import { formatRelative, t } from '../i18n'
import {
  changeCount,
  changesDetail,
  ciFailed,
  ciLabel,
  hasChanges,
  hasStatus,
  integrationIssue,
  isBehind,
  prLabel,
  remoteCount,
  syncState,
  type Label,
} from './status'

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

/**
 * Метки карточки радара: почему репозиторий в своей группе, плюс серверные
 * факты (CI, PR, незавершённая настройка). Серверные метки группу не меняют.
 */
export function radarTags(r: Repository): Label[] {
  const tags: Label[] = []
  const w = r.worktree
  const b = r.branch
  if (r.error) tags.push({ tone: 'bad', text: t('radar.tag.readError') })
  if (w?.conflicts) tags.push({ tone: 'bad', text: t('changes.conflicts', { count: w.conflicts }) })
  if (w?.operation) tags.push({ tone: 'bad', text: t('radar.tag.operation', { operation: w.operation }) })
  const local = changeCount(r)
  if (local > 0) tags.push({ tone: 'warn', text: t('radar.tag.local', { count: local }) })
  switch (syncState(b)) {
    case 'diverged':
      tags.push({ tone: 'bad', text: t('radar.tag.diverged') })
      break
    case 'behind':
      tags.push({ tone: 'warn', text: t('radar.tag.behind', { count: b!.behind! }) })
      break
    case 'ahead':
      tags.push({ tone: 'info', text: t('radar.tag.ahead', { count: b!.ahead! }) })
      break
    case 'gone':
      tags.push({ tone: 'warn', text: t('sync.gone') })
      break
    case 'detached':
      tags.push({ tone: 'warn', text: t('sync.detached') })
      break
    case 'unborn':
      tags.push({ tone: 'muted', text: t('sync.unborn') })
      break
  }
  const active = remoteCount(r.ci, r.ci.active)
  if (active) tags.push({ tone: 'info', text: t('radar.tag.ciActive', { count: active }) })
  else if (ciFailed(r)) tags.push({ tone: 'bad', text: t('radar.tag.ciFailed') })
  if (integrationIssue(r)) {
    const bad = (a: Availability) => a === Availability.AvailabilityUnavailable || a === Availability.AvailabilityError
    tags.push({ tone: 'warn', text: `⚠ ${(bad(r.pr.availability) ? prLabel(r) : ciLabel(r)).text}` })
  }
  const prs = remoteCount(r.pr, r.pr.open)
  if (prs) tags.push({ tone: 'info', text: t('radar.tag.prs', { count: prs }) })
  if (tags.length === 0 && radarGroup(r) === 'ok') tags.push({ tone: 'good', text: t('radar.tag.ok') })
  return tags
}

/** Строка внизу карточки: главное объяснение состояния. */
export function radarNote(r: Repository, now: Date = new Date()): string {
  if (r.error) return r.error.message
  if (hasChanges(r) || r.worktree?.operation) return changesDetail(r)
  const b = r.branch
  if (!b) return t('status.unknown')
  const fetched = b.lastFetchAt ? t('sync.fetchedAt', { time: formatRelative(b.lastFetchAt, now) }) : t('sync.neverFetched')
  switch (syncState(b)) {
    case 'behind':
    case 'diverged':
      return `${t('radar.note.behind', { count: b.behind! })} · ${fetched}`
    case 'inSync':
    case 'ahead':
      return `${t('radar.note.synced')} · ${fetched}`
    case 'noUpstream':
      return t('radar.note.noUpstream')
    case 'detached':
      return t('radar.note.detached', { sha: (b.headSha ?? '').slice(0, 8) })
    case 'gone':
      return t('radar.note.gone')
  }
  return ''
}
