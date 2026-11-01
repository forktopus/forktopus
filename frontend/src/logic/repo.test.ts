import { describe, expect, it } from 'vitest'
import { Provider, type Repository } from '../data/model'
import { branchOf, compareText, parentPath, remoteRef, repoHost, repoName, repoNamespace, repoProvider } from './repo'

const repo = (over: Partial<Repository>): Repository => ({ id: 'r', path: '/srv/repos/Мой проект', remotes: [], ...over }) as Repository

describe('repo', () => {
  it('имя и группа берутся из пути на сервере, вложенность не ограничена (A03)', () => {
    const r = repo({
      remote: 'origin',
      remotes: [{ name: 'origin', host: 'git.example.org:8443', path: 'group/sub/deeper/project', provider: Provider.ProviderGitLab }] as Repository['remotes'],
    })
    expect(repoName(r)).toBe('project')
    expect(repoNamespace(r)).toBe('group/sub/deeper')
    expect(repoHost(r)).toBe('git.example.org:8443')
    expect(repoProvider(r)).toBe(Provider.ProviderGitLab)
  })

  it('без remote — имя каталога, группы и хоста нет, провайдер «другой»', () => {
    const r = repo({})
    expect(repoName(r)).toBe('Мой проект')
    expect(repoNamespace(r)).toBe('')
    expect(repoHost(r)).toBe('')
    expect(repoProvider(r)).toBe(Provider.ProviderOther)
  })

  it('путь Windows — имя каталога без диска и родителей', () => {
    expect(repoName(repo({ path: 'C:\\fk\\.local\\git-fixtures\\gitlab.com\\team\\demo-ahead' }))).toBe('demo-ahead')
  })

  it('выбранный remote, которого нет в списке, не подменяется первым', () => {
    const r = repo({
      remote: 'upstream',
      remotes: [{ name: 'origin', host: 'github.com', path: 'o/r', provider: Provider.ProviderGitHub }] as Repository['remotes'],
    })
    expect(repoHost(r)).toBe('')
    expect(repoName(r)).toBe('Мой проект')
  })

  it('detached HEAD и непрочитанный статус — пустая ветка (A10)', () => {
    expect(branchOf(repo({ branch: { name: 'main', detached: false } as Repository['branch'] }))).toBe('main')
    expect(branchOf(repo({ branch: { name: 'main', detached: true, headSha: 'abc' } as Repository['branch'] }))).toBe('')
    expect(branchOf(repo({ branch: undefined }))).toBe('')
  })

  it('серверные данные detached HEAD относятся к коммиту (@SHA)', () => {
    expect(remoteRef(repo({ branch: { name: 'main', detached: false, headSha: 'abc' } as Repository['branch'] }))).toBe('main')
    expect(remoteRef(repo({ branch: { name: '', detached: true, headSha: 'abc' } as Repository['branch'] }))).toBe('@abc')
    expect(remoteRef(repo({ branch: { name: '', detached: true } as Repository['branch'] }))).toBe('')
  })

  it('сортировка по-русски: ё рядом с е, регистр не разносит', () => {
    expect(['ж', 'ёж', 'Еда', 'а', 'Б'].sort(compareText)).toEqual(['а', 'Б', 'Еда', 'ёж', 'ж'])
  })

  it('родительский каталог: Unix, Windows, корень', () => {
    expect(parentPath('/srv/repos/locked/.harness')).toBe('/srv/repos/locked')
    expect(parentPath('/srv/repos/')).toBe('/srv')
    expect(parentPath('/srv')).toBe('/')
    expect(parentPath('/')).toBe('/')
    expect(parentPath('C:\\Users\\me\\repo')).toBe('C:\\Users\\me')
    expect(parentPath('C:\\Users')).toBe('C:\\')
  })
})
