import { describe, expect, it } from 'vitest'
import { repo, unavailable } from '../test/fixtures'
import { buildTree, healthTone, treeRepos } from './explorer'

describe('дерево проводника', () => {
  it('корень → хост → namespace → репозиторий, без remote — в конце', () => {
    const roots = [
      { id: 'r1', path: '/srv/repos' },
      { id: 'empty', path: '/srv/empty' },
    ]
    const repos = [
      repo({ id: 'b', rootId: 'r1', host: 'gitlab.com', remotePath: 'group/sub/b', pr: unavailable }),
      repo({ id: 'a2', rootId: 'r1', remotePath: 'team/zeta' }),
      repo({ id: 'a1', rootId: 'r1', remotePath: 'team/alpha' }),
      repo({ id: 'x', rootId: 'r1', remotePath: null, path: '/srv/repos/local' }),
      repo({ id: 'other-root', rootId: 'r2' }),
    ]
    const tree = buildTree(roots, repos)

    expect(tree).toHaveLength(2)
    expect(tree[1].hosts).toEqual([])
    const hosts = tree[0].hosts
    expect(hosts.map((h) => [h.host, h.issue])).toEqual([
      ['github.com', false],
      ['gitlab.com', true],
      ['', false],
    ])
    expect(hosts[0].namespaces[0].namespace).toBe('team')
    expect(hosts[0].namespaces[0].repos.map((n) => n.repo.id)).toEqual(['a1', 'a2'])
    expect(hosts[2].namespaces[0].repos.map((n) => n.repo.id)).toEqual(['x'])
  })

  it('namespace глубже двух уровней — один узел (A03)', () => {
    const deep = repo({ id: 'd', host: 'git.my-company.local', remotePath: 'group/subgroup/team/repo' })
    const [root] = buildTree([{ id: 'root', path: '/srv' }], [deep])
    expect(root.hosts[0].host).toBe('git.my-company.local')
    expect(root.hosts[0].namespaces[0].namespace).toBe('group/subgroup/team')
  })

  it('вложенные — под родителем, даже с другим хостом; без родителя в выборке — на своём месте (D-025)', () => {
    const parent = repo({ id: 'p', remotePath: 'team/p' })
    const sub = { ...repo({ id: 's', host: 'gitlab.com', remotePath: 'lib/s' }), parentId: 'p' }
    const deep = { ...repo({ id: 'd', remotePath: 'team/d' }), parentId: 's' }
    const root = { id: 'root', path: '/srv' }

    const tree = buildTree([root], [deep, sub, parent])
    expect(tree[0].hosts.map((h) => h.host)).toEqual(['github.com'])
    const [p] = tree[0].hosts[0].namespaces[0].repos
    expect(p.children.map((n) => n.repo.id)).toEqual(['s'])
    expect(p.children[0].children.map((n) => n.repo.id)).toEqual(['d'])
    expect(treeRepos(tree).map((r) => r.id)).toEqual(['p', 's', 'd'])

    expect(buildTree([root], [sub])[0].hosts.map((h) => h.host)).toEqual(['gitlab.com'])
  })
})

describe('цвет состояния в дереве', () => {
  it('по группе радара, без статуса — серый', () => {
    expect(healthTone(repo())).toBe('good')
    expect(healthTone(repo({ worktree: { unstaged: 1 } }))).toBe('warn')
    expect(healthTone(repo({ worktree: { conflicts: 1 } }))).toBe('bad')
    expect(healthTone({ ...repo(), branch: null })).toBe('muted')
  })
})
