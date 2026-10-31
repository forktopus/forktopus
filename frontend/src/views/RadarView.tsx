// Черновой радар: три колонки по правилам группировки. Полноценный экран
// (карточки с действиями, переход в реестр/проводник) — этап 13.

import { Box, Paper, Typography, useTheme } from '@mui/material'
import { useMemo } from 'react'
import { BranchChip, RepoIcon, StatusText } from '../components/status'
import { t, type MessageKey } from '../i18n'
import { groupRadar, radarGroups, type RadarGroup } from '../logic/radar'
import { repoHost, repoName, repoNamespace } from '../logic/repo'
import { changesLabel, ciLabel, prLabel, syncLabel } from '../logic/status'
import { useApp } from '../store/context'
import { useSearchedRepos } from '../store/selectors'
import type { Tone } from '../theme/theme'
import { toneColor } from '../theme/theme'

const groupTitle: Record<RadarGroup, MessageKey> = { attention: 'radar.attention', progress: 'radar.progress', ok: 'radar.ok' }
const groupTone: Record<RadarGroup, Tone> = { attention: 'bad', progress: 'info', ok: 'good' }

export function RadarView() {
  const theme = useTheme()
  const repos = useSearchedRepos()
  const setMode = useApp((s) => s.setMode)
  const select = useApp((s) => s.select)
  const groups = useMemo(() => groupRadar(repos), [repos])

  return (
    <>
      {groups.unknown.length > 0 && (
        <Typography sx={{ fontSize: 12, color: 'text.secondary', mb: 2 }}>
          {t('scan.unknownStatus', { count: groups.unknown.length })}
        </Typography>
      )}
      <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 2, alignItems: 'start' }}>
        {radarGroups.map((g) => (
          <Box key={g} sx={{ minWidth: 0 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1.5, px: 0.5 }}>
              <Typography sx={{ fontSize: 13, fontWeight: 700 }}>{t(groupTitle[g])}</Typography>
              <Box component="span" sx={{ fontSize: 11, bgcolor: theme.app.chip, borderRadius: '7px', px: 1, py: 0.25 }}>
                {groups[g].length}
              </Box>
            </Box>
            {groups[g].map((r) => (
              <Paper
                key={r.id}
                onClick={() => {
                  select(r.id)
                  setMode('explorer')
                }}
                sx={{
                  p: 2,
                  mb: 1.5,
                  borderRadius: '15px',
                  cursor: 'pointer',
                  borderLeft: `3px solid ${toneColor(theme, groupTone[g])}`,
                  '&:hover': { borderColor: theme.app.accent },
                }}
              >
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, mb: 1.5 }}>
                  <RepoIcon repo={r} size={35} />
                  <Box sx={{ minWidth: 0 }}>
                    <Typography noWrap sx={{ fontSize: 13, fontWeight: 700 }}>
                      {repoName(r)}
                    </Typography>
                    <Typography noWrap sx={{ fontSize: 10, color: 'text.secondary' }}>
                      {[repoHost(r) || t('repo.noRemote'), repoNamespace(r)].filter(Boolean).join(' / ')}
                    </Typography>
                  </Box>
                </Box>
                <BranchChip repo={r} />
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.5, mt: 1.5 }}>
                  {r.error ? <StatusText label={{ tone: 'bad', text: t('repo.readError') }} /> : <StatusText label={changesLabel(r)} />}
                  {r.worktree?.operation && (
                    <StatusText label={{ tone: 'bad', text: t('changes.operation', { operation: r.worktree.operation }) }} />
                  )}
                  <StatusText label={syncLabel(r)} />
                  <StatusText label={prLabel(r)} />
                  <StatusText label={ciLabel(r)} />
                </Box>
              </Paper>
            ))}
          </Box>
        ))}
      </Box>
    </>
  )
}
