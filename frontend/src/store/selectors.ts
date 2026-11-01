import { useMemo } from 'react'
import type { Repository } from '../data/model'
import { filterRepos, sortRepos } from '../logic/registry'
import { compareText, tracked } from '../logic/repo'
import { useApp } from './context'

/** Все найденные репозитории в стабильном порядке (по пути), включая скрытые настройкой. */
export function useAllRepos(): Repository[] {
  const repos = useApp((s) => s.repos)
  return useMemo(() => Object.values(repos).sort((a, b) => compareText(a.path, b.path)), [repos])
}

/** Отслеживаемые репозитории — основа всех режимов и счётчиков. */
export function useRepos(): Repository[] {
  const all = useAllRepos()
  const hideLocal = useApp((s) => s.hideLocal)
  return useMemo(() => tracked(all, hideLocal), [all, hideLocal])
}

/** Репозитории с учётом глобального поиска — общая выборка всех режимов. */
export function useSearchedRepos(): Repository[] {
  const repos = useRepos()
  const query = useApp((s) => s.query)
  return useMemo(() => filterRepos(repos, { filter: 'all', host: '', query }), [repos, query])
}

/** Поиск, вкладка статуса и хост — общая выборка реестра и радара. */
export function useFilteredRepos(): Repository[] {
  const repos = useRepos()
  const query = useApp((s) => s.query)
  const filter = useApp((s) => s.filters.registry)
  const host = useApp((s) => s.filters.host)
  return useMemo(() => filterRepos(repos, { filter, host, query }), [repos, query, filter, host])
}

/** Выборка реестра: поиск, фильтр, хост, сортировка. */
export function useRegistryRepos(): Repository[] {
  const list = useFilteredRepos()
  const sort = useApp((s) => s.filters.sort)
  const asc = useApp((s) => s.filters.asc)
  return useMemo(() => sortRepos(list, sort, asc), [list, sort, asc])
}
