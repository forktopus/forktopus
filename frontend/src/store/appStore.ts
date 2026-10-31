// Единый store приложения. Режимы читают одни и те же данные, поэтому
// переключение режима не перечитывает репозитории (A26). Режим, тема и
// фильтры сохраняются в настройках Go через DataSource.saveUI.

import { createStore, type StoreApi } from 'zustand/vanilla'
import { Operation, type BranchInfo, type Diagnostics, type OpResult, type RemoteStatus, type Repository, type Root, type ScanIssue, type ToolDiagnostic, type UI } from '../data/model'
import { errorOf, type DataSource, type Snapshot } from '../data/source'
import { t } from '../i18n'
import { registryFilters, sortKeys, type RegistryFilter, type SortKey } from '../logic/registry'
import { opResultLabel } from '../logic/ops'
import { branchOf, repoName } from '../logic/repo'
import type { Tone } from '../theme/theme'
import type { ThemeMode } from '../theme/tokens'

export type Mode = 'registry' | 'explorer' | 'radar'
export const modes: Mode[] = ['registry', 'explorer', 'radar']

/** Тема из настроек; null — как в системе. */
export type ThemeChoice = ThemeMode | null

export interface Filters {
  registry: RegistryFilter
  host: string
  sort: SortKey
  asc: boolean
}

export const defaultFilters: Filters = { registry: 'all', host: '', sort: 'name', asc: true }

/** Срок свежести PR/CI в кэше Go (D-046): с этим шагом серверные данные перезапрашиваются в фоне. */
export const remoteTTLms = 2 * 60 * 1000

/** Как часто найденные во время сканирования репозитории и фоновые PR/CI попадают в список. */
export const scanFlushMs = 150

/** Уведомление внизу экрана: итог операции, ошибка, копирование. */
export interface Notice {
  id: number
  tone: Tone
  text: string
}

/** Операция, ждущая подтверждения пользователя. */
export interface PendingOp {
  id: string
  op: Operation
  branch?: string
}

export interface AppState {
  sourceKind: DataSource['kind']
  /** первые данные (корни и кэш репозиториев) загружены */
  ready: boolean
  scanning: boolean
  /** сколько репозиториев найдено текущим сканированием (прогресс по событиям) */
  scanFound: number
  roots: Root[]
  repos: Record<string, Repository>
  /** каталоги, которые не удалось просмотреть при последнем сканировании */
  issues: ScanIssue[]
  /** список из кэша прошлого запуска, ещё не подтверждён сканированием (D-027) */
  stale: boolean
  /** время последнего сканирования (в том числе закэшированного) */
  lastCheckAt: string | null
  tools: ToolDiagnostic[]
  toolsCheckedAt: string | null
  mode: Mode
  theme: ThemeChoice
  filters: Filters
  query: string
  selectedId: string | null
  /** операция, которая сейчас идёт над репозиторием: кнопки операций заблокированы (D-031) */
  running: Record<string, Operation>
  /** итог последней операции над репозиторием */
  lastOps: Record<string, OpResult>
  /** уведомления, старые — первыми */
  notices: Notice[]
  /** репозиторий, открытый в панели деталей */
  detailsId: string | null
  pendingOp: PendingOp | null

  init(): Promise<void>
  /** перечитать корни и пересканировать */
  refresh(): Promise<void>
  scan(): Promise<void>
  checkTools(): Promise<void>
  /**
   * Фоновое обновление PR/MR и CI по TTL: Go отдаёт свежее из кэша и
   * перезапрашивает устаревшее. Ошибки не показываются — они в самих данных.
   */
  pollRemote(): Promise<void>
  /** отчёт диагностики; null — ошибка (она уже в уведомлениях) */
  loadDiagnostics(): Promise<Diagnostics | null>
  addRoot(path: string): Promise<boolean>
  /** системный диалог выбора папки + addRoot; false — диалог закрыт или ошибка */
  pickAndAddRoot(): Promise<boolean>
  removeRoot(id: string): Promise<void>
  /** перечитать локальный статус одного репозитория и мимо кэша — его PR/MR и CI */
  refreshRepo(id: string): Promise<void>
  /** локальные ветки репозитория; null — ошибка (она уже в уведомлениях) */
  loadBranches(id: string): Promise<BranchInfo[] | null>
  /**
   * Fetch / ff-pull / switch. Отпечаток — ветка и HEAD из показанного статуса
   * (D-032). Итог — в lastOps, свежий статус — в repos.
   */
  runOp(id: string, op: Operation, branch?: string): Promise<OpResult | null>
  /** Fetch выполняется сразу, Pull и Switch — после подтверждения (D-047). */
  requestOp(id: string, op: Operation, branch?: string): void
  confirmOp(): Promise<void>
  cancelOp(): void
  openDetails(id: string | null): void
  notify(tone: Tone, text: string): void
  dismissNotice(id: number): void
  copyText(text: string): Promise<void>
  openURL(url: string): void
  setMode(mode: Mode): void
  setTheme(theme: ThemeMode): void
  setFilters(patch: Partial<Filters>): void
  setQuery(query: string): void
  select(id: string | null): void
}

export type AppStore = StoreApi<AppState>

function oneOf<T extends string>(value: string | undefined, allowed: readonly T[], fallback: T): T {
  return allowed.includes(value as T) ? (value as T) : fallback
}

/** Состояние интерфейса из настроек; неизвестные значения — по умолчанию. */
export function fromUI(ui: UI): Pick<AppState, 'mode' | 'theme' | 'filters'> {
  const f = ui.filters ?? {}
  return {
    mode: oneOf(ui.mode, modes, 'registry'),
    theme: ui.theme === 'light' || ui.theme === 'dark' ? ui.theme : null,
    filters: {
      registry: oneOf(f.registry, registryFilters, defaultFilters.registry),
      host: f.host ?? '',
      sort: oneOf(f.sort, sortKeys, defaultFilters.sort),
      asc: f.dir !== 'desc',
    },
  }
}

export function toUI(s: Pick<AppState, 'mode' | 'theme' | 'filters'>): UI {
  const f = s.filters
  return {
    mode: s.mode,
    theme: s.theme ?? '',
    filters: { registry: f.registry, host: f.host, sort: f.sort, dir: f.asc ? 'asc' : 'desc' },
  }
}

const byId = (repos: Repository[]) => Object.fromEntries(repos.map((r) => [r.id, r]))

/** Серверные данные в репозитории, если получены для его текущей ветки (как withRemoteLocked в Go). */
export function withRemote(repo: Repository, st: RemoteStatus | undefined): Repository {
  if (!st || branchOf(repo) !== st.branch) return repo
  const remotes = st.provider && repo.remotes ? repo.remotes.map((r) => (r.name === repo.remote ? { ...r, provider: st.provider } : r)) : repo.remotes
  return { ...repo, pr: st.pr, ci: st.ci, remotes }
}

export function createAppStore(source: DataSource): AppStore {
  // Последние серверные данные по репозиториям. Событие может прийти раньше
  // ответа сканирования или обновления, где они ещё прежние, — поэтому они
  // накладываются на каждый пришедший из Go репозиторий.
  const remote = new Map<string, RemoteStatus>()
  const withKnown = (repo: Repository) => withRemote(repo, remote.get(repo.id))
  const known = (repos: Repository[]) => byId(repos.map(withKnown))
  const applyRemote = (st: RemoteStatus) => {
    remote.set(st.repoId, st)
    const { repos } = store.getState()
    if (repos[st.repoId]) store.setState({ repos: { ...repos, [st.repoId]: withRemote(repos[st.repoId], st) } })
  }
  // Фоновые данные (после сканирования, опрос по TTL) приходят по одному на
  // репозиторий — в store они попадают пачкой, как и события сканирования.
  const remoteDirty = new Set<string>()
  let remoteTimer: ReturnType<typeof setTimeout> | undefined
  const flushRemote = () => {
    remoteTimer = undefined
    const { repos } = store.getState()
    let next: Record<string, Repository> | undefined
    for (const id of remoteDirty) {
      if (repos[id]) (next ??= { ...repos })[id] = withRemote(repos[id], remote.get(id))
    }
    remoteDirty.clear()
    if (next) store.setState({ repos: next })
  }
  const queueRemote = (st: RemoteStatus) => {
    remote.set(st.repoId, st)
    remoteDirty.add(st.repoId)
    remoteTimer ??= setTimeout(flushRemote, scanFlushMs)
  }

  // Репозитории, найденные текущим сканированием. События приходят пачкой
  // (сотни за доли секунды) и попадают в store не чаще раза в scanFlushMs:
  // перерисовка списка на каждое событие замораживала UI (11.1).
  const found = new Set<string>()
  let pending: Repository[] = []
  let flushTimer: ReturnType<typeof setTimeout> | undefined
  const dropPending = () => {
    clearTimeout(flushTimer)
    flushTimer = undefined
    pending = []
  }
  const flushPending = () => {
    const batch = pending
    dropPending()
    const { scanning, repos } = store.getState()
    if (!scanning || batch.length === 0) return
    const next = { ...repos }
    // В событии ещё нет parentId — берём из прежней записи; итог сканирования всё заменит.
    for (const repo of batch) next[repo.id] = withKnown({ ...repo, parentId: repo.parentId ?? repos[repo.id]?.parentId })
    store.setState({ repos: next, scanFound: found.size })
  }

  const store = createStore<AppState>()((set, get) => {
    let noticeSeq = 0
    const notify = (tone: Tone, text: string) => set({ notices: [...get().notices, { id: ++noticeSeq, tone, text }] })
    const fail = (err: unknown) => notify('bad', errorOf(err).message)
    const persist = () => {
      source.saveUI(toUI(get())).catch(fail)
    }
    const applySnapshot = (snap: Snapshot) =>
      set({ repos: known(snap.repositories), issues: snap.issues, stale: snap.stale, lastCheckAt: snap.scannedAt ?? null })

    return {
      sourceKind: source.kind,
      ready: false,
      scanning: false,
      scanFound: 0,
      roots: [],
      repos: {},
      issues: [],
      stale: false,
      lastCheckAt: null,
      tools: [],
      toolsCheckedAt: null,
      mode: 'registry',
      theme: null,
      filters: defaultFilters,
      query: '',
      selectedId: null,
      running: {},
      lastOps: {},
      notices: [],
      detailsId: null,
      pendingOp: null,

      // Сначала — сохранённое (UI, корни, кэш), чтобы экран появился сразу;
      // затем в фоне сканирование и диагностика CLI.
      async init() {
        try {
          set(fromUI(await source.getUI()))
          const [roots, snap] = await Promise.all([source.listRoots(), source.listRepositories()])
          set({ roots })
          applySnapshot(snap)
        } catch (err) {
          fail(err)
        }
        set({ ready: true })
        await Promise.all([get().scan(), get().checkTools()])
      },

      async refresh() {
        try {
          set({ roots: await source.listRoots() })
        } catch (err) {
          fail(err)
        }
        await get().scan()
      },

      async scan() {
        if (get().scanning) return
        found.clear()
        dropPending()
        set({ scanning: true, scanFound: 0 })
        try {
          const res = await source.scanRoots()
          dropPending()
          set({ repos: known(res.repositories), issues: res.issues, stale: false, lastCheckAt: res.scannedAt, scanFound: res.repositories.length })
        } catch (err) {
          fail(err)
        } finally {
          set({ scanning: false })
        }
      },

      async checkTools() {
        try {
          set({ tools: await source.checkTools(), toolsCheckedAt: new Date().toISOString() })
        } catch (err) {
          fail(err)
        }
      },

      async pollRemote() {
        const { scanning, repos, running } = get()
        // После сканирования Go запрашивает всё сам; во время операции статус ещё изменится.
        if (scanning) return
        const ids = Object.values(repos)
          .filter((r) => r.remote && !running[r.id])
          .map((r) => r.id)
        await Promise.all(ids.map((id) => source.getRemoteStatus(id).then(queueRemote, () => undefined)))
      },

      async loadDiagnostics() {
        try {
          return await source.getDiagnostics()
        } catch (err) {
          fail(err)
          return null
        }
      },

      async addRoot(path) {
        try {
          await source.addRoot(path)
          set({ roots: await source.listRoots() })
        } catch (err) {
          fail(err)
          return false
        }
        void get().scan()
        return true
      },

      async pickAndAddRoot() {
        let path: string
        try {
          path = await source.pickFolder()
        } catch (err) {
          fail(err)
          return false
        }
        return path ? get().addRoot(path) : false
      },

      async removeRoot(id) {
        try {
          await source.removeRoot(id)
          const [roots, snap] = await Promise.all([source.listRoots(), source.listRepositories()])
          set({ roots })
          applySnapshot(snap)
        } catch (err) {
          fail(err)
        }
      },

      // Сначала локальный статус (быстро), затем сервер: так ответ Go с
      // прежними PR/CI не перекроет свежие.
      async refreshRepo(id) {
        try {
          const repo = await source.refreshRepo(id)
          if (get().repos[id]) set({ repos: { ...get().repos, [id]: withKnown(repo) } })
          applyRemote(await source.refreshRemote(id))
        } catch (err) {
          fail(err)
        }
      },

      async loadBranches(id) {
        try {
          return await source.getBranches(id)
        } catch (err) {
          fail(err)
          return null
        }
      },

      async runOp(id, op, branch = '') {
        const repo = get().repos[id]
        if (!repo || get().running[id]) return null
        const expect = { branch: repo.branch?.name ?? '', headSha: repo.branch?.headSha ?? '' }
        set({ running: { ...get().running, [id]: op } })
        try {
          const res =
            op === Operation.OperationFetch
              ? await source.fetchRepo(id, expect)
              : op === Operation.OperationPull
                ? await source.pullRepo(id, expect)
                : await source.switchBranch(id, branch, expect)
          const repos = get().repos
          set({ lastOps: { ...get().lastOps, [id]: res }, repos: repos[id] ? { ...repos, [id]: withKnown(res.repo) } : repos })
          const label = opResultLabel(res)
          notify(label.tone, `${repoName(res.repo)}: ${label.text}`)
          return res
        } catch (err) {
          fail(err)
          return null
        } finally {
          const running = { ...get().running }
          delete running[id]
          set({ running })
        }
      },

      setMode(mode) {
        set({ mode })
        persist()
      },

      setTheme(theme) {
        set({ theme })
        persist()
      },

      setFilters(patch) {
        set({ filters: { ...get().filters, ...patch } })
        persist()
      },

      setQuery: (query) => set({ query }),
      select: (selectedId) => set({ selectedId }),
      requestOp(id, op, branch) {
        if (op === Operation.OperationFetch) void get().runOp(id, op)
        else set({ pendingOp: { id, op, branch } })
      },

      async confirmOp() {
        const p = get().pendingOp
        if (!p) return
        set({ pendingOp: null })
        await get().runOp(p.id, p.op, p.branch)
      },

      cancelOp: () => set({ pendingOp: null }),
      openDetails: (detailsId) => set({ detailsId }),
      notify,
      dismissNotice: (id) => set({ notices: get().notices.filter((n) => n.id !== id) }),

      async copyText(text) {
        try {
          await source.copyText(text)
          notify('good', t('notice.copied'))
        } catch {
          notify('warn', t('notice.copyFailed', { text }))
        }
      },

      openURL(url) {
        source.openURL(url).catch(fail)
      },
    }
  })

  // Найденные по ходу сканирования репозитории видны в списке с задержкой не больше scanFlushMs.
  source.onScanRepository((repo) => {
    if (!store.getState().scanning) return
    found.add(repo.id)
    pending.push(repo)
    flushTimer ??= setTimeout(flushPending, scanFlushMs)
  })

  // PR/MR и CI приходят в фоне после сканирования, смены ветки и обновления.
  source.onRemoteStatus(queueRemote)

  return store
}
