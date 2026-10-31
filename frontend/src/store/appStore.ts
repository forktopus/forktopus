// Единый store приложения. Режимы читают одни и те же данные, поэтому
// переключение режима не перечитывает репозитории (A26). Режим, тема и
// фильтры сохраняются в настройках Go через DataSource.saveUI.

import { createStore, type StoreApi } from 'zustand/vanilla'
import type { Repository, Root, ToolDiagnostic, UI } from '../data/model'
import { errorOf, type DataSource } from '../data/source'
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
  ready: boolean
  refreshing: boolean
  roots: Root[]
  repos: Record<string, Repository>
  tools: ToolDiagnostic[]
  toolsCheckedAt: string | null
  lastCheckAt: string | null
  mode: Mode
  theme: ThemeChoice
  filters: Filters
  query: string
  selectedId: string | null
  /** последняя ошибка для показа пользователю */
  error: string | null

  init(): Promise<void>
  refresh(): Promise<void>
  checkTools(): Promise<void>
  addRoot(path: string): Promise<boolean>
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

export function createAppStore(source: DataSource): AppStore {
  return createStore<AppState>()((set, get) => {
    const fail = (err: unknown) => set({ error: errorOf(err).message })
    const persist = () => {
      source.saveUI(toUI(get())).catch(fail)
    }

    return {
      sourceKind: source.kind,
      ready: false,
      refreshing: false,
      roots: [],
      repos: {},
      tools: [],
      toolsCheckedAt: null,
      lastCheckAt: null,
      mode: 'registry',
      theme: null,
      filters: defaultFilters,
      query: '',
      selectedId: null,
      error: null,

      async init() {
        try {
          set(fromUI(await source.getUI()))
        } catch (err) {
          fail(err)
        }
        await Promise.all([get().refresh(), get().checkTools()])
        set({ ready: true })
      },

      async refresh() {
        if (get().refreshing) return
        set({ refreshing: true })
        try {
          const [roots, repos] = await Promise.all([source.listRoots(), source.listRepositories()])
          set({
            roots,
            repos: Object.fromEntries(repos.map((r) => [r.id, r])),
            lastCheckAt: new Date().toISOString(),
          })
        } catch (err) {
          fail(err)
        } finally {
          set({ refreshing: false })
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
        } catch (err) {
          fail(err)
          return false
        }
        await get().refresh()
        return true
      },

      async removeRoot(id) {
        try {
          await source.removeRoot(id)
        } catch (err) {
          fail(err)
          return
        }
        await get().refresh()
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
}
