import { Availability, type CISummary, type PRSummary, type Repository } from '../data/model'

/** Веб-ссылки приходят из Go с учётом base path и SSH-псевдонимов. Старый
 * кэш без listUrl не заменяем адресом случайного PR или догадкой по remote. */
export function serverURL(value?: string): string | undefined {
  if (!value) return undefined
  try {
    const url = new URL(value)
    return ['https:', 'http:'].includes(url.protocol) && !url.username && !url.password ? value : undefined
  } catch {
    return undefined
  }
}

export function remoteListURL(summary: PRSummary | CISummary): string | undefined {
  return summary.availability === Availability.AvailabilityOK ? serverURL(summary.listUrl) : undefined
}

/** Номер последнего закрытого PR остаётся ссылкой на этот PR. Число открытых
 * PR и подпись CI ведут к спискам; отдельные значки сохраняют прежние цели. */
export function indicatorURL(repo: Repository, kind: 'pr' | 'ci'): string | undefined {
  if (kind === 'pr' && repo.pr.availability === Availability.AvailabilityOK && repo.pr.open === 0 && repo.pr.last) return serverURL(repo.pr.last.url)
  return remoteListURL(repo[kind])
}
