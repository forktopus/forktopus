// Построитель Repository для тестов: чистый репозиторий на github.com с
// upstream и доступными PR/CI; сценарий задаётся переопределениями.

import { Availability, Provider, type Repository } from '../data/model'

export interface RepoOverrides {
  id?: string
  rootId?: string
  path?: string
  host?: string
  remotePath?: string | null
  branch?: Partial<Repository['branch']>
  worktree?: Partial<Repository['worktree']>
  pr?: Partial<Repository['pr']>
  ci?: Partial<Repository['ci']>
  error?: Repository['error']
}

export function repo(o: RepoOverrides = {}): Repository {
  const remotePath = o.remotePath === undefined ? 'owner/project' : o.remotePath
  const path = o.path ?? `/srv/repos/${o.host ?? 'github.com'}/${remotePath ?? 'local'}`
  return {
    id: o.id ?? path,
    rootId: o.rootId ?? 'root',
    path,
    gitDir: `${path}/.git`,
    remotes:
      remotePath === null
        ? null
        : [
            {
              name: 'origin',
              fetchUrl: `git@${o.host ?? 'github.com'}:${remotePath}.git`,
              host: o.host ?? 'github.com',
              path: remotePath,
              provider: Provider.ProviderGitHub,
            },
          ],
    remote: remotePath === null ? undefined : 'origin',
    branch: { name: 'main', detached: false, upstream: 'origin/main', ahead: 0, behind: 0, ...o.branch },
    worktree: { staged: 0, unstaged: 0, untracked: 0, conflicts: 0, ...o.worktree },
    pr: { availability: Availability.AvailabilityOK, open: 0, draft: 0, ...o.pr },
    ci: { availability: Availability.AvailabilityOK, active: 0, lastResult: 'success', ...o.ci },
    error: o.error ?? null,
    updatedAt: '2026-10-31T12:00:00Z',
  }
}

/** Источник PR/CI недоступен (нет CLI или авторизации): данных нет. */
export const unavailable = { availability: Availability.AvailabilityUnavailable, open: null, active: null }
