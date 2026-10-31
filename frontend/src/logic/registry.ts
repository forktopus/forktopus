import type { Repository } from '../data/model'
import { radarGroup } from './radar'
import { repoHost, repoName, repoNamespace } from './repo'
import { changeCount, ciFailed, hasChanges, integrationIssue, isBehind, remoteCount } from './status'

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
  return [repoName(r), repoNamespace(r), r.branch.name ?? '', repoHost(r), r.path]
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
      return r.branch.name ?? ''
    case 'changes':
      return changeCount(r) + r.worktree.conflicts
    case 'sync':
      return r.branch.behind ?? -1
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
  /** Часть репозиториев без данных PR/CI: суммы неполные. */
  partial: boolean
}

export function kpis(shown: Repository[], total: number): Kpis {
  let openPRs = 0
  let activeCI = 0
  let partial = false
  for (const r of shown) {
    const pr = remoteCount(r.pr, r.pr.open)
    const ci = remoteCount(r.ci, r.ci.active)
    openPRs += pr ?? 0
    activeCI += ci ?? 0
    if (pr == null || ci == null) partial = true
  }
  return {
    shown: shown.length,
    total,
    withChanges: shown.filter(hasChanges).length,
    behind: shown.filter(isBehind).length,
    openPRs,
    activeCI,
    partial,
  }
}

/** Хосты для фильтра: уникальные, по алфавиту. */
export function hostsOf(repos: Repository[]): string[] {
  return [...new Set(repos.map(repoHost).filter(Boolean))].sort((a, b) => a.localeCompare(b))
}

export function integrationIssueCount(repos: Repository[]): number {
  return repos.filter(integrationIssue).length
}
