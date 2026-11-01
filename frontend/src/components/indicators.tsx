// Индикаторы состояния репозитория для всех режимов. Подпись — из logic/status;
// пояснение (детализация изменений, свежесть fetch, причина ошибки PR/CI)
// показывается строкой ниже или подсказкой при наведении.

import OpenInNewIcon from '@mui/icons-material/OpenInNew'
import { Box, IconButton, Tooltip, Typography } from '@mui/material'
import type { ReactNode } from 'react'
import { useNow } from '../app/useNow'
import { Availability, type CISummary, type PRSummary, type Repository } from '../data/model'
import { t } from '../i18n'
import { changesDetail, changesLabel, ciLabel, prLabel, remoteDetail, syncFreshness, syncLabel, type Label } from '../logic/status'
import { useApp } from '../store/context'
import { StatusText } from './status'

export type DetailMode = 'inline' | 'tooltip'

function Indicator({ label, detail, mode, action }: { label: Label; detail: string; mode: DetailMode; action?: ReactNode }) {
  const text = <StatusText label={label} wrap={mode === 'inline' || 'words'} />
  if (mode === 'tooltip') {
    return (
      <Box component="span" sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5 }}>
        {detail ? (
          <Tooltip title={detail}>
            <span>{text}</span>
          </Tooltip>
        ) : (
          text
        )}
        {action}
      </Box>
    )
  }
  return (
    <Box sx={{ minWidth: 0 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
        {text}
        {action}
      </Box>
      {detail && (
        <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', pl: 1.75, overflowWrap: 'anywhere' }}>
          {detail}
        </Typography>
      )}
    </Box>
  )
}

/** Изменения рабочей копии с детализацией staged / unstaged / untracked. */
export function ChangesIndicator({ repo, mode = 'tooltip' }: { repo: Repository; mode?: DetailMode }) {
  if (repo.error) return <StatusText wrap label={{ tone: 'bad', text: t('repo.readError') }} />
  return <Indicator label={changesLabel(repo)} detail={changesDetail(repo)} mode={mode} />
}

/** Синхронизация с upstream и время fetch, по которому она посчитана (A09). */
export function SyncIndicator({ repo, mode = 'tooltip' }: { repo: Repository; mode?: DetailMode }) {
  const now = useNow()
  return <Indicator label={syncLabel(repo)} detail={syncFreshness(repo, now)} mode={mode} />
}

function RemoteLink({ summary }: { summary: PRSummary | CISummary }) {
  const openURL = useApp((s) => s.openURL)
  // Открытых PR нет — ссылка на последний закрытый.
  const url = summary.url || ('last' in summary ? summary.last?.url : undefined)
  if (summary.availability !== Availability.AvailabilityOK || !url) return null
  return (
    <Tooltip title={t('action.openOnServer')}>
      <IconButton
        size="small"
        aria-label={t('action.openOnServer')}
        onClick={(e) => {
          e.stopPropagation() // строка таблицы открывает детали
          openURL(url)
        }}
        sx={{ p: 0.25 }}
      >
        <OpenInNewIcon sx={{ fontSize: 15 }} />
      </IconButton>
    </Tooltip>
  )
}

/** Открытые PR/MR ветки: число только при успешном ответе CLI, иначе причина. */
export function PRIndicator({ repo, mode = 'tooltip' }: { repo: Repository; mode?: DetailMode }) {
  const now = useNow()
  return <Indicator label={prLabel(repo, now)} detail={remoteDetail(repo.pr, now)} mode={mode} action={<RemoteLink summary={repo.pr} />} />
}

/** CI-запуски ветки: активные, последний итог или причина отсутствия данных. */
export function CIIndicator({ repo, mode = 'tooltip' }: { repo: Repository; mode?: DetailMode }) {
  const now = useNow()
  return <Indicator label={ciLabel(repo)} detail={remoteDetail(repo.ci, now)} mode={mode} action={<RemoteLink summary={repo.ci} />} />
}
