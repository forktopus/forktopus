// Общие фильтры реестра и радара: источники (хосты) и вкладки статусов.
// Состояние одно (store.filters), поэтому переход между режимами сохраняет выборку.

import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import { Box, ButtonBase, ToggleButton, ToggleButtonGroup, Tooltip, Typography, useTheme } from '@mui/material'
import { useMemo, type ReactNode } from 'react'
import { t, type MessageKey } from '../i18n'
import { hostSources, registryFilters, type RegistryFilter } from '../logic/registry'
import { useApp } from '../store/context'
import { useSearchedRepos } from '../store/selectors'
import { toneColor } from '../theme/theme'
import { caption } from './status'

const filterLabel: Record<RegistryFilter, MessageKey> = {
  all: 'filter.all',
  attention: 'filter.attention',
  dirty: 'filter.dirty',
  behind: 'filter.behind',
  ci: 'filter.ci',
  integration: 'filter.integration',
}

function SourcePill({ active, onClick, children, warn = false }: { active: boolean; onClick(): void; children: ReactNode; warn?: boolean }) {
  const theme = useTheme()
  return (
    <ButtonBase
      onClick={onClick}
      aria-pressed={active}
      sx={{
        gap: 0.75,
        px: 1.5,
        py: 0.9,
        borderRadius: '10px',
        fontSize: 13.5,
        border: `1px solid ${warn ? toneColor(theme, 'warn') : active ? theme.palette.primary.main : theme.app.line}`,
        bgcolor: active ? theme.app.tabActive : 'transparent',
        color: active ? theme.app.tabActiveText : 'text.primary',
      }}
    >
      {children}
    </ButtonBase>
  )
}

/** Фильтр источников: хосты со счётчиками и предупреждением о незавершённой настройке CLI. */
export function Sources() {
  const theme = useTheme()
  const searched = useSearchedRepos()
  const host = useApp((s) => s.filters.host)
  const setFilters = useApp((s) => s.setFilters)
  const sources = useMemo(() => hostSources(searched), [searched])
  const count = (n: number) => (
    <Box component="span" sx={{ fontSize: 12, color: 'text.secondary' }}>
      {n}
    </Box>
  )
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', mb: 2.5 }}>
      <Typography sx={{ ...caption, mr: 1 }}>{t('registry.sources')}</Typography>
      <SourcePill active={host === ''} onClick={() => setFilters({ host: '' })}>
        {t('filter.all')} {count(searched.length)}
      </SourcePill>
      {sources.map((s) => (
        <Tooltip key={s.host} title={s.issue ? t('registry.sourceIssue') : ''}>
          <span>
            <SourcePill active={host === s.host} warn={s.issue} onClick={() => setFilters({ host: host === s.host ? '' : s.host })}>
              {s.host}
              {s.issue && <WarningAmberIcon sx={{ fontSize: 16, color: toneColor(theme, 'warn') }} />}
              {count(s.count)}
            </SourcePill>
          </span>
        </Tooltip>
      ))}
    </Box>
  )
}

/** Вкладки «Все / Требуют внимания / Изменения / Отстают / Ошибки CI / Нет данных PR/CI». */
export function StatusTabs() {
  const value = useApp((s) => s.filters.registry)
  const setFilters = useApp((s) => s.setFilters)
  return (
    <ToggleButtonGroup
      size="small"
      exclusive
      value={value}
      onChange={(_, v: RegistryFilter | null) => v && setFilters({ registry: v })}
      sx={{ mb: 1.5, flexWrap: 'wrap' }}
    >
      {registryFilters.map((f) => (
        <ToggleButton key={f} value={f} sx={{ textTransform: 'none', px: 1.5 }}>
          {t(filterLabel[f])}
        </ToggleButton>
      ))}
    </ToggleButtonGroup>
  )
}
