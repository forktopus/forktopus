import { Provider, type RemoteInfo, type Repository } from '../data/model'

// Один коллатор на всё приложение: localeCompare с локалью создаёт его на
// каждое сравнение, и сортировка сотен репозиториев заметно тормозит UI.
const collator = new Intl.Collator('ru')

/** Сравнение строк по-русски для сортировки. */
export const compareText = (a: string, b: string): number => collator.compare(a, b)

export function selectedRemote(r: Repository): RemoteInfo | undefined {
  return r.remotes?.find((x) => x.name === r.remote)
}

/** host[:port] выбранного remote; пусто — remote нет или он не выбран. */
export function repoHost(r: Repository): string {
  return selectedRemote(r)?.host ?? ''
}

/** Текущая ветка, как её понимает Go (branchOf); пусто — detached HEAD или статус не прочитан. */
export function branchOf(r: Repository): string {
  return !r.branch || r.branch.detached ? '' : (r.branch.name ?? '')
}

/** К чему относятся серверные данные, как remoteRef в Go: ветка или `@SHA` при detached HEAD. */
export function remoteRef(r: Repository): string {
  return r.branch?.detached && r.branch.headSha ? `@${r.branch.headSha}` : branchOf(r)
}

export function repoProvider(r: Repository): Provider {
  return selectedRemote(r)?.provider || Provider.ProviderOther
}

function segments(r: Repository): string[] {
  const remotePath = selectedRemote(r)?.path
  // Локальный путь на Windows — с обратными слэшами.
  return (remotePath || r.path).split(/[\\/]/).filter(Boolean)
}

/** Имя проекта: последний сегмент пути на сервере, иначе имя каталога. */
export function repoName(r: Repository): string {
  return segments(r).at(-1) ?? r.path
}

/** Владелец/группа на сервере; для репозитория без remote — пусто. */
export function repoNamespace(r: Repository): string {
  if (!selectedRemote(r)?.path) return ''
  return segments(r).slice(0, -1).join('/')
}

/** Отслеживаемые: с настройкой hideLocal — только с remote (D-070). */
export function tracked(repos: Repository[], hideLocal: boolean): Repository[] {
  return hideLocal ? repos.filter((r) => (r.remotes?.length ?? 0) > 0) : repos
}

/** path совпадает с dir или лежит внутри него (как within в Go); «/» и «\». */
export function within(path: string, dir: string): boolean {
  const d = dir.replace(/[/\\]+$/, '')
  return path === d || path.startsWith(d + '/') || path.startsWith(d + '\\')
}

/** Родительский каталог: «/» и «\» (Windows); у корня ФС — он сам. */
export function parentPath(path: string): string {
  const trimmed = path.replace(/[/\\]+$/, '')
  const i = Math.max(trimmed.lastIndexOf('/'), trimmed.lastIndexOf('\\'))
  if (i < 0) return path
  const parent = trimmed.slice(0, i)
  // «/srv» → «/», «C:\Users» → «C:\»
  return parent === '' || /^[A-Za-z]:$/.test(parent) ? trimmed.slice(0, i + 1) : parent
}
