// Радар (03-radar.png): три колонки по группам §3 (logic/radar.ts), фильтры —
// общие с реестром. Карточка открывает проводник; из неё же — Pull, детали
// и переход к строке реестра.

import { Box, Button, CircularProgress, Paper, Tooltip, Typography, alpha, useTheme } from '@mui/material'
import { useMemo, type MouseEvent } from 'react'
import { Virtuoso } from 'react-virtuoso'
import { useNow } from '../app/useNow'
import { Sources, StatusTabs } from '../components/filters'
import { RepoIcon } from '../components/status'
import { Operation, type Repository } from '../data/model'
import { t, type MessageKey } from '../i18n'
import { opName, pullBlock } from '../logic/ops'
import { groupRadar, radarGroups, radarNote, radarTags, type RadarGroup } from '../logic/radar'
import { repoHost, repoName, repoNamespace } from '../logic/repo'
import type { Label } from '../logic/status'
import { remoteCaption } from '../logic/status'
import { NoAutoCheckMark } from '../components/indicators'
import { useApp } from '../store/context'
import { useFilteredRepos, useSearchedRepos } from '../store/selectors'
import type { Tone } from '../theme/theme'
import { toneColor } from '../theme/theme'
import { monoFontFamily } from '../theme/tokens'
import { EmptyText } from './EmptyText'

const groupTitle: Record<RadarGroup, MessageKey> = { attention: 'radar.attention', progress: 'radar.progress', ok: 'radar.ok' }
const groupLegend: Record<RadarGroup, MessageKey> = { attention: 'radar.legend.attention', progress: 'radar.legend.progress', ok: 'radar.legend.ok' }
const groupTone: Record<RadarGroup, Tone> = { attention: 'bad', progress: 'info', ok: 'good' }
const groupMark: Record<RadarGroup, string> = { attention: '!', progress: '↻', ok: '✓' }

function Tag({ label }: { label: Label }) {
  const theme = useTheme()
  const color = toneColor(theme, label.tone)
  return (
    <Box
      component="span"
      sx={{ fontSize: 12, lineHeight: 1.2, px: 0.875, py: 0.5, borderRadius: '6px', color, bgcolor: alpha(color, 0.13), whiteSpace: 'nowrap' }}
    >
      {label.text}
    </Box>
  )
}

function CardActions({ repo }: { repo: Repository }) {
  const requestOp = useApp((s) => s.requestOp)
  const openDetails = useApp((s) => s.openDetails)
  const select = useApp((s) => s.select)
  const setMode = useApp((s) => s.setMode)
  const running = useApp((s) => s.running[repo.id])
  const canPull = pullBlock(repo) === null
  const stop = (e: MouseEvent) => e.stopPropagation() // клик по карточке открывает проводник
  const toRegistry = () => {
    select(repo.id)
    setMode('registry')
  }

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0, ml: 'auto' }} onClick={stop}>
      <Button size="small" variant="text" onClick={toRegistry} sx={{ fontSize: 12, px: 1, minWidth: 0 }}>
        {t('radar.toRegistry')}
      </Button>
      {running ? (
        <Tooltip title={t('op.running', { op: opName(running) })}>
          <CircularProgress size={18} sx={{ mx: 1.5 }} />
        </Tooltip>
      ) : canPull ? (
        <Button size="small" variant="contained" onClick={() => requestOp(repo.id, Operation.OperationPull)} sx={{ fontSize: 12 }}>
          {t('registry.pull')}
        </Button>
      ) : (
        <Button size="small" variant="outlined" onClick={() => openDetails(repo.id)} sx={{ fontSize: 12 }}>
          {t('action.details')} ↗
        </Button>
      )}
    </Box>
  )
}

function RadarCard({ repo, group }: { repo: Repository; group: RadarGroup }) {
  const theme = useTheme()
  const now = useNow()
  const select = useApp((s) => s.select)
  const setMode = useApp((s) => s.setMode)
  const tone = toneColor(theme, groupTone[group])
  const b = repo.branch
  const branch = b?.detached ? `${t('sync.detached')} · ${(b.headSha ?? '').slice(0, 8)}` : b?.name || t('status.unknown')
  const open = () => {
    select(repo.id)
    setMode('explorer')
  }

  return (
    <Paper
      role="button"
      tabIndex={0}
      aria-label={t('radar.toExplorer')}
      onClick={open}
      onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && e.target === e.currentTarget && (e.preventDefault(), open())}
      sx={{
        p: 2.25,
        borderRadius: '15px',
        cursor: 'pointer',
        borderLeft: `3px solid ${tone}`,
        '&:hover, &:focus-visible': { outline: `1px solid ${theme.app.accent}` },
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25 }}>
        <RepoIcon repo={repo} size={35} />
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography noWrap sx={{ fontSize: 14, fontWeight: 700 }}>
            {repoName(repo)}
            <NoAutoCheckMark repo={repo} />
          </Typography>
          <Typography noWrap sx={{ fontSize: 12, color: 'text.secondary' }}>
            {[repoHost(repo) || remoteCaption(repo), repoNamespace(repo)].filter(Boolean).join(' / ')}
          </Typography>
        </Box>
        <Box
          component="span"
          sx={{
            width: 22,
            height: 22,
            display: 'grid',
            placeItems: 'center',
            borderRadius: '6px',
            fontSize: 12,
            fontWeight: 800,
            color: tone,
            bgcolor: alpha(tone, 0.13),
            flexShrink: 0,
          }}
        >
          {groupMark[group]}
        </Box>
      </Box>
      <Typography noWrap sx={{ font: `13px ${monoFontFamily}`, bgcolor: theme.app.chip, borderRadius: '8px', px: 1.25, py: 0.9, mt: 1.75 }}>
        ⑂ {branch}
      </Typography>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.75, mt: 1.5 }}>
        {radarTags(repo).map((l) => (
          <Tag key={l.text} label={l} />
        ))}
      </Box>
      <Box
        sx={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 1,
          mt: 1.75,
          pt: 1.5,
          borderTop: `1px solid ${theme.app.line}`,
        }}
      >
        <Typography sx={{ flex: '1 1 160px', fontSize: 12, color: 'text.secondary', minWidth: 0, overflowWrap: 'anywhere' }}>{radarNote(repo, now)}</Typography>
        <CardActions repo={repo} />
      </Box>
    </Paper>
  )
}

export function RadarView() {
  const theme = useTheme()
  const repos = useFilteredRepos()
  const searched = useSearchedRepos()
  const groups = useMemo(() => groupRadar(repos), [repos])
  const total = useMemo(() => groupRadar(searched), [searched])
  const notOk = total.attention.length + total.progress.length

  return (
    <Box>
      <Sources />
      <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 2, flexWrap: 'wrap', mb: 1 }}>
        <StatusTabs />
        <Typography sx={{ fontSize: 13, color: 'text.secondary', pt: 1 }}>{t('radar.summary', { count: notOk, total: searched.length })}</Typography>
      </Box>
      {groups.unknown.length > 0 && (
        <Typography sx={{ fontSize: 13, color: 'text.secondary', mb: 2 }}>{t('scan.unknownStatus', { count: groups.unknown.length })}</Typography>
      )}
      <Box
        sx={{
          borderTop: `1px solid ${theme.app.line}`,
          pt: 2.5,
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: 'repeat(3, minmax(0, 1fr))' },
          gap: 2,
          alignItems: 'start',
        }}
      >
        {radarGroups.map((g) => (
          <Box key={g} sx={{ minWidth: 0 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', px: 0.5 }}>
              <Typography sx={{ fontSize: 14, fontWeight: 700, color: toneColor(theme, groupTone[g]) }}>● {t(groupTitle[g])}</Typography>
              <Box component="span" sx={{ fontSize: 12, bgcolor: theme.app.chip, borderRadius: '7px', px: 1, py: 0.25 }}>
                {groups[g].length}
              </Box>
            </Box>
            {/* Пояснение — над карточками: при длинных колонках строка внизу не видна (З-1). */}
            <Typography sx={{ fontSize: 12, color: 'text.secondary', px: 0.5, mt: 0.5, mb: 1.5 }}>{t(groupLegend[g])}</Typography>
            {groups[g].length === 0 ? (
              <EmptyText text={t('radar.empty')} />
            ) : (
              // Карточки у видимой области (15.3); у каждой колонки свой список, прокрутка — общая, окна.
              <Virtuoso
                useWindowScroll
                data={groups[g]}
                computeItemKey={(_, r) => r.id}
                increaseViewportBy={600}
                itemContent={(_, r) => (
                  // Отступ — padding, не margin: виртуальный список меряет высоту обёртки.
                  <Box sx={{ pb: 1.5 }}>
                    <RadarCard repo={r} group={g} />
                  </Box>
                )}
              />
            )}
          </Box>
        ))}
      </Box>
    </Box>
  )
}
