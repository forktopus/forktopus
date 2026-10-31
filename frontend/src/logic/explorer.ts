// Дерево проводника: корень → хост → namespace → репозиторий. Хост и
// namespace берутся из выбранного remote, а не из имён папок; namespace
// любой глубины (group/subgroup/…, A03) — один узел. Вложенные репозитории
// (D-025) — узлы под родителем, а не под своим хостом.

import { Provider, type Repository, type Root, type ScanIssue } from '../data/model'
import type { Tone } from '../theme/theme'
import { radarGroup } from './radar'
import { repoHost, repoName, repoNamespace, repoProvider } from './repo'
import { integrationIssue } from './status'

export interface RepoNode {
  repo: Repository
  children: RepoNode[]
}

export interface NamespaceNode {
  namespace: string
  repos: RepoNode[]
}

export interface HostNode {
  /** пусто — репозитории без remote */
  host: string
  provider: Provider
  namespaces: NamespaceNode[]
  /** у части репозиториев хоста нет данных PR/CI (нет CLI, авторизации, связи) */
  issue: boolean
}

export interface RootNode {
  root: Root
  hosts: HostNode[]
}

const byText = (a: string, b: string) => a.localeCompare(b, 'ru')
const byRepo = (a: Repository, b: Repository) => byText(repoName(a), repoName(b)) || byText(a.path, b.path)

export function buildTree(roots: Root[], repos: Repository[]): RootNode[] {
  const shown = new Set(repos.map((r) => r.id))
  const kids = new Map<string, Repository[]>()
  for (const r of repos) {
    if (r.parentId && shown.has(r.parentId)) kids.set(r.parentId, [...(kids.get(r.parentId) ?? []), r])
  }
  const node = (r: Repository): RepoNode => ({ repo: r, children: [...(kids.get(r.id) ?? [])].sort(byRepo).map(node) })

  return roots.map((root) => {
    const hosts = new Map<string, Map<string, Repository[]>>()
    for (const r of repos) {
      // вложенный, чей родитель в выборке, показывается под родителем
      if (r.rootId !== root.id || (r.parentId && shown.has(r.parentId))) continue
      const host = repoHost(r)
      const namespaces = hosts.get(host) ?? new Map<string, Repository[]>()
      hosts.set(host, namespaces)
      const namespace = repoNamespace(r)
      namespaces.set(namespace, [...(namespaces.get(namespace) ?? []), r])
    }
    return {
      root,
      hosts: [...hosts]
        // хост без remote — в конце
        .sort(([a], [b]) => (!a ? 1 : !b ? -1 : byText(a, b)))
        .map(([host, namespaces]) => {
          const all = [...namespaces.values()].flat()
          return {
            host,
            provider: host ? repoProvider(all[0]) : Provider.$zero,
            issue: all.some(integrationIssue),
            namespaces: [...namespaces]
              .sort(([a], [b]) => byText(a, b))
              .map(([namespace, list]) => ({ namespace, repos: [...list].sort(byRepo).map(node) })),
          }
        }),
    }
  })
}

/** Репозитории дерева в порядке показа (выбор по умолчанию — первый). */
export function treeRepos(tree: RootNode[]): Repository[] {
  const out: Repository[] = []
  const walk = (n: RepoNode) => {
    out.push(n.repo)
    n.children.forEach(walk)
  }
  for (const root of tree) for (const h of root.hosts) for (const ns of h.namespaces) ns.repos.forEach(walk)
  return out
}

/** Цвет точки у репозитория в дереве — по группе радара; серый — статус не прочитан. */
export function healthTone(r: Repository): Tone {
  const group = radarGroup(r)
  return group === 'attention' ? 'bad' : group === 'progress' ? 'warn' : group === 'ok' ? 'good' : 'muted'
}

/**
 * Корни, в которых сканирование не нашло ни одного репозитория. Корень, который
 * не удалось открыть (нет папки, нет прав), — не «пустой»: он среди ошибок скана.
 */
export function emptyRoots(roots: Root[], repos: Repository[], issues: ScanIssue[] = []): Root[] {
  const used = new Set(repos.map((r) => r.rootId))
  const broken = new Set(issues.filter((i) => roots.some((r) => r.id === i.rootId && r.path === i.path)).map((i) => i.rootId))
  return roots.filter((r) => !used.has(r.id) && !broken.has(r.id))
}
