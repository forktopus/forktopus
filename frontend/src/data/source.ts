import { ErrorCode, type Error as ModelError, type Repository, type Root, type ToolDiagnostic, type UI } from './model'

/**
 * Источник данных UI. Методы повторяют AppService из internal/app; списки
 * всегда массивы (null из Go превращается в []). Подписка на события
 * появится, когда Go начнёт их отправлять (сканирование, этап 05).
 */
export interface DataSource {
  readonly kind: 'wails' | 'mock'
  listRoots(): Promise<Root[]>
  addRoot(path: string): Promise<Root>
  removeRoot(id: string): Promise<void>
  listRepositories(): Promise<Repository[]>
  checkTools(): Promise<ToolDiagnostic[]>
  getUI(): Promise<UI>
  saveUI(ui: UI): Promise<void>
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
