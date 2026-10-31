import { describe, expect, it } from 'vitest'
import { repo } from '../test/fixtures'
import { buildTree } from './explorer'

describe('дерево проводника', () => {
  it('корень → хост → namespace → репозиторий, без remote — в конце', () => {
    const roots = [
      { id: 'r1', path: '/srv/repos' },
      { id: 'empty', path: '/srv/empty' },
    ]
    const repos = [
      repo({ id: 'b', rootId: 'r1', host: 'gitlab.com', remotePath: 'group/sub/b' }),
      repo({ id: 'a2', rootId: 'r1', remotePath: 'team/zeta' }),
      repo({ id: 'a1', rootId: 'r1', remotePath: 'team/alpha' }),
      repo({ id: 'x', rootId: 'r1', remotePath: null, path: '/srv/repos/local' }),
      repo({ id: 'other-root', rootId: 'r2' }),
    ]
    const tree = buildTree(roots, repos)

    expect(tree).toHaveLength(2)
    expect(tree[1].hosts).toEqual([])
    const hosts = tree[0].hosts
    expect(hosts.map((h) => h.host)).toEqual(['github.com', 'gitlab.com', ''])
    expect(hosts[0].namespaces[0].namespace).toBe('team')
    expect(hosts[0].namespaces[0].repos.map((r) => r.id)).toEqual(['a1', 'a2'])
    expect(hosts[1].namespaces[0].namespace).toBe('group/sub')
    expect(hosts[2].namespaces[0].repos.map((r) => r.id)).toEqual(['x'])
  })
})
