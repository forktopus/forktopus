// Демо-источник по сценариям макета idea/mockups/interactive-prototypes.html
// плюс состояния, которых в макете нет: нет upstream, detached HEAD, конфликт,
// ошибка чтения. Git и CLI не вызываются, данные вымышлены.

import {
  Availability,
  ErrorCode,
  Provider,
  type CISummary,
  type PRSummary,
  type Repository,
  type Root,
  type ToolDiagnostic,
  type UI,
} from './model'
import type { DataSource } from './source'

interface MockRoot {
  root: Root
  host: string
  provider: Provider
  pr: Availability
  ci: Availability
}

const authRequired = { code: ErrorCode.CodeAuthRequired, message: 'glab: нужна авторизация (glab auth login)' }

const mockRoots: MockRoot[] = [
  { root: { id: 'r-github', path: '/srv/repos/github.com' }, host: 'github.com', provider: Provider.ProviderGitHub, pr: Availability.AvailabilityOK, ci: Availability.AvailabilityOK },
  { root: { id: 'r-gitlab', path: '/srv/repos/gitlab.com' }, host: 'gitlab.com', provider: Provider.ProviderGitLab, pr: Availability.AvailabilityUnavailable, ci: Availability.AvailabilityUnavailable },
  { root: { id: 'r-gitea', path: '/srv/repos/git.home.test' }, host: 'git.home.test', provider: Provider.ProviderGitea, pr: Availability.AvailabilityOK, ci: Availability.AvailabilityOK },
  { root: { id: 'r-other', path: '/srv/repos/git.example.org' }, host: 'git.example.org', provider: Provider.ProviderOther, pr: Availability.AvailabilityNotSupported, ci: Availability.AvailabilityNotSupported },
]

interface Scenario {
  root: string
  ns: string
  name: string
  branch?: string
  detached?: boolean
  noUpstream?: boolean
  ahead?: number
  behind?: number
  staged?: number
  unstaged?: number
  untracked?: number
  conflicts?: number
  operation?: string
  prs?: number
  drafts?: number
  ciActive?: number
  ciResult?: string
  ciAvailability?: Availability
  readError?: string
}

const scenarios: Scenario[] = [
  { root: 'r-github', ns: 'schemor', name: 'schemor', branch: 'feature/permissions', staged: 1, unstaged: 3, untracked: 1, ahead: 1, prs: 2, drafts: 1, ciActive: 1, ciResult: 'success' },
  { root: 'r-github', ns: 'ai-development-harness', name: 'ai-development-harness-template', branch: 'main', behind: 3, prs: 1, ciResult: 'success' },
  { root: 'r-github', ns: 'ai-development-harness', name: 'ai-development-harness-client', branch: 'ui/activity-feed', ahead: 2, behind: 1, prs: 1, ciResult: 'failure' },
  { root: 'r-github', ns: 'ai-animation-tools', name: 'music-clip-tools', branch: 'main', ciResult: 'success' },
  { root: 'r-github', ns: 'dev', name: 'dev-scripts', branch: 'main', unstaged: 1 },
  { root: 'r-github', ns: 'forktopus', name: 'forktopus', branch: 'spike/local-only', noUpstream: true, untracked: 2 },
  { root: 'r-github', ns: 'forktopus', name: 'forktopus-docs', branch: 'main', conflicts: 2, unstaged: 1, operation: 'merge', ahead: 1, behind: 4, prs: 0, ciResult: 'success' },
  { root: 'r-gitlab', ns: 'studio/payments', name: 'service-api', branch: 'feat/refund', behind: 2 },
  { root: 'r-gitlab', ns: 'studio', name: 'landing', branch: 'main' },
  { root: 'r-gitea', ns: 'homelab', name: 'infrastructure', branch: 'main', behind: 1, prs: 1, ciActive: 2, ciResult: 'success' },
  { root: 'r-gitea', ns: 'homelab', name: 'services', branch: 'release/1.2', prs: 2, ciResult: 'failure' },
  { root: 'r-gitea', ns: 'homelab', name: 'dotfiles', branch: 'main', prs: 0, ciAvailability: Availability.AvailabilityNotSupported },
  { root: 'r-other', ns: 'tools', name: 'shell-utils', branch: 'main', unstaged: 2 },
  { root: 'r-other', ns: 'tools', name: 'firmware', detached: true },
  { root: 'r-other', ns: 'archive', name: 'legacy-site', readError: 'нет доступа к каталогу: permission denied' },
]

const minutesAgo = (m: number) => new Date(Date.now() - m * 60_000).toISOString()

function idOf(path: string): string {
  let h = 0
  for (const ch of path) h = (Math.imul(h, 31) + ch.charCodeAt(0)) | 0
  return 'm' + (h >>> 0).toString(16).padStart(8, '0')
}

function buildRepository(s: Scenario, index: number): Repository {
  const r = mockRoots.find((m) => m.root.id === s.root)!
  const path = `${r.root.path}/${s.ns}/${s.name}`
  const lastAttemptAt = minutesAgo(4)
  const prOk = r.pr === Availability.AvailabilityOK
  const ciAvailability = s.ciAvailability ?? r.ci
  const ciOk = ciAvailability === Availability.AvailabilityOK
  const sourceError = r.pr === Availability.AvailabilityUnavailable ? authRequired : null

  const pr: PRSummary = {
    availability: r.pr,
    open: prOk ? (s.prs ?? 0) : null,
    draft: prOk ? (s.drafts ?? 0) : null,
    url: prOk ? `https://${r.host}/${s.ns}/${s.name}/pulls` : undefined,
    error: sourceError,
    lastSuccessAt: prOk ? lastAttemptAt : null,
    lastAttemptAt,
  }
  const ci: CISummary = {
    availability: ciAvailability,
    active: ciOk ? (s.ciActive ?? 0) : null,
    lastResult: ciOk ? s.ciResult : undefined,
    error: sourceError,
    lastSuccessAt: ciOk ? lastAttemptAt : null,
    lastAttemptAt,
  }
  const upstream = s.branch && !s.detached && !s.noUpstream ? `origin/${s.branch}` : undefined

  return {
    id: idOf(path),
    rootId: r.root.id,
    path,
    gitDir: `${path}/.git`,
    remotes: [
      {
        name: 'origin',
        fetchUrl: `git@${r.host}:${s.ns}/${s.name}.git`,
        host: r.host,
        path: `${s.ns}/${s.name}`,
        provider: r.provider,
      },
    ],
    remote: 'origin',
    branch: {
      name: s.detached ? undefined : s.branch,
      detached: Boolean(s.detached),
      headSha: s.readError ? undefined : (index * 2654435761 >>> 0).toString(16).padStart(8, '0') + 'c0ffee',
      upstream,
      ahead: upstream ? (s.ahead ?? 0) : null,
      behind: upstream ? (s.behind ?? 0) : null,
      lastFetchAt: upstream ? minutesAgo(15 + index * 7) : null,
    },
    worktree: {
      staged: s.staged ?? 0,
      unstaged: s.unstaged ?? 0,
      untracked: s.untracked ?? 0,
      conflicts: s.conflicts ?? 0,
      operation: s.operation,
    },
    pr,
    ci,
    error: s.readError ? { code: ErrorCode.CodeOther, message: s.readError } : null,
    updatedAt: minutesAgo(1),
  }
}

const mockTools: ToolDiagnostic[] = [
  { name: 'git', availability: Availability.AvailabilityOK, path: '/usr/bin/git', version: '2.43.0' },
  { name: 'gh', availability: Availability.AvailabilityOK, path: '/usr/bin/gh', version: '2.103.3' },
  { name: 'glab', availability: Availability.AvailabilityUnavailable, path: '/usr/local/bin/glab', version: '1.126.0', error: authRequired },
  { name: 'tea', availability: Availability.AvailabilityOK, path: '/usr/local/bin/tea', version: '0.16.0' },
]

function invalidInput(message: string): Error {
  return Object.assign(new Error(message), { cause: { code: ErrorCode.CodeInvalidInput, message } })
}

const uiKey = 'forktopus.demo.ui'

export interface MockOptions {
  /** Искусственная задержка ответов, мс: видно состояние загрузки. */
  delayMs?: number
}

export function createMockSource({ delayMs = 250 }: MockOptions = {}): DataSource {
  let roots = mockRoots.map((m) => m.root)
  let repos = scenarios.map(buildRepository)
  let ui: UI = {}
  try {
    ui = JSON.parse(localStorage.getItem(uiKey) ?? '{}') as UI
  } catch {
    // нет localStorage (тесты) или повреждённое значение — значения по умолчанию
  }

  const reply = <T>(value: T) => new Promise<T>((resolve) => setTimeout(() => resolve(structuredClone(value)), delayMs))

  return {
    kind: 'mock',
    listRoots: () => reply(roots),
    async addRoot(path) {
      const clean = path.trim().replace(/\/+$/, '')
      if (!clean.startsWith('/')) throw invalidInput(`нужен абсолютный путь: "${path}"`)
      const existing = roots.find((r) => r.path === clean)
      if (existing) return reply(existing)
      const root = { id: idOf(clean), path: clean }
      roots = [...roots, root]
      return reply(root)
    },
    async removeRoot(id) {
      if (!roots.some((r) => r.id === id)) throw invalidInput(`корневая папка "${id}" не найдена`)
      roots = roots.filter((r) => r.id !== id)
      repos = repos.filter((r) => r.rootId !== id)
      return reply(undefined)
    },
    listRepositories: () => reply(repos),
    checkTools: () => reply(mockTools),
    getUI: () => reply(ui),
    async saveUI(next) {
      ui = structuredClone(next)
      try {
        localStorage.setItem(uiKey, JSON.stringify(ui))
      } catch {
        // демо работает и без localStorage
      }
      return reply(undefined)
    },
  }
}
