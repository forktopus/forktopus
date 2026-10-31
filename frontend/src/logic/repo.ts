import { Provider, type RemoteInfo, type Repository } from '../data/model'

export function selectedRemote(r: Repository): RemoteInfo | undefined {
  return r.remotes?.find((x) => x.name === r.remote)
}

/** host[:port] выбранного remote; пусто — remote нет или он не выбран. */
export function repoHost(r: Repository): string {
  return selectedRemote(r)?.host ?? ''
}

export function repoProvider(r: Repository): Provider {
  return selectedRemote(r)?.provider || Provider.ProviderOther
}

function segments(r: Repository): string[] {
  const remotePath = selectedRemote(r)?.path
  return (remotePath || r.path).split('/').filter(Boolean)
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
