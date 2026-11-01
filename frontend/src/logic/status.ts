// Статусы репозитория для отображения. Правило: «нет данных» никогда не
// превращается в 0 — число показывается только при Availability = ok.

import { Availability, ErrorCode, type BranchStatus, type ChangedFile, type CISummary, type PRSummary, type Repository } from '../data/model'
import { formatRelative, t, type MessageKey } from '../i18n'
import type { Tone } from '../theme/theme'
import { repoHost, selectedRemote } from './repo'

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

export type SyncState = 'unknown' | 'detached' | 'unborn' | 'gone' | 'noUpstream' | 'inSync' | 'ahead' | 'behind' | 'diverged'

export function syncState(b: BranchStatus | null | undefined): SyncState {
  if (!b) return 'unknown'
  if (b.detached) return 'detached'
  if (b.unborn) return 'unborn'
  if (b.upstreamGone) return 'gone'
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

// Причина отсутствия данных по коду ошибки: «CLI не установлен» и «нужна
// авторизация» требуют разных действий пользователя (A16, A17).
const reasonKeys: Partial<Record<ErrorCode, MessageKey>> = {
  [ErrorCode.CodeCLIMissing]: 'remote.cliMissing',
  [ErrorCode.CodeAuthRequired]: 'remote.authRequired',
  [ErrorCode.CodeNetworkError]: 'remote.offline',
  [ErrorCode.CodeTimeout]: 'remote.offline',
  [ErrorCode.CodeRateLimit]: 'remote.rateLimit',
  [ErrorCode.CodeRepoUnresolved]: 'remote.notFound',
  [ErrorCode.CodeNoRemote]: 'remote.noRemote',
  [ErrorCode.CodeInvalidInput]: 'remote.noBranch',
}

function unknownLabel(s: PRSummary | CISummary): Label {
  const key = s.error ? reasonKeys[s.error.code] : undefined
  switch (s.availability) {
    case Availability.AvailabilityNotSupported:
      return { tone: 'muted', text: t(key ?? 'remote.notSupported') }
    case Availability.AvailabilityError:
      return { tone: 'bad', text: t(key ?? 'remote.error') }
    case Availability.AvailabilityUnavailable:
      return { tone: 'warn', text: t(key ?? 'remote.unavailable') }
  }
  return { tone: 'muted', text: t('remote.pending') }
}

/**
 * Пояснение к PR/CI: причина ошибки и возраст данных. Число не показывается
 * при ошибке, поэтому важно, когда был последний успешный ответ (A24).
 */
export function remoteDetail(s: PRSummary | CISummary, now: Date = new Date()): string {
  const parts: string[] = []
  if (s.availability !== Availability.AvailabilityOK) {
    if (s.error?.message) parts.push(s.error.message)
    if (s.lastSuccessAt) parts.push(t('remote.lastSuccess', { time: formatRelative(s.lastSuccessAt, now) }))
  } else if (s.lastAttemptAt) {
    parts.push(t('remote.checkedAt', { time: formatRelative(s.lastAttemptAt, now) }))
  }
  return parts.join(' · ')
}

/** Почему у репозитория нет сервера: remote нет совсем или это локальный путь; есть хост — пусто. */
export function remoteCaption(r: Repository): string {
  if (repoHost(r)) return ''
  return selectedRemote(r) ? t('repo.localRemote') : t('repo.noRemote')
}

const statusUnknown: Label = { tone: 'muted', text: t('status.unknown') }

/** Детализация изменений: staged / unstaged / untracked отдельно (A07) и незавершённая операция. */
export function changesDetail(r: Repository): string {
  const w = r.worktree
  if (!w) return ''
  const parts = [
    w.staged > 0 && t('changes.staged', { count: w.staged }),
    w.unstaged > 0 && t('changes.unstaged', { count: w.unstaged }),
    w.untracked > 0 && t('changes.untracked', { count: w.untracked }),
    w.conflicts > 0 && t('changes.conflicts', { count: w.conflicts }),
    w.operation && t('changes.operation', { operation: w.operation }),
  ]
  return parts.filter(Boolean).join(' · ')
}

/**
 * Свежесть ahead/behind: они посчитаны по remote-tracking refs последнего
 * fetch, а не по серверу сейчас (A09). Без upstream — пусто.
 */
export function syncFreshness(r: Repository, now: Date = new Date()): string {
  const b = r.branch
  if (!b?.upstream || b.detached) return ''
  return b.lastFetchAt ? t('sync.fetchedAt', { time: formatRelative(b.lastFetchAt, now) }) : t('sync.neverFetched')
}

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
    case 'unborn':
      return { tone: 'muted', text: t('sync.unborn') }
    case 'gone':
      return { tone: 'warn', text: t('sync.gone') }
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
  if (open == null) return unknownLabel(r.pr)
  if (open === 0) return { tone: 'muted', text: t('pr.none') }
  const draft = r.pr.draft ?? 0
  const text = t('pr.open', { count: open })
  return { tone: 'info', text: draft > 0 ? `${text} · ${t('pr.draft', { count: draft })}` : text }
}

export function ciLabel(r: Repository): Label {
  const active = remoteCount(r.ci, r.ci.active)
  if (active == null) return unknownLabel(r.ci)
  if (active > 0) return { tone: 'info', text: t('ci.active', { count: active }) }
  if (r.ci.lastResult === 'failure') return { tone: 'bad', text: t('ci.failure') }
  if (r.ci.lastResult === 'success') return { tone: 'good', text: t('ci.success') }
  return { tone: 'muted', text: t('ci.none') }
}

/**
 * Подпись изменённого файла по коду XY из `git status --short`. Цвет — как у
 * счётчиков Проводника: только в индексе — info, есть правки вне индекса или
 * новый файл — warn, конфликт — bad.
 */
export function fileLabel(f: ChangedFile): Label {
  const [x, y] = f.code
  if (f.code === '??') return { tone: 'warn', text: t('file.untracked') }
  if (x === 'U' || y === 'U' || f.code === 'AA' || f.code === 'DD') return { tone: 'bad', text: t('file.conflict') }
  const tone: Tone = y === ' ' ? 'info' : 'warn'
  if (x === 'R') return { tone, text: t('file.renamed') }
  if (x === 'C') return { tone, text: t('file.copied') }
  if (x === 'A') return { tone, text: t('file.added') }
  if (x === 'D' || y === 'D') return { tone, text: t('file.deleted') }
  return { tone, text: t('file.modified') }
}
