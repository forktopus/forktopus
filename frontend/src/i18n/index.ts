import { ru, type PluralForms } from './ru'

export type MessageKey = keyof typeof ru
export type Params = Record<string, string | number>

const locale = 'ru-RU'
const pluralRules = new Intl.PluralRules(locale)
const numberFormat = new Intl.NumberFormat(locale)
const dateTimeFormat = new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' })
const timeFormat = new Intl.DateTimeFormat(locale, { timeStyle: 'short' })
const relativeFormat = new Intl.RelativeTimeFormat(locale, { numeric: 'auto', style: 'short' })

/** Строка интерфейса по ключу словаря с подстановкой параметров. */
export function t(key: MessageKey, params?: Params): string {
  const entry: string | PluralForms = ru[key]
  let template: string
  if (typeof entry === 'string') {
    template = entry
  } else {
    const form = pluralRules.select(Number(params?.count ?? 0)) as keyof PluralForms
    template = entry[form] ?? entry.many
  }
  return template.replace(/\{(\w+)\}/g, (match, name: string) => {
    const value = params?.[name]
    if (value === undefined) return match
    return typeof value === 'number' ? formatNumber(value) : value
  })
}

export function formatNumber(n: number): string {
  return numberFormat.format(n)
}

export function formatDateTime(value: Date | string): string {
  return dateTimeFormat.format(new Date(value))
}

/** «14:05» */
export function formatTime(value: Date | string): string {
  return timeFormat.format(new Date(value))
}

const MINUTE = 60
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

/** «только что», «5 мин. назад», «вчера»; старше месяца — дата и время. */
export function formatRelative(value: Date | string, now: Date = new Date()): string {
  const seconds = Math.round((new Date(value).getTime() - now.getTime()) / 1000)
  const abs = Math.abs(seconds)
  if (abs < 45) return t('time.justNow')
  if (abs < HOUR) return relativeFormat.format(Math.round(seconds / MINUTE), 'minute')
  if (abs < DAY) return relativeFormat.format(Math.round(seconds / HOUR), 'hour')
  if (abs < 30 * DAY) return relativeFormat.format(Math.round(seconds / DAY), 'day')
  return formatDateTime(value)
}
