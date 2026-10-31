// Дерево проводника: корень → хост → namespace → репозиторий. Хост и
// namespace берутся из выбранного remote, а не из имён папок.

import type { Repository, Root } from '../data/model'
import { repoHost, repoName, repoNamespace } from './repo'

export interface NamespaceNode {
  namespace: string
  repos: Repository[]
}

export interface HostNode {
  /** пусто — репозитории без remote */
  host: string
  namespaces: NamespaceNode[]
}

export interface RootNode {
  root: Root
  hosts: HostNode[]
}

const byText = (a: string, b: string) => a.localeCompare(b, 'ru')

export function buildTree(roots: Root[], repos: Repository[]): RootNode[] {
  return roots.map((root) => {
    const hosts = new Map<string, Map<string, Repository[]>>()
    for (const r of repos) {
      if (r.rootId !== root.id) continue
      const host = repoHost(r)
      const namespace = repoNamespace(r)
      const namespaces = hosts.get(host) ?? new Map<string, Repository[]>()
      hosts.set(host, namespaces)
      namespaces.set(namespace, [...(namespaces.get(namespace) ?? []), r])
    }
    return {
      root,
      hosts: [...hosts]
        // хост без remote — в конце
        .sort(([a], [b]) => (!a ? 1 : !b ? -1 : byText(a, b)))
        .map(([host, namespaces]) => ({
          host,
          namespaces: [...namespaces]
            .sort(([a], [b]) => byText(a, b))
            .map(([namespace, list]) => ({
              namespace,
              repos: [...list].sort((a, b) => byText(repoName(a), repoName(b)) || byText(a.path, b.path)),
            })),
        })),
    }
  })
}
