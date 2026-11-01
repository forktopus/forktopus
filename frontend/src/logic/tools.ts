// Подсказки диагностики CLI: что не так и что выполнить самому. Приложение
// не устанавливает CLI и не авторизуется за пользователя (idea 05 §2).

import { Availability, ErrorCode, type ToolDiagnostic } from '../data/model'
import { t } from '../i18n'
import type { Tone } from '../theme/theme'

export const availabilityTone: Record<Availability, Tone> = {
  [Availability.$zero]: 'muted',
  [Availability.AvailabilityOK]: 'good',
  [Availability.AvailabilityUnavailable]: 'warn',
  [Availability.AvailabilityError]: 'bad',
  [Availability.AvailabilityNotSupported]: 'muted',
}

export interface ToolHint {
  text: string
  /** команда для терминала; пусто — только текст */
  command?: string
  /** страница установки — ссылка под текстом */
  url?: string
}

/** Разделы установки: на каждой странице — способы для Linux, Windows и macOS. */
export const installURL: Record<string, string> = {
  git: 'https://git-scm.com/downloads',
  gh: 'https://github.com/cli/cli#installation',
  glab: 'https://gitlab.com/gitlab-org/cli/-/blob/main/docs/installation_options.md',
  gt: 'https://gitea.com/gitea/cli#installation',
  tea: 'https://gitea.com/gitea/tea#installation',
}

function loginCommand(name: string, host: string): string | undefined {
  switch (name) {
    case 'gh':
      return `gh auth login --hostname ${host}`
    case 'glab':
      return `glab auth login --hostname ${host}`
    case 'gt':
      return `gt auth login --host https://${host}`
    case 'tea':
      return `tea login add --url https://${host}`
  }
  return undefined
}

export function toolHint(d: ToolDiagnostic): ToolHint | null {
  if (d.availability === Availability.AvailabilityOK) return null
  const host = d.host ?? ''
  switch (d.error?.code) {
    case ErrorCode.CodeCLIMissing:
      return { text: t('tools.install', { name: d.name }), url: installURL[d.name] }
    case ErrorCode.CodeAuthRequired:
      return host ? { text: t('tools.login', { host }), command: loginCommand(d.name, host) } : { text: t('tools.loginAny') }
    case ErrorCode.CodeNetworkError:
    case ErrorCode.CodeTimeout:
      return { text: t('tools.offline', { host: host || d.name }) }
  }
  return null
}

/** Команды для самостоятельной проверки установленных CLI. */
export const selfCheckCommands = ['git --version', 'gh auth status', 'glab auth status', 'gt auth status'].join('\n')

/**
 * Git не готов (A28): без него не прочитать ни одного статуса. null — git в
 * порядке или проверка ещё не выполнялась.
 */
export function gitProblem(tools: ToolDiagnostic[]): ToolDiagnostic | null {
  const git = tools.find((d) => d.name === 'git')
  return git && git.availability !== Availability.AvailabilityOK ? git : null
}
