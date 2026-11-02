// Демо-источник по сценариям макета idea/mockups/interactive-prototypes.html
// плюс состояния, которых в макете нет: нет upstream, detached HEAD, конфликт,
// ошибка чтения. Git и CLI не вызываются, данные вымышлены.

import {
  type AppUpdate,
  type BranchInfo,
  Availability,
  ErrorCode,
  IssueReason,
  Nesting,
  Operation,
  type Error as ModelError,
  type OpExpect,
  type OpRecord,
  type OpResult,
  Provider,
  type ChangedFile,
  type CISummary,
  type PRSummary,
  type RemoteStatus,
  type Repository,
  type Root,
  type ScanIssue,
  type ToolDiagnostic,
  type UI,
} from './model'
import type { DataSource, Snapshot } from './source'
import { remoteRef, within } from '../logic/repo'

interface MockRoot {
  root: Root
  host: string
  provider: Provider
  pr: Availability
  ci: Availability
}

const offline = (host: string) => ({ code: ErrorCode.CodeNetworkError, message: `gt: cannot reach https://${host} (dial tcp: lookup ${host}: no such host)` })
const authRequired = { code: ErrorCode.CodeAuthRequired, message: 'glab: нужна авторизация (glab auth login)' }

const mockRoots: MockRoot[] = [
  {
    root: { id: 'r-github', path: '/srv/repos/github.com' },
    host: 'github.com',
    provider: Provider.ProviderGitHub,
    pr: Availability.AvailabilityOK,
    ci: Availability.AvailabilityOK,
  },
  {
    root: { id: 'r-gitlab', path: '/srv/repos/gitlab.com' },
    host: 'gitlab.com',
    provider: Provider.ProviderGitLab,
    pr: Availability.AvailabilityUnavailable,
    ci: Availability.AvailabilityUnavailable,
  },
  {
    root: { id: 'r-gitea', path: '/srv/repos/git.home.test' },
    host: 'git.home.test',
    provider: Provider.ProviderGitea,
    pr: Availability.AvailabilityOK,
    ci: Availability.AvailabilityOK,
  },
  {
    root: { id: 'r-other', path: '/srv/repos/git.example.org' },
    host: 'git.example.org',
    provider: Provider.ProviderOther,
    pr: Availability.AvailabilityNotSupported,
    ci: Availability.AvailabilityNotSupported,
  },
]

interface Scenario {
  root: string
  ns: string
  name: string
  /** каталог относительно корня, если не `ns/name` (вложенные репозитории) */
  dir?: string
  nesting?: Nesting
  branch?: string
  detached?: boolean
  noUpstream?: boolean
  /** upstream настроен, но ветка на сервере удалена */
  upstreamGone?: boolean
  /** репозиторий без коммитов */
  unborn?: boolean
  ahead?: number
  behind?: number
  staged?: number
  unstaged?: number
  untracked?: number
  conflicts?: number
  operation?: string
  prs?: number
  drafts?: number
  /** Последний закрытый PR, когда открытых нет: номер, влит ли, сколько минут назад. */
  lastPR?: { number: number; merged: boolean; minutes: number }
  ciActive?: number
  ciResult?: string
  ciAvailability?: Availability
  /** PR и CI не получены: сервер недоступен, последние данные — час назад */
  offline?: boolean
  readError?: string
}

const scenarios: Scenario[] = [
  {
    root: 'r-github',
    ns: 'northwind',
    name: 'storefront',
    branch: 'feature/permissions',
    staged: 1,
    unstaged: 3,
    untracked: 1,
    ahead: 1,
    prs: 2,
    drafts: 1,
    ciActive: 1,
    ciResult: 'success',
  },
  {
    root: 'r-github',
    ns: 'northwind',
    name: 'ui-kit',
    dir: 'northwind/storefront/libs/ui-kit',
    nesting: Nesting.NestingSubmodule,
    detached: true,
    prs: 0,
    ciResult: 'success',
  },
  {
    root: 'r-github',
    ns: 'northwind',
    name: 'storefront-docs',
    dir: 'northwind/storefront/docs-site',
    nesting: Nesting.NestingNested,
    branch: 'main',
    untracked: 1,
    prs: 0,
    lastPR: { number: 41, merged: true, minutes: 2 * 24 * 60 },
  },
  { root: 'r-github', ns: 'acme', name: 'project-template', branch: 'main', behind: 3, prs: 1, ciResult: 'success' },
  {
    root: 'r-github',
    ns: 'acme',
    name: 'web-client',
    branch: 'ui/activity-feed',
    ahead: 2,
    behind: 1,
    prs: 1,
    ciResult: 'failure',
  },
  { root: 'r-github', ns: 'pixel-lab', name: 'image-tools', branch: 'main', ciResult: 'success' },
  { root: 'r-github', ns: 'dev', name: 'dev-scripts', branch: 'main', unstaged: 1 },
  { root: 'r-github', ns: 'octo-labs', name: 'tentacle', branch: 'spike/local-only', noUpstream: true, untracked: 2 },
  {
    root: 'r-github',
    ns: 'octo-labs',
    name: 'tentacle-docs',
    branch: 'main',
    conflicts: 2,
    unstaged: 1,
    operation: 'merge',
    ahead: 1,
    behind: 4,
    prs: 0,
    ciResult: 'success',
  },
  { root: 'r-gitlab', ns: 'studio/payments', name: 'service-api', branch: 'feat/refund', behind: 2 },
  { root: 'r-gitlab', ns: 'studio', name: 'landing', branch: 'main' },
  { root: 'r-gitea', ns: 'homelab', name: 'infrastructure', branch: 'main', behind: 1, prs: 1, ciActive: 2, ciResult: 'success' },
  { root: 'r-gitea', ns: 'homelab', name: 'services', branch: 'release/1.2', prs: 2, ciResult: 'failure' },
  { root: 'r-gitea', ns: 'homelab', name: 'media-server', branch: 'main', offline: true },
  {
    root: 'r-gitea',
    ns: 'homelab',
    name: 'dotfiles',
    branch: 'main',
    prs: 0,
    lastPR: { number: 7, merged: false, minutes: 5 * 60 },
    ciAvailability: Availability.AvailabilityNotSupported,
  },
  { root: 'r-other', ns: 'tools', name: 'shell-utils', branch: 'main', unstaged: 2 },
  { root: 'r-other', ns: 'tools', name: 'scratch', branch: 'main', unborn: true, noUpstream: true, untracked: 3 },
  { root: 'r-gitlab', ns: 'studio', name: 'promo-2025', branch: 'promo/autumn', upstreamGone: true },
  { root: 'r-other', ns: 'tools', name: 'firmware', detached: true },
  { root: 'r-other', ns: 'archive', name: 'legacy-site', readError: 'нет доступа к каталогу: permission denied' },
]

const minutesAgo = (m: number) => new Date(Date.now() - m * 60_000).toISOString()

function idOf(path: string): string {
  let h = 0
  for (const ch of path) h = (Math.imul(h, 31) + ch.charCodeAt(0)) | 0
  return 'm' + (h >>> 0).toString(16).padStart(8, '0')
}

/** Изменённые файлы под счётчики сценария; первый staged — переименование. */
function mockFiles(s: Scenario): ChangedFile[] {
  const files: ChangedFile[] = []
  for (let i = 0; i < (s.conflicts ?? 0); i++) files.push({ path: `src/merge/part-${i + 1}.ts`, code: 'UU' })
  for (let i = 0; i < (s.staged ?? 0); i++)
    files.push(i === 0 ? { path: 'docs/guide.md', origPath: 'docs/README old.md', code: 'R ' } : { path: `src/api/handler-${i}.ts`, code: 'M ' })
  for (let i = 0; i < (s.unstaged ?? 0); i++) files.push({ path: `src/components/Панель ${i + 1}.tsx`, code: ' M' })
  for (let i = 0; i < (s.untracked ?? 0); i++) files.push({ path: `notes/todo-${i + 1}.md`, code: '??' })
  return files
}

function buildRepository(s: Scenario, index: number): Repository {
  const r = mockRoots.find((m) => m.root.id === s.root)!
  const path = `${r.root.path}/${s.dir ?? `${s.ns}/${s.name}`}`
  const lastAttemptAt = minutesAgo(4)
  const prAvailability = s.offline ? Availability.AvailabilityError : r.pr
  const prOk = prAvailability === Availability.AvailabilityOK
  const ciAvailability = s.offline ? Availability.AvailabilityError : (s.ciAvailability ?? r.ci)
  const ciOk = ciAvailability === Availability.AvailabilityOK
  const sourceError = s.offline ? offline(r.host) : r.pr === Availability.AvailabilityUnavailable ? authRequired : null
  const lastSuccessAt = s.offline ? minutesAgo(65) : lastAttemptAt

  const pr: PRSummary = {
    availability: prAvailability,
    open: prOk ? (s.prs ?? 0) : null,
    draft: prOk ? (s.drafts ?? 0) : null,
    url: prOk ? `https://${r.host}/${s.ns}/${s.name}/pulls` : undefined,
    listUrl: prOk ? `https://${r.host}/${s.ns}/${s.name}/${r.provider === Provider.ProviderGitLab ? '-/merge_requests' : 'pulls'}` : undefined,
    error: sourceError,
    lastSuccessAt: prOk || s.offline ? lastSuccessAt : null,
    lastAttemptAt,
    last:
      prOk && !s.prs && s.lastPR
        ? {
            number: s.lastPR.number,
            merged: s.lastPR.merged,
            url: `https://${r.host}/${s.ns}/${s.name}/pulls/${s.lastPR.number}`,
            closedAt: minutesAgo(s.lastPR.minutes),
          }
        : undefined,
  }
  const ci: CISummary = {
    availability: ciAvailability,
    active: ciOk ? (s.ciActive ?? 0) : null,
    listUrl: ciOk ? `https://${r.host}/${s.ns}/${s.name}/${r.provider === Provider.ProviderGitLab ? '-/pipelines' : 'actions'}` : undefined,
    lastResult: ciOk ? s.ciResult : undefined,
    error: sourceError,
    lastSuccessAt: ciOk || s.offline ? lastSuccessAt : null,
    lastAttemptAt,
  }
  const upstream = s.branch && !s.detached && !s.noUpstream ? `origin/${s.branch}` : undefined
  const tracked = upstream && !s.upstreamGone

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
    nesting: s.nesting,
    branch: {
      name: s.detached ? undefined : s.branch,
      detached: Boolean(s.detached),
      headSha: s.readError || s.unborn ? undefined : ((index * 2654435761) >>> 0).toString(16).padStart(8, '0') + 'c0ffee',
      unborn: s.unborn,
      upstream,
      upstreamGone: s.upstreamGone,
      ahead: tracked ? (s.ahead ?? 0) : null,
      behind: tracked ? (s.behind ?? 0) : null,
      lastFetchAt: upstream ? minutesAgo(15 + index * 7) : null,
    },
    worktree: {
      staged: s.staged ?? 0,
      unstaged: s.unstaged ?? 0,
      untracked: s.untracked ?? 0,
      conflicts: s.conflicts ?? 0,
      operation: s.operation,
      files: mockFiles(s),
    },
    pr,
    ci,
    error: s.readError ? { code: ErrorCode.CodeOther, message: s.readError } : null,
    updatedAt: minutesAgo(1),
  }
}

const mockTools: ToolDiagnostic[] = [
  { name: 'git', availability: Availability.AvailabilityOK, path: '/usr/bin/git', version: '2.43.0' },
  { name: 'gh', host: 'github.com', availability: Availability.AvailabilityOK, path: '/usr/bin/gh', version: '2.103.3' },
  {
    name: 'glab',
    host: 'gitlab.com',
    availability: Availability.AvailabilityUnavailable,
    path: '/usr/local/bin/glab',
    version: '1.126.0',
    error: authRequired,
  },
  { name: 'gt', host: 'git.home.test', availability: Availability.AvailabilityOK, path: '/usr/local/bin/gt', version: '0.2.0' },
]

/** Ветки для демо: текущая, main и у каждого третьего — ветка, занятая соседним worktree. */
function mockBranches(repo: Repository, index: number): BranchInfo[] {
  const b = repo.branch
  if (!b || b.unborn) return []
  const list: BranchInfo[] = []
  if (b.name) {
    list.push({
      name: b.name,
      current: true,
      headSha: b.headSha ?? '',
      commitAt: minutesAgo(30 + index * 11),
      upstream: b.upstream,
      upstreamGone: b.upstreamGone,
      ahead: b.ahead,
      behind: b.behind,
    })
  }
  if (b.name !== 'main') {
    list.push({
      name: 'main',
      current: false,
      headSha: 'a1b2c3d4e5f6',
      commitAt: minutesAgo(60 * 24 * (index + 1)),
      upstream: 'origin/main',
      ahead: 0,
      behind: index % 2,
    })
  }
  if (index % 3 === 0) {
    list.push({ name: 'wip/experiment', current: false, headSha: '0badc0de1234', commitAt: minutesAgo(60 * 5), worktreePath: `${repo.path}-wip` })
  }
  return list
}

const blocked = (code: ErrorCode, message: string): ModelError => ({ code, message })

/** Упрощённая матрица предусловий Go (internal/gitops.Check) для демо. */
function mockCheck(op: Operation, repo: Repository, target: BranchInfo | undefined): ModelError | null {
  const b = repo.branch
  const w = repo.worktree
  if (!b || !w) return blocked(ErrorCode.CodeOther, 'статус репозитория не прочитан')
  const busy = w.conflicts > 0 || Boolean(w.operation)
  const dirty = w.staged + w.unstaged + w.untracked > 0
  const conflict = blocked(ErrorCode.CodeGitConflict, w.operation ? `не завершена операция Git: ${w.operation}` : 'есть неразрешённые конфликты')
  const dirtyErr = blocked(ErrorCode.CodeDirtyWorktree, 'есть локальные изменения: закоммитьте или отложите их в терминале')
  switch (op) {
    case Operation.OperationFetch:
      return repo.remote ? null : blocked(ErrorCode.CodeNoRemote, 'нечего fetch: у репозитория нет remote')
    case Operation.OperationPull:
      if (busy) return conflict
      if (dirty) return dirtyErr
      if (b.detached) return blocked(ErrorCode.CodeNoUpstream, 'detached HEAD: pull выполняется только на ветке')
      if (!b.upstream || b.upstreamGone) return blocked(ErrorCode.CodeNoUpstream, `у ветки ${b.name ?? ''} нет upstream`)
      if ((b.ahead ?? 0) > 0 && (b.behind ?? 0) > 0) return blocked(ErrorCode.CodeDiverged, `история разошлась с ${b.upstream}: fast-forward невозможен`)
      return null
    case Operation.OperationSwitch:
      if (!target) return blocked(ErrorCode.CodeInvalidInput, 'такой локальной ветки нет')
      if (target.current) return blocked(ErrorCode.CodeInvalidInput, `ветка ${target.name} уже выбрана`)
      if (target.worktreePath) return blocked(ErrorCode.CodeBranchInWorktree, `ветка ${target.name} открыта в другой рабочей копии: ${target.worktreePath}`)
      if (busy) return conflict
      if (dirty) return dirtyErr
      return null
  }
  return blocked(ErrorCode.CodeInvalidInput, `неизвестная операция ${String(op)}`)
}

function invalidInput(message: string): Error {
  return Object.assign(new Error(message), { cause: { code: ErrorCode.CodeInvalidInput, message } })
}

// Родитель вложенного репозитория — ближайший репозиторий выше по пути.
function linkParents(repos: Repository[]): Repository[] {
  return repos.map((r) => {
    if (!r.nesting) return r
    const parent = repos.filter((p) => r.path.startsWith(p.path + '/')).sort((a, b) => b.path.length - a.path.length)[0]
    return { ...r, parentId: parent?.id }
  })
}

const mockIssues: ScanIssue[] = [
  {
    rootId: 'r-other',
    path: '/srv/repos/git.example.org/private',
    reason: IssueReason.IssueNoAccess,
    error: { code: ErrorCode.CodeOther, message: 'open /srv/repos/git.example.org/private: permission denied' },
  },
]

/** Как filepath.Rel в Go: путь совпадает с каталогом или лежит внутри него. */

const uiKey = 'forktopus.demo.ui'

/** Корень без репозиториев — пустое состояние «в папке ничего не найдено». */
const emptyRoot: Root = { id: 'r-sandbox', path: '/srv/repos/sandbox' }

/**
 * Особые демо-состояния (параметр ?demo= в адресе): нет корней (A01), нет
 * Git в PATH (A28), не установлен CLI GitHub (A16).
 */
export type MockScenario = 'default' | 'noroots' | 'nogit' | 'nocli'

const missing = (name: string, message: string): ToolDiagnostic => ({
  name,
  host: name === 'git' ? undefined : 'github.com',
  availability: Availability.AvailabilityUnavailable,
  error: { code: ErrorCode.CodeCLIMissing, message },
})
const gitMissing = 'git не найден в PATH: установите Git и перезапустите Forktopus'
const ghMissing = 'gh не найден в PATH: PR и CI этого провайдера недоступны'

function withScenario(repo: Repository, scenario: MockScenario): Repository {
  if (scenario === 'nogit') {
    // Без git статус не прочитать: каждый репозиторий — с ошибкой чтения.
    return { ...repo, branch: null, worktree: null, error: { code: ErrorCode.CodeCLIMissing, message: 'git не найден в PATH' } }
  }
  if (scenario === 'nocli' && repo.remotes?.[0]?.host === 'github.com') {
    const error = { code: ErrorCode.CodeCLIMissing, message: ghMissing }
    return {
      ...repo,
      pr: { availability: Availability.AvailabilityUnavailable, error, lastAttemptAt: repo.pr.lastAttemptAt },
      ci: { availability: Availability.AvailabilityUnavailable, error, lastAttemptAt: repo.ci.lastAttemptAt },
    }
  }
  return repo
}

function toolsFor(scenario: MockScenario): ToolDiagnostic[] {
  if (scenario === 'nogit') return mockTools.map((d) => (d.name === 'git' ? missing('git', gitMissing) : d))
  if (scenario === 'nocli') return mockTools.map((d) => (d.name === 'gh' ? missing('gh', ghMissing) : d))
  return mockTools
}

export interface MockOptions {
  /** Искусственная задержка ответов, мс: видно состояние загрузки. */
  delayMs?: number
  scenario?: MockScenario
}

export function createMockSource({ delayMs = 250, scenario = 'default' }: MockOptions = {}): DataSource {
  // Демо не обращается к Releases и не создаёт файлы; прогресс живёт в источнике.
  let update: AppUpdate = {
    revision: 1,
    status: 'idle',
    currentVersion: '0.3.1',
    version: '',
    format: 'appimage',
    filename: '',
    size: 0,
    downloaded: 0,
    releaseUrl: '',
    checkedAt: '',
    message: 'Демонстрация: пакеты не скачиваются.',
    canDownload: false,
    instruction: 'В установленном приложении здесь будет инструкция ручной установки.',
  }
  const updateListeners = new Set<(state: AppUpdate) => void>()
  let updateTimer: ReturnType<typeof setTimeout> | undefined
  const changeUpdate = (patch: Partial<AppUpdate>) => {
    update = { ...update, ...patch, revision: update.revision + 1 }
    updateListeners.forEach((cb) => cb(update))
    return update
  }
  const noRoots = scenario === 'noroots'
  let roots = noRoots ? [] : [...mockRoots.map((m) => m.root), emptyRoot]
  let repos = noRoots ? [] : linkParents(scenarios.map(buildRepository)).map((r) => withScenario(r, scenario))
  const tools = toolsFor(scenario)
  // Как scan.DefaultSkipDirs в Go; демо-скан список не учитывает.
  const defaultSkipDirs = ['node_modules', 'vendor', '.venv', 'venv', '__pycache__', 'dist', 'build', 'target', '.cache']
  let skipDirs: string[] = []
  let skipPaths: string[] = []
  const skipped = (path: string) => skipPaths.some((p) => within(path, p))
  let issues = noRoots ? [] : mockIssues
  // Как будто кэш прошлого запуска: два часа назад, не подтверждён сканированием.
  let snapshot: Snapshot = { repositories: repos, issues, scannedAt: minutesAgo(120), stale: true }
  let picked = 0
  const opLog: OpRecord[] = []
  const running = new Set<string>()
  const listeners = new Set<(repo: Repository) => void>()
  const remoteListeners = new Set<(status: RemoteStatus) => void>()
  let ui: UI = {}
  let autostart = false
  let noAutoCheck: string[] = []
  try {
    ui = JSON.parse(localStorage.getItem(uiKey) ?? '{}') as UI
  } catch {
    // нет localStorage (тесты) или повреждённое значение — значения по умолчанию
  }

  const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))
  const reply = async <T>(value: T) => {
    await sleep(delayMs)
    return structuredClone(value)
  }
  const known = (rootId: string) => roots.some((r) => r.id === rootId)

  // Как AppService.runOp: блокировка, сверка с показанным, предусловия, «Git», свежий статус.
  async function runOp(repoId: string, op: Operation, branch: string, expect: OpExpect): Promise<OpResult> {
    const index = repos.findIndex((r) => r.id === repoId)
    if (index < 0) throw invalidInput(`репозиторий "${repoId}" не найден`)
    const repo = repos[index]
    const start = Date.now()
    const record: OpRecord = {
      repoId,
      op,
      target: op === Operation.OperationFetch ? repo.remote : branch || undefined,
      at: new Date(start).toISOString(),
      durationMs: 0,
    }
    const finish = (error: ModelError | null) => {
      repo.updatedAt = new Date().toISOString()
      const done = { ...record, durationMs: Date.now() - start, error }
      opLog.push(done)
      return reply<OpResult>({ ...done, repo })
    }
    if (running.has(repoId)) return finish(blocked(ErrorCode.CodeRepoBusy, 'над репозиторием уже выполняется операция, повторите после её завершения'))
    running.add(repoId)
    try {
      await sleep(delayMs * 3)
      const b = repo.branch
      if (!b || (b.name ?? '') !== expect.branch || (b.headSha ?? '') !== expect.headSha) {
        return finish(blocked(ErrorCode.CodeStateChanged, 'состояние репозитория изменилось, пока вы его смотрели: проверьте его и повторите'))
      }
      const target = op === Operation.OperationSwitch ? mockBranches(repo, index).find((x) => x.name === branch) : undefined
      const error = mockCheck(op, repo, target)
      if (error) return finish(error)
      if (op === Operation.OperationFetch) b.lastFetchAt = new Date().toISOString()
      if (op === Operation.OperationPull) b.behind = 0
      if (op === Operation.OperationSwitch && target) {
        Object.assign(b, {
          name: target.name,
          headSha: target.headSha,
          upstream: target.upstream,
          upstreamGone: target.upstreamGone,
          ahead: target.ahead,
          behind: target.behind,
        })
      }
      return finish(null)
    } finally {
      running.delete(repoId)
    }
  }

  // Как AppService.remoteStatus: данные ветки из сценария; обновление
  // освежает время попытки и рассылает событие.
  async function remoteStatus(repoId: string, force: boolean): Promise<RemoteStatus> {
    const repo = repos.find((r) => r.id === repoId)
    if (!repo) throw invalidInput(`репозиторий "${repoId}" не найден`)
    if (force) {
      await sleep(delayMs * 2)
      const at = new Date().toISOString()
      repo.pr = { ...repo.pr, lastAttemptAt: at, lastSuccessAt: repo.pr.availability === Availability.AvailabilityOK ? at : repo.pr.lastSuccessAt }
      repo.ci = { ...repo.ci, lastAttemptAt: at, lastSuccessAt: repo.ci.availability === Availability.AvailabilityOK ? at : repo.ci.lastSuccessAt }
    }
    const remote = repo.remotes?.find((r) => r.name === repo.remote)
    const status: RemoteStatus = { repoId, branch: remoteRef(repo), provider: remote?.provider ?? Provider.$zero, pr: repo.pr, ci: repo.ci }
    remoteListeners.forEach((cb) => cb(structuredClone(status)))
    return reply(status)
  }

  return {
    kind: 'mock',
    getAppUpdate: () => reply(update),
    checkAppUpdate: async () => {
      if (['checking', 'downloading', 'verifying'].includes(update.status)) return update
      return changeUpdate({
        status: 'available',
        version: '0.4.0',
        filename: 'Forktopus-0.4.0-x86_64.AppImage',
        size: 64 * 1024 * 1024,
        downloaded: 0,
        canDownload: true,
        releaseUrl: 'https://github.com/forktopus/forktopus/releases',
        checkedAt: new Date().toISOString(),
        message: 'Демонстрация доступного обновления.',
      })
    },
    downloadAppUpdate: async () => {
      if (!update.canDownload) return update
      const state = changeUpdate({ status: 'downloading', downloaded: 0, canDownload: false, message: '' })
      const tick = () => {
        if (update.status !== 'downloading') return
        if (update.downloaded < update.size) {
          changeUpdate({ downloaded: Math.min(update.size, update.downloaded + 16 * 1024 * 1024) })
          updateTimer = setTimeout(tick, 200)
        } else {
          changeUpdate({ status: 'verifying' })
          updateTimer = setTimeout(() => changeUpdate({ status: 'ready', message: 'Демонстрация: файл не создавался.' }), 200)
        }
      }
      updateTimer = setTimeout(tick, 200)
      return state
    },
    cancelAppUpdate: async () => {
      clearTimeout(updateTimer)
      return changeUpdate({ status: 'available', canDownload: true, downloaded: 0, message: 'Скачивание отменено.' })
    },
    openAppUpdateFolder: async () => {
      console.info('Демонстрация: папка обновления не создавалась.')
    },
    onAppUpdate(cb) {
      updateListeners.add(cb)
      return () => {
        updateListeners.delete(cb)
      }
    },
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
      if (!known(id)) throw invalidInput(`корневая папка "${id}" не найдена`)
      roots = roots.filter((r) => r.id !== id)
      repos = repos.filter((r) => r.rootId !== id)
      issues = issues.filter((i) => i.rootId !== id)
      snapshot = { ...snapshot, repositories: repos, issues }
      return reply(undefined)
    },
    getSkipDirs: () => reply({ dirs: skipDirs.length ? skipDirs : defaultSkipDirs, defaults: defaultSkipDirs }),
    async setSkipDirs(dirs) {
      const clean: string[] = []
      for (const raw of dirs) {
        const d = raw.trim()
        if (!d || clean.includes(d)) continue
        if (d === '.' || d === '..' || /[/\\]/.test(d)) throw invalidInput(`нужно имя каталога, а не путь: "${d}"`)
        clean.push(d)
      }
      const same = clean.length === defaultSkipDirs.length && clean.every((d) => defaultSkipDirs.includes(d))
      skipDirs = same ? [] : clean
      return reply({ dirs: skipDirs.length ? skipDirs : defaultSkipDirs, defaults: defaultSkipDirs })
    },
    getSkipPaths: () => reply([...skipPaths]),
    async setSkipPaths(paths) {
      const clean = [...new Set(paths.map((p) => p.trim()).filter(Boolean))].sort()
      const relative = clean.find((p) => !p.startsWith('/'))
      if (relative) throw invalidInput(`нужен абсолютный путь: "${relative}"`)
      skipPaths = clean
      issues = issues.filter((i) => !skipped(i.path))
      snapshot = { ...snapshot, issues, repositories: snapshot.repositories.filter((r) => !skipped(r.path)) }
      return reply([...issues])
    },
    pickFolder: () => reply(`/srv/repos/demo-folder-${++picked}`),
    listRepositories: () => reply(snapshot),
    async scanRoots() {
      repos = repos.filter((r) => known(r.rootId))
      // Исключённые каталоги (D-071) сканер обходит; сами данные демо не теряются — вернутся после «Вернуть в сканирование».
      const shown = () => repos.filter((r) => !skipped(r.path))
      for (const repo of shown()) {
        await sleep(delayMs / 5)
        // Как в Go: во время сканирования parentId ещё не известен.
        listeners.forEach((cb) => cb(structuredClone({ ...repo, parentId: undefined, updatedAt: new Date().toISOString() })))
      }
      const scannedAt = new Date().toISOString()
      repos = repos.map((r) => ({ ...r, updatedAt: scannedAt }))
      issues = (noRoots ? [] : mockIssues).filter((i) => known(i.rootId) && !skipped(i.path))
      snapshot = { repositories: shown(), issues, scannedAt, stale: false }
      return reply({ repositories: shown(), issues, scannedAt })
    },
    async selectRemote(repoId, remote) {
      const repo = repos.find((r) => r.id === repoId)
      if (!repo) throw invalidInput(`репозиторий "${repoId}" не найден`)
      repo.remote = remote || 'origin'
      return reply(undefined)
    },
    async getBranches(repoId) {
      const index = repos.findIndex((r) => r.id === repoId)
      if (index < 0) throw invalidInput(`репозиторий "${repoId}" не найден`)
      return reply(mockBranches(repos[index], index))
    },
    async refreshRepo(repoId) {
      const repo = repos.find((r) => r.id === repoId)
      if (!repo) throw invalidInput(`репозиторий "${repoId}" не найден`)
      repo.updatedAt = new Date().toISOString()
      return reply(repo)
    },
    fetchRepo: (repoId, expect) => runOp(repoId, Operation.OperationFetch, '', expect),
    pullRepo: (repoId, expect) => runOp(repoId, Operation.OperationPull, '', expect),
    switchBranch: (repoId, branch, expect) => runOp(repoId, Operation.OperationSwitch, branch, expect),
    getRemoteStatus: (repoId) => remoteStatus(repoId, false),
    refreshRemote: (repoId) => remoteStatus(repoId, true),
    listOperations: () => reply([...opLog].reverse()),
    checkTools: () => reply(tools),
    getDiagnostics() {
      const errors = tools
        .filter((d) => d.error)
        .map((d) => ({ time: new Date().toISOString(), level: 'WARN', message: `${d.name}${d.host ? ` ${d.host}` : ''}: ${d.error!.message}` }))
      const lines = [
        'Forktopus — отчёт диагностики (демо-данные)',
        'ОС: браузер, демо-режим',
        'Настройки: ~/.config/forktopus/config.json',
        'Лог: ~/.cache/forktopus/forktopus.log',
        `Корневых папок: ${roots.length}, репозиториев: ${repos.length}`,
        '',
        'Утилиты:',
        ...tools.map((d) => `- ${[d.name, d.host, d.version, d.availability, d.error?.message].filter(Boolean).join(' · ')}`),
        '',
        'Последние ошибки:',
        ...(errors.length ? errors.map((e) => `- ${e.level} ${e.message}`) : ['- нет']),
        '',
        'Последние операции:',
        ...(opLog.length
          ? [...opLog]
              .reverse()
              .slice(0, 10)
              .map((o) => `- ${o.op} ${o.target ?? ''} — ${o.error ? o.error.code : 'готово'}`)
          : ['- нет']),
      ]
      return reply({
        os: 'браузер, демо-режим',
        configPath: '~/.config/forktopus/config.json',
        logPath: '~/.cache/forktopus/forktopus.log',
        errors,
        report: lines.join('\n') + '\n',
      })
    },
    async openURL(url) {
      window.open(url, '_blank', 'noopener')
    },
    copyText: (text) => navigator.clipboard.writeText(text),
    // Файлового менеджера и терминала у браузера нет: в превью — только в консоль.
    async openFolder(path) {
      console.info('openFolder', path)
    },
    async openTerminal(path) {
      console.info('openTerminal', path)
    },
    getUI: () => reply(ui),
    // Трея в браузере нет.
    setTraySummary: () => reply(undefined),
    // Автозапуск в превью — только в памяти.
    getAutostart: () => reply(autostart),
    async setAutostart(on) {
      autostart = on
      return reply(undefined)
    },
    getNoAutoCheck: () => reply([...noAutoCheck]),
    async setAutoCheck(path, on) {
      noAutoCheck = noAutoCheck.filter((p) => p !== path)
      if (!on) noAutoCheck = [...noAutoCheck, path].sort()
      return reply([...noAutoCheck])
    },
    // В браузере без Wails нативного зума нет — CSS zoom, только для превью.
    async setZoom(zoom) {
      if (typeof document !== 'undefined') document.documentElement.style.zoom = String(zoom)
    },
    async saveUI(next) {
      ui = structuredClone(next)
      try {
        localStorage.setItem(uiKey, JSON.stringify(ui))
      } catch {
        // демо работает и без localStorage
      }
      return reply(undefined)
    },
    onScanRepository(cb) {
      listeners.add(cb)
      return () => listeners.delete(cb)
    },
    onRemoteStatus(cb) {
      remoteListeners.add(cb)
      return () => remoteListeners.delete(cb)
    },
    // Фоновой проверки в демо нет: статус меняется только сканированием.
    onRepository: () => () => {},
    onChecked: () => () => {},
    onReveal: () => () => {},
  }
}
