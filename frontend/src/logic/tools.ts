// Подсказки диагностики CLI: что не так и что выполнить самому. Приложение
// не устанавливает CLI и не авторизуется за пользователя (idea 05 §2).

import { Availability, ErrorCode, type ToolDiagnostic } from '../data/model'
import { t } from '../i18n'
import type { Tone } from '../theme/theme'
import type { Label } from './status'

export const availabilityTone: Record<Availability, Tone> = {
  [Availability.$zero]: 'muted',
  [Availability.AvailabilityOK]: 'good',
  [Availability.AvailabilityUnavailable]: 'warn',
  [Availability.AvailabilityError]: 'bad',
  [Availability.AvailabilityNotSupported]: 'muted',
}

/** Причина отказа важнее общей доступности: отсутствие прав не означает отсутствие поддержки API. */
export function toolLabel(d: ToolDiagnostic): Label {
  if (d.availability !== Availability.AvailabilityOK) {
    switch (d.error?.code) {
      case ErrorCode.CodeAccountBlocked:
        return { tone: 'warn', text: t('tools.signInProhibited') }
      case ErrorCode.CodeAccessDenied:
        return { tone: 'warn', text: t('tools.noAccess') }
      case ErrorCode.CodeUnsupported:
        return { tone: 'muted', text: t('availability.not_supported') }
    }
  }
  return { tone: availabilityTone[d.availability] ?? 'muted', text: t(`availability.${d.availability || 'unavailable'}`) }
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
    case ErrorCode.CodeAccountBlocked:
      return { text: t('tools.accountBlocked') }
    case ErrorCode.CodeAccessDenied:
      return { text: t('tools.accessDenied') }
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
