// Статусы репозитория для отображения. Правило: «нет данных» никогда не
// превращается в 0 — число показывается только при Availability = ok.

import { Availability, type BranchStatus, type CISummary, type PRSummary, type Repository } from '../data/model'
import { t } from '../i18n'
import type { Tone } from '../theme/theme'

export interface Label {
  tone: Tone
  text: string
}

/** Статус рабочей копии прочитан (branch и worktree есть). */
export function hasStatus(r: Repository): boolean {
  return r.branch != null && r.worktree != null
}

/** staged + unstaged + untracked; статус не прочитан — 0, проверять hasStatus. */
export function changeCount(r: Repository): number {
  const w = r.worktree
  return w ? w.staged + w.unstaged + w.untracked : 0
}

export function hasChanges(r: Repository): boolean {
  return changeCount(r) + (r.worktree?.conflicts ?? 0) > 0
}

export type SyncState = 'unknown' | 'detached' | 'noUpstream' | 'inSync' | 'ahead' | 'behind' | 'diverged'

export function syncState(b: BranchStatus | null | undefined): SyncState {
  if (!b) return 'unknown'
  if (b.detached) return 'detached'
  if (!b.upstream || b.ahead == null || b.behind == null) return 'noUpstream'
  if (b.ahead > 0 && b.behind > 0) return 'diverged'
  if (b.behind > 0) return 'behind'
  if (b.ahead > 0) return 'ahead'
  return 'inSync'
}

/** Отставание по достоверным данным; без upstream — false, а не «0». */
export function isBehind(r: Repository): boolean {
  const state = syncState(r.branch)
  return state === 'behind' || state === 'diverged'
}

/** Число из серверного источника или null, если данных нет. */
export function remoteCount(s: PRSummary | CISummary, n: number | null | undefined): number | null {
  return s.availability === Availability.AvailabilityOK && n != null ? n : null
}

export function ciFailed(r: Repository): boolean {
  return r.ci.availability === Availability.AvailabilityOK && r.ci.lastResult === 'failure'
}

/**
 * Интеграция не настроена: CLI нет, нет авторизации или источник ответил
 * ошибкой. not_supported — постоянное ограничение сервера, не проблема настройки.
 */
export function integrationIssue(r: Repository): boolean {
  const bad = (a: Availability) => a === Availability.AvailabilityUnavailable || a === Availability.AvailabilityError
  return bad(r.pr.availability) || bad(r.ci.availability)
}

function unknownLabel(availability: Availability): Label {
  if (availability === Availability.AvailabilityNotSupported) return { tone: 'muted', text: t('remote.notSupported') }
  if (availability === Availability.AvailabilityError) return { tone: 'bad', text: t('remote.error') }
  return { tone: 'muted', text: t('remote.unavailable') }
}

const statusUnknown: Label = { tone: 'muted', text: t('status.unknown') }

export function changesLabel(r: Repository): Label {
  const w = r.worktree
  if (!w) return statusUnknown
  if (w.conflicts > 0) return { tone: 'bad', text: t('changes.conflicts', { count: w.conflicts }) }
  const count = changeCount(r)
  if (count > 0) return { tone: 'warn', text: t('changes.count', { count }) }
  return { tone: 'good', text: t('changes.clean') }
}

export function syncLabel(r: Repository): Label {
  const b = r.branch
  switch (syncState(b)) {
    case 'unknown':
      return statusUnknown
    case 'detached':
      return { tone: 'warn', text: t('sync.detached') }
    case 'noUpstream':
      return { tone: 'muted', text: t('sync.noUpstream') }
    case 'diverged':
      return { tone: 'bad', text: t('sync.diverged', { ahead: b!.ahead!, behind: b!.behind! }) }
    case 'behind':
      return { tone: 'warn', text: t('sync.behind', { behind: b!.behind! }) }
    case 'ahead':
      return { tone: 'info', text: t('sync.ahead', { ahead: b!.ahead! }) }
    case 'inSync':
      return { tone: 'good', text: t('sync.inSync') }
  }
}

export function prLabel(r: Repository): Label {
  const open = remoteCount(r.pr, r.pr.open)
  if (open == null) return unknownLabel(r.pr.availability)
  return open > 0 ? { tone: 'info', text: t('pr.open', { count: open }) } : { tone: 'muted', text: t('pr.none') }
}

export function ciLabel(r: Repository): Label {
  const active = remoteCount(r.ci, r.ci.active)
  if (active == null) return unknownLabel(r.ci.availability)
  if (active > 0) return { tone: 'info', text: t('ci.active', { count: active }) }
  if (r.ci.lastResult === 'failure') return { tone: 'bad', text: t('ci.failure') }
  if (r.ci.lastResult === 'success') return { tone: 'good', text: t('ci.success') }
  return { tone: 'muted', text: t('ci.none') }
}
