// Единый store приложения. Режимы читают одни и те же данные, поэтому
// переключение режима не перечитывает репозитории (A26). Режим, тема и
// фильтры сохраняются в настройках Go через DataSource.saveUI.

import { createStore, type StoreApi } from 'zustand/vanilla'
import {
  Operation,
  type BranchInfo,
  type Diagnostics,
  type Notify,
  type OpResult,
  type RemoteStatus,
  type Repository,
  type Root,
  type ScanIssue,
  type ToolDiagnostic,
  type UI,
} from '../data/model'
import { errorOf, type DataSource, type SkipList, type Snapshot } from '../data/source'
import { t } from '../i18n'
import { registryFilters, sortKeys, type RegistryFilter, type SortKey } from '../logic/registry'
import { opResultLabel } from '../logic/ops'
import { traySummary } from '../logic/radar'
import { remoteRef, repoName } from '../logic/repo'
import { clampZoom, minZoom } from '../logic/zoom'
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

/** Как часто найденные во время сканирования репозитории и фоновые данные попадают в список. */
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
  /** Fetch с --prune (D-057). */
  fetchPrune: boolean
  /** Масштаб интерфейса, 1–2 (D-061). */
  zoom: number
  /** Интервал фоновой проверки, минуты (D-063). */
  backgroundMinutes: number
  /** Fetch перед фоновой проверкой (D-063). */
  autoFetch: boolean
  /** Крестик закрывает приложение, а не прячет окно в трей (D-063). */
  quitOnClose: boolean
  /** Запуск при входе в систему; null — не поддерживается или ещё не прочитан. */
  autostart: boolean | null
  /** Какие системные уведомления показывать (D-063). */
  notifications: Notify
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
  /** отчёт диагностики; null — ошибка (она уже в уведомлениях) */
  loadDiagnostics(): Promise<Diagnostics | null>
  addRoot(path: string): Promise<boolean>
  /** каталоги, в которые сканер не заходит (D-024); null — ошибка (она уже в уведомлениях) */
  loadSkipDirs(): Promise<SkipList | null>
  /** сохраняет список пропусков и пересканирует; null — ошибка */
  saveSkipDirs(dirs: string[]): Promise<SkipList | null>
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
  setFetchPrune(on: boolean): void
  setZoom(zoom: number): void
  setBackgroundMinutes(minutes: number): void
  setAutoFetch(on: boolean): void
  setQuitOnClose(on: boolean): void
  /** Перечитать автозапуск из системы (открыт диалог настроек). */
  loadAutostart(): Promise<void>
  setAutostart(on: boolean): Promise<void>
  setNotifications(patch: Partial<Notify>): void
  setFilters(patch: Partial<Filters>): void
  setQuery(query: string): void
  select(id: string | null): void
}

export type AppStore = StoreApi<AppState>

function oneOf<T extends string>(value: string | undefined, allowed: readonly T[], fallback: T): T {
  return allowed.includes(value as T) ? (value as T) : fallback
}

/** Состояние интерфейса из настроек; неизвестные значения — по умолчанию. */
type SavedUI = Pick<AppState, 'mode' | 'theme' | 'filters' | 'fetchPrune' | 'zoom' | 'backgroundMinutes' | 'autoFetch' | 'quitOnClose' | 'notifications'>

/** Интервалы фоновой проверки на выбор, минуты; по умолчанию — 5 (как в Go). */
export const backgroundIntervals = [1, 2, 5, 10, 15, 30, 60] as const
export const defaultBackgroundMinutes = 5
/** По умолчанию включены все уведомления (как в Go). */
export const defaultNotify: Notify = { ci: true, behind: true, pr: true }

export function fromUI(ui: UI): SavedUI {
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
    fetchPrune: ui.fetchPrune === true,
    zoom: clampZoom(ui.zoom),
    backgroundMinutes: backgroundIntervals.find((m) => m === ui.backgroundMinutes) ?? defaultBackgroundMinutes,
    autoFetch: ui.autoFetch === true,
    quitOnClose: ui.quitOnClose === true,
    notifications: ui.notify ? { ci: ui.notify.ci === true, behind: ui.notify.behind === true, pr: ui.notify.pr === true } : defaultNotify,
  }
}

export function toUI(s: SavedUI): UI {
  const f = s.filters
  return {
    mode: s.mode,
    theme: s.theme ?? '',
    filters: { registry: f.registry, host: f.host, sort: f.sort, dir: f.asc ? 'asc' : 'desc' },
    fetchPrune: s.fetchPrune,
    zoom: s.zoom,
    backgroundMinutes: s.backgroundMinutes,
    autoFetch: s.autoFetch,
    quitOnClose: s.quitOnClose,
    notify: s.notifications,
  }
}

const byId = (repos: Repository[]) => Object.fromEntries(repos.map((r) => [r.id, r]))

/** Серверные данные в репозитории, если получены для его текущей ветки или коммита (как withRemoteLocked в Go). */
export function withRemote(repo: Repository, st: RemoteStatus | undefined): Repository {
  if (!st || remoteRef(repo) !== st.branch) return repo
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
  // Фоновые данные (PR/CI, статус из фоновой проверки Go) приходят по одному
  // на репозиторий — в store они попадают пачкой, как и события сканирования.
  const remoteDirty = new Set<string>()
  const updated = new Map<string, Repository>()
  let remoteTimer: ReturnType<typeof setTimeout> | undefined
  const flushRemote = () => {
    remoteTimer = undefined
    const { repos, scanning } = store.getState()
    let next: Record<string, Repository> | undefined
    // Во время сканирования статус придёт с его итогом; убранные корни не возвращаются.
    for (const [id, repo] of scanning ? [] : updated) {
      if (repos[id]) (next ??= { ...repos })[id] = { ...repo, parentId: repo.parentId ?? repos[id].parentId }
    }
    updated.clear()
    for (const id of remoteDirty) {
      const cur = next?.[id] ?? repos[id]
      if (cur) (next ??= { ...repos })[id] = withRemote(cur, remote.get(id))
    }
    remoteDirty.clear()
    if (next) store.setState({ repos: next })
  }
  const queueRepository = (repo: Repository) => {
    updated.set(repo.id, repo)
    remoteDirty.add(repo.id)
    remoteTimer ??= setTimeout(flushRemote, scanFlushMs)
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
      fetchPrune: false,
      zoom: minZoom,
      backgroundMinutes: defaultBackgroundMinutes,
      autoFetch: false,
      quitOnClose: false,
      autostart: null,
      notifications: defaultNotify,
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
          // Окно Wails уже открыто с сохранённым масштабом; в превью его ставит CSS.
          source.setZoom(get().zoom).catch(fail)
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

      async loadDiagnostics() {
        try {
          return await source.getDiagnostics()
        } catch (err) {
          fail(err)
          return null
        }
      },

      async loadSkipDirs() {
        try {
          return await source.getSkipDirs()
        } catch (err) {
          fail(err)
          return null
        }
      },

      async saveSkipDirs(dirs) {
        let saved: SkipList
        try {
          saved = await source.setSkipDirs(dirs)
        } catch (err) {
          fail(err)
          return null
        }
        void get().scan()
        return saved
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

      setFetchPrune(fetchPrune) {
        set({ fetchPrune })
        persist()
      },

      setBackgroundMinutes(backgroundMinutes) {
        set({ backgroundMinutes })
        persist()
      },

      setAutoFetch(autoFetch) {
        set({ autoFetch })
        persist()
      },

      setQuitOnClose(quitOnClose) {
        set({ quitOnClose })
        persist()
      },

      async loadAutostart() {
        try {
          set({ autostart: await source.getAutostart() })
        } catch (err) {
          fail(err)
        }
      },

      async setAutostart(on) {
        try {
          await source.setAutostart(on)
        } catch (err) {
          fail(err)
        }
        await get().loadAutostart()
      },

      setNotifications(patch) {
        set({ notifications: { ...get().notifications, ...patch } })
        persist()
      },

      setZoom(value) {
        const zoom = clampZoom(value)
        if (zoom === get().zoom) return
        set({ zoom })
        source.setZoom(zoom).catch(fail)
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

  // Фоновая проверка Go (D-063): свежий статус по репозиториям и её время.
  source.onRepository(queueRepository)
  source.onChecked((lastCheckAt) => store.setState({ lastCheckAt, stale: false }))
  // Клик по уведомлению: репозиторий выбран и открыт в деталях.
  source.onReveal((id) => {
    if (store.getState().repos[id]) store.setState({ selectedId: id, detailsId: id })
  })

  // Сводка для значка в трее: пересчитывается, только когда меняется список.
  let summaryOf: AppState['repos'] | undefined
  let summary = ''
  store.subscribe(({ ready, repos }) => {
    if (!ready || repos === summaryOf) return
    summaryOf = repos
    const text = traySummary(Object.values(repos))
    if (text === summary) return
    summary = text
    source.setTraySummary(text).catch(() => undefined)
  })

  return store
}
