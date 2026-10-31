import { useMemo } from 'react'
import type { Repository } from '../data/model'
import { filterRepos, sortRepos } from '../logic/registry'
import { useApp } from './context'

/** Все репозитории в стабильном порядке (по пути). */
export function useRepos(): Repository[] {
  const repos = useApp((s) => s.repos)
  return useMemo(() => Object.values(repos).sort((a, b) => a.path.localeCompare(b.path, 'ru')), [repos])
}

/** Репозитории с учётом глобального поиска — общая выборка всех режимов. */
export function useSearchedRepos(): Repository[] {
  const repos = useRepos()
  const query = useApp((s) => s.query)
  return useMemo(() => filterRepos(repos, { filter: 'all', host: '', query }), [repos, query])
}

/** Выборка реестра: поиск, фильтр, хост, сортировка. */
export function useRegistryRepos(): Repository[] {
  const repos = useRepos()
  const query = useApp((s) => s.query)
  const filters = useApp((s) => s.filters)
  return useMemo(
    () => sortRepos(filterRepos(repos, { filter: filters.registry, host: filters.host, query }), filters.sort, filters.asc),
    [repos, query, filters],
  )
}
