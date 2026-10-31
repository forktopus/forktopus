import { describe, expect, it } from 'vitest'
import { repo, unavailable } from '../test/fixtures'
import { filterRepos, hostsOf, integrationIssueCount, kpis, sortRepos } from './registry'
import { repoName, repoNamespace } from './repo'

const alpha = repo({ id: 'alpha', remotePath: 'team/alpha', branch: { name: 'feature/login', behind: 2 } })
const beta = repo({ id: 'beta', host: 'gitlab.com', remotePath: 'group/sub/beta', worktree: { unstaged: 2 }, pr: unavailable, ci: unavailable })
const gamma = repo({ id: 'gamma', remotePath: 'team/gamma', pr: { open: 3 }, ci: { active: 1, lastResult: 'failure' } })
const local = repo({ id: 'local', remotePath: null, path: '/srv/repos/Мои проекты/sandbox', worktree: { conflicts: 1 } })
const all = [alpha, beta, gamma, local]

const ids = (list: { id: string }[]) => list.map((r) => r.id)
const base = { filter: 'all' as const, host: '', query: '' }

describe('имя и namespace', () => {
  it('из remote, а без remote — из пути', () => {
    expect(repoName(beta)).toBe('beta')
    expect(repoNamespace(beta)).toBe('group/sub')
    expect(repoName(local)).toBe('sandbox')
    expect(repoNamespace(local)).toBe('')
  })
})

describe('фильтры реестра', () => {
  it('по статусу', () => {
    expect(ids(filterRepos(all, { ...base, filter: 'attention' }))).toEqual(['local'])
    expect(ids(filterRepos(all, { ...base, filter: 'dirty' }))).toEqual(['beta', 'local'])
    expect(ids(filterRepos(all, { ...base, filter: 'behind' }))).toEqual(['alpha'])
    expect(ids(filterRepos(all, { ...base, filter: 'ci' }))).toEqual(['gamma'])
  })

  it('по хосту', () => {
    expect(ids(filterRepos(all, { ...base, host: 'gitlab.com' }))).toEqual(['beta'])
  })

  it('поиск по имени, ветке, хосту и пути без учёта регистра', () => {
    expect(ids(filterRepos(all, { ...base, query: 'LOGIN' }))).toEqual(['alpha'])
    expect(ids(filterRepos(all, { ...base, query: 'gitlab' }))).toEqual(['beta'])
    expect(ids(filterRepos(all, { ...base, query: 'мои проекты' }))).toEqual(['local'])
    expect(ids(filterRepos(all, { ...base, query: '  ' }))).toHaveLength(4)
  })
})

describe('сортировка', () => {
  it('по имени в обе стороны', () => {
    expect(ids(sortRepos(all, 'name', true))).toEqual(['local', 'beta', 'alpha', 'gamma'])
    expect(ids(sortRepos(all, 'name', false))).toEqual(['gamma', 'alpha', 'beta', 'local'])
  })

  it('нет данных PR — ниже нуля', () => {
    const sorted = ids(sortRepos(all, 'pr', false))
    expect(sorted[0]).toBe('gamma')
    expect(sorted.at(-1)).toBe('beta')
    expect(ids(sortRepos(all, 'pr', true))[0]).toBe('beta')
  })
})

describe('KPI', () => {
  it('считает только достоверные данные и отмечает неполноту', () => {
    expect(kpis(all, 10)).toEqual({
      shown: 4,
      total: 10,
      withChanges: 2,
      behind: 1,
      openPRs: 3,
      activeCI: 1,
      noStatus: 0,
      partial: true,
    })
    expect(kpis([alpha, gamma], 2).partial).toBe(false)
    // Отставание alpha без прочитанного статуса не считается, но и не теряется молча.
    expect(kpis([{ ...alpha, branch: null, worktree: null }, gamma], 2)).toMatchObject({ noStatus: 1, behind: 0 })
  })

  it('хосты и проблемы интеграции', () => {
    expect(hostsOf(all)).toEqual(['github.com', 'gitlab.com'])
    expect(integrationIssueCount(all)).toBe(1)
  })
})
