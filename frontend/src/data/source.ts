import {
  type BranchInfo,
  type Diagnostics,
  ErrorCode,
  type Error as ModelError,
  type OpExpect,
  type OpRecord,
  type OpResult,
  type RemoteStatus,
  type Repository,
  type RepositorySnapshot,
  type Root,
  type ScanIssue,
  type ScanResult,
  type ToolDiagnostic,
  type UI,
} from './model'

type Lists = { repositories: Repository[]; issues: ScanIssue[] }
/** RepositorySnapshot без null-массивов. */
export type Snapshot = Omit<RepositorySnapshot, keyof Lists> & Lists
/** SkipDirs из Go без null-массивов. */
export type SkipList = { dirs: string[]; defaults: string[] }
/** ScanResult без null-массивов. */
export type ScanOutcome = Omit<ScanResult, keyof Lists> & Lists

/**
 * Источник данных UI. Методы повторяют AppService из internal/app; списки
 * всегда массивы (null из Go превращается в []).
 */
export interface DataSource {
  readonly kind: 'wails' | 'mock'
  listRoots(): Promise<Root[]>
  addRoot(path: string): Promise<Root>
  removeRoot(id: string): Promise<void>
  /** Имена каталогов, в которые сканер не заходит, и значения по умолчанию (D-024). */
  getSkipDirs(): Promise<SkipList>
  /** Сохраняет список пропусков; пустой — значения по умолчанию. Действует со следующего сканирования. */
  setSkipDirs(dirs: string[]): Promise<SkipList>
  /** Пути каталогов, в которые сканер не заходит (D-067). */
  getSkipPaths(): Promise<string[]>
  /** Сохраняет пути-исключения; проблемы внутри них убираются сразу. Возвращает оставшиеся проблемы сканирования. */
  setSkipPaths(paths: string[]): Promise<ScanIssue[]>
  /** Системный диалог выбора папки; '' — пользователь его закрыл. */
  pickFolder(): Promise<string>
  /** Известные репозитории; до первого сканирования — кэш прошлого запуска (stale). */
  listRepositories(): Promise<Snapshot>
  /** Сканирует корни; найденные по ходу репозитории приходят в onScanRepository. */
  scanRoots(): Promise<ScanOutcome>
  selectRemote(repoId: string, remote: string): Promise<void>
  /** Локальные ветки: upstream, ahead/behind, занятые другими worktree. */
  getBranches(repoId: string): Promise<BranchInfo[]>
  /** Перечитывает локальный статус одного репозитория. */
  refreshRepo(repoId: string): Promise<Repository>
  /**
   * Безопасные операции. expect — ветка и HEAD, которые видел пользователь (D-032).
   * Блокировка по предусловиям и сбой Git приходят в result.error вместе со свежим статусом.
   */
  fetchRepo(repoId: string, expect: OpExpect): Promise<OpResult>
  /** Pull только fast-forward. */
  pullRepo(repoId: string, expect: OpExpect): Promise<OpResult>
  switchBranch(repoId: string, branch: string, expect: OpExpect): Promise<OpResult>
  /** PR/MR и CI текущей ветки; свежие данные — из кэша Go (D-046). */
  getRemoteStatus(repoId: string): Promise<RemoteStatus>
  /** То же мимо кэша: ручное обновление. */
  refreshRemote(repoId: string): Promise<RemoteStatus>
  /** Журнал последних операций, новые — первыми. */
  listOperations(): Promise<OpRecord[]>
  checkTools(): Promise<ToolDiagnostic[]>
  /** Отчёт диагностики: ОС, пути, утилиты, последние ошибки и операции — без секретов. */
  getDiagnostics(): Promise<Diagnostics>
  /** Открыть ссылку (PR, CI) в браузере системы. */
  openURL(url: string): Promise<void>
  copyText(text: string): Promise<void>
  /** Каталог в файловом менеджере системы; нет каталога — ближайший существующий выше (D-066). */
  openFolder(path: string): Promise<void>
  /** Терминал системы в каталоге; '' — домашний каталог (D-066). */
  openTerminal(path: string): Promise<void>
  getUI(): Promise<UI>
  saveUI(ui: UI): Promise<void>
  /** Запуск при входе в систему: состояние из системы; null — не поддерживается (D-063). */
  getAutostart(): Promise<boolean | null>
  setAutostart(on: boolean): Promise<void>
  /** Пути репозиториев без автоматической проверки (D-064). */
  getNoAutoCheck(): Promise<string[]>
  /** Включить или выключить автоматическую проверку; возвращает новый список путей. */
  setAutoCheck(path: string, on: boolean): Promise<string[]>
  /** Сводка для подсказки значка в трее (D-063). */
  setTraySummary(text: string): Promise<void>
  /** Масштаб интерфейса окна (1 — 100 %). */
  setZoom(zoom: number): Promise<void>
  /** Подписка на найденные во время сканирования репозитории; возвращает отписку. */
  onScanRepository(cb: (repo: Repository) => void): () => void
  /** Подписка на серверные данные веток, полученные в фоне; возвращает отписку. */
  onRemoteStatus(cb: (status: RemoteStatus) => void): () => void
  /** Подписка на статус репозитория, перечитанный фоновой проверкой Go (D-063). */
  onRepository(cb: (repo: Repository) => void): () => void
  /** Подписка на окончание фоновой проверки; at — её время (ISO). */
  onChecked(cb: (at: string) => void): () => void
  /** Подписка на просьбу показать репозиторий (клик по уведомлению). */
  onReveal(cb: (repoId: string) => void): () => void
}

/** Ошибка вызова в виде model.Error: Wails кладёт её JSON в err.cause. */
export function errorOf(err: unknown): ModelError {
  const cause = (err as { cause?: unknown } | null)?.cause
  if (cause && typeof cause === 'object' && 'code' in cause && 'message' in cause) {
    return cause as ModelError
  }
  const message = err instanceof globalThis.Error ? err.message : String(err)
  return { code: ErrorCode.CodeOther, message }
}

export function normalizeSkipDirs(d: { dirs: string[] | null; defaults: string[] | null }): SkipList {
  return { dirs: d.dirs ?? [], defaults: d.defaults ?? [] }
}

export function normalizeOpResult(r: OpResult): OpResult {
  return { ...r, repo: normalizeRepository(r.repo) }
}

/** Приводит null-массивы из Go к пустым. */
export function normalizeRepository(r: Repository): Repository {
  return { ...r, remotes: r.remotes ?? [] }
}

export function normalizeSnapshot<T extends { repositories: Repository[] | null; issues: ScanIssue[] | null }>(s: T): Omit<T, keyof Lists> & Lists {
  return { ...s, repositories: (s.repositories ?? []).map(normalizeRepository), issues: s.issues ?? [] }
}
