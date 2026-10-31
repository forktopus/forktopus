// Единый store приложения. Режимы читают одни и те же данные, поэтому
// переключение режима не перечитывает репозитории (A26). Режим, тема и
// фильтры сохраняются в настройках Go через DataSource.saveUI.

import { createStore, type StoreApi } from 'zustand/vanilla'
import type { Repository, Root, ScanIssue, ToolDiagnostic, UI } from '../data/model'
import { errorOf, type DataSource, type Snapshot } from '../data/source'
import { registryFilters, sortKeys, type RegistryFilter, type SortKey } from '../logic/registry'
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

export interface AppState {
  sourceKind: DataSource['kind']
  /** первые данные (корни и кэш репозиториев) загружены */
  ready: boolean
  scanning: boolean
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
  /** последняя ошибка для показа пользователю */
  error: string | null

  init(): Promise<void>
  /** перечитать корни и пересканировать */
  refresh(): Promise<void>
  scan(): Promise<void>
  checkTools(): Promise<void>
  addRoot(path: string): Promise<boolean>
  /** системный диалог выбора папки + addRoot; false — диалог закрыт или ошибка */
  pickAndAddRoot(): Promise<boolean>
  removeRoot(id: string): Promise<void>
  setMode(mode: Mode): void
  setTheme(theme: ThemeMode): void
  setFilters(patch: Partial<Filters>): void
  setQuery(query: string): void
  select(id: string | null): void
  clearError(): void
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

export function createAppStore(source: DataSource): AppStore {
  const store = createStore<AppState>()((set, get) => {
    const fail = (err: unknown) => set({ error: errorOf(err).message })
    const persist = () => {
      source.saveUI(toUI(get())).catch(fail)
    }
    const applySnapshot = (snap: Snapshot) =>
      set({ repos: byId(snap.repositories), issues: snap.issues, stale: snap.stale, lastCheckAt: snap.scannedAt ?? null })

    return {
      sourceKind: source.kind,
      ready: false,
      scanning: false,
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
      error: null,

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
        set({ scanning: true })
        try {
          const res = await source.scanRoots()
          set({ repos: byId(res.repositories), issues: res.issues, stale: false, lastCheckAt: res.scannedAt })
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
      clearError: () => set({ error: null }),
    }
  })

  // Найденные по ходу сканирования репозитории сразу видны в списке. В событии
  // ещё нет parentId — берём из прежней записи; итог сканирования всё заменит.
  source.onScanRepository((repo) => {
    const { scanning, repos } = store.getState()
    if (!scanning) return
    const prev = repos[repo.id]
    store.setState({ repos: { ...repos, [repo.id]: { ...repo, parentId: repo.parentId ?? prev?.parentId } } })
  })

  return store
}
