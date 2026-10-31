import type { Repository } from '../data/model'
import { radarGroup } from './radar'
import { repoHost, repoName, repoNamespace } from './repo'
import { changeCount, ciFailed, hasChanges, hasStatus, integrationIssue, isBehind, remoteCount } from './status'

export type RegistryFilter = 'all' | 'attention' | 'dirty' | 'behind' | 'ci'
export const registryFilters: RegistryFilter[] = ['all', 'attention', 'dirty', 'behind', 'ci']

export type SortKey = 'name' | 'branch' | 'changes' | 'sync' | 'pr' | 'ci' | 'host'
export const sortKeys: SortKey[] = ['name', 'branch', 'changes', 'sync', 'pr', 'ci', 'host']

export interface RegistryQuery {
  filter: RegistryFilter
  /** host[:port]; пусто — все хосты */
  host: string
  query: string
}

export function matchesQuery(r: Repository, query: string): boolean {
  const q = query.trim().toLocaleLowerCase('ru')
  if (!q) return true
  return [repoName(r), repoNamespace(r), r.branch?.name ?? '', repoHost(r), r.path]
    .join(' ')
    .toLocaleLowerCase('ru')
    .includes(q)
}

export function matchesFilter(r: Repository, filter: RegistryFilter): boolean {
  switch (filter) {
    case 'all':
      return true
    case 'attention':
      return radarGroup(r) === 'attention'
    case 'dirty':
      return hasChanges(r)
    case 'behind':
      return isBehind(r)
    case 'ci':
      return ciFailed(r)
  }
}

export function filterRepos(repos: Repository[], q: RegistryQuery): Repository[] {
  return repos.filter(
    (r) => (!q.host || repoHost(r) === q.host) && matchesFilter(r, q.filter) && matchesQuery(r, q.query),
  )
}

// Нет данных сортируется как -1: ниже любого достоверного значения.
function sortValue(r: Repository, key: SortKey): string | number {
  switch (key) {
    case 'name':
      return `${repoNamespace(r)}/${repoName(r)}`
    case 'branch':
      return r.branch?.name ?? ''
    case 'changes':
      return r.worktree ? changeCount(r) + r.worktree.conflicts : -1
    case 'sync':
      return r.branch?.behind ?? -1
    case 'pr':
      return remoteCount(r.pr, r.pr.open) ?? -1
    case 'ci':
      return remoteCount(r.ci, r.ci.active) ?? -1
    case 'host':
      return repoHost(r)
  }
}

export function sortRepos(repos: Repository[], key: SortKey, asc: boolean): Repository[] {
  const dir = asc ? 1 : -1
  return [...repos].sort((a, b) => {
    const x = sortValue(a, key)
    const y = sortValue(b, key)
    const cmp = typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y), 'ru')
    return cmp * dir || a.path.localeCompare(b.path, 'ru')
  })
}

export interface Kpis {
  shown: number
  total: number
  withChanges: number
  behind: number
  openPRs: number
  activeCI: number
  /** Сколько репозиториев без прочитанного git-статуса: их изменения и отставание не учтены. */
  noStatus: number
  /** Часть репозиториев без данных PR/CI: суммы неполные. */
  partial: boolean
  /** Сколько репозиториев с данными PR или CI; 0 — суммы неизвестны, а не нулевые. */
  remoteKnown: number
}

export function kpis(shown: Repository[], total: number): Kpis {
  let openPRs = 0
  let activeCI = 0
  let partial = false
  let remoteKnown = 0
  for (const r of shown) {
    const pr = remoteCount(r.pr, r.pr.open)
    const ci = remoteCount(r.ci, r.ci.active)
    openPRs += pr ?? 0
    activeCI += ci ?? 0
    if (pr == null || ci == null) partial = true
    if (pr != null || ci != null) remoteKnown++
  }
  return {
    shown: shown.length,
    total,
    withChanges: shown.filter(hasChanges).length,
    behind: shown.filter(isBehind).length,
    openPRs,
    activeCI,
    noStatus: shown.filter((r) => !hasStatus(r)).length,
    partial,
    remoteKnown,
  }
}

/** Хосты для фильтра: уникальные, по алфавиту. */
export function hostsOf(repos: Repository[]): string[] {
  return [...new Set(repos.map(repoHost).filter(Boolean))].sort((a, b) => a.localeCompare(b))
}

export function integrationIssueCount(repos: Repository[]): number {
  return repos.filter(integrationIssue).length
}

/** Источник в фильтре реестра: хост, число репозиториев и незавершённая настройка интеграции. */
export interface HostSource {
  host: string
  count: number
  /** у части репозиториев хоста нет данных PR/CI (нет CLI, авторизации, связи) */
  issue: boolean
}

export function hostSources(repos: Repository[]): HostSource[] {
  const byHost = new Map<string, HostSource>()
  for (const r of repos) {
    const host = repoHost(r)
    if (!host) continue
    const s = byHost.get(host) ?? { host, count: 0, issue: false }
    s.count++
    s.issue ||= integrationIssue(r)
    byHost.set(host, s)
  }
  return [...byHost.values()].sort((a, b) => a.host.localeCompare(b.host))
}

/** Строка реестра: вложенные репозитории (D-025) идут сразу под родителем. */
export interface RegistryRow {
  repo: Repository
  depth: number
  /** сколько вложенных строк под этой (для кнопки свернуть/развернуть) */
  children: number
}

/**
 * Раскладывает отсортированный список в дерево по parentId, сохраняя порядок
 * среди соседей. Вложенный, чей родитель не попал в выборку, — на верхнем
 * уровне. Потомки свёрнутых строк пропускаются.
 */
export function registryRows(list: Repository[], collapsed: ReadonlySet<string> = new Set()): RegistryRow[] {
  const shown = new Set(list.map((r) => r.id))
  const kids = new Map<string, Repository[]>()
  const top: Repository[] = []
  for (const r of list) {
    if (r.parentId && shown.has(r.parentId)) kids.set(r.parentId, [...(kids.get(r.parentId) ?? []), r])
    else top.push(r)
  }
  const rows: RegistryRow[] = []
  const walk = (r: Repository, depth: number) => {
    const children = kids.get(r.id) ?? []
    rows.push({ repo: r, depth, children: children.length })
    if (!collapsed.has(r.id)) children.forEach((c) => walk(c, depth + 1))
  }
  top.forEach((r) => walk(r, 0))
  return rows
}
