// Черновой реестр: KPI, фильтры, таблица статусов. Полноценный экран
// (детали, Fetch/Pull) — этап 10.

import { Box, MenuItem, Paper, Select, Table, TableBody, TableCell, TableHead, TableRow, TableSortLabel, ToggleButton, ToggleButtonGroup, Typography, useTheme } from '@mui/material'
import { useMemo } from 'react'
import { BranchChip, RepoIcon, StatusText } from '../components/status'
import { t, type MessageKey } from '../i18n'
import { hostsOf, kpis, registryFilters, type RegistryFilter, type SortKey } from '../logic/registry'
import { repoHost, repoName, repoNamespace } from '../logic/repo'
import { changesLabel, ciLabel, prLabel, syncLabel } from '../logic/status'
import { useApp } from '../store/context'
import { useRegistryRepos, useRepos, useSearchedRepos } from '../store/selectors'
import { EmptyText } from './EmptyText'

const filterLabel: Record<RegistryFilter, MessageKey> = {
  all: 'filter.all',
  attention: 'filter.attention',
  dirty: 'filter.dirty',
  behind: 'filter.behind',
  ci: 'filter.ci',
}

function Metric({ label, value, foot }: { label: string; value: string; foot: string }) {
  return (
    <Paper sx={{ p: 2.5, borderRadius: '14px', minHeight: 104 }}>
      <Typography sx={{ fontSize: 12, color: 'text.secondary', mb: 1 }}>{label}</Typography>
      <Typography component="strong" sx={{ fontSize: 29, fontWeight: 750, letterSpacing: '-1px' }}>
        {value}
      </Typography>
      <Typography component="span" sx={{ fontSize: 11, color: 'text.secondary', ml: 1 }}>
        {foot}
      </Typography>
    </Paper>
  )
}

const columns: { key: SortKey; label: MessageKey }[] = [
  { key: 'name', label: 'column.repo' },
  { key: 'branch', label: 'column.branch' },
  { key: 'changes', label: 'column.changes' },
  { key: 'sync', label: 'column.sync' },
  { key: 'pr', label: 'column.pr' },
  { key: 'ci', label: 'column.ci' },
]

export function RegistryView() {
  const theme = useTheme()
  const all = useRepos()
  const searched = useSearchedRepos()
  const list = useRegistryRepos()
  const filters = useApp((s) => s.filters)
  const setFilters = useApp((s) => s.setFilters)
  const metrics = useMemo(() => kpis(list, all.length), [list, all.length])
  const hosts = useMemo(() => hostsOf(searched), [searched])

  // Пока статус не прочитан ни у одного, 0 был бы неправдой — показываем прочерк.
  const gitValue = (n: number) => (metrics.shown > 0 && metrics.noStatus === metrics.shown ? t('kpi.none') : String(n))
  const gitFoot = (key: 'kpi.changesFoot' | 'kpi.behindFoot') =>
    metrics.noStatus > 0 ? t('kpi.noStatus', { count: metrics.noStatus }) : t(key)

  const sortBy = (key: SortKey) => setFilters(filters.sort === key ? { asc: !filters.asc } : { sort: key, asc: true })

  return (
    <Box>
      <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 1.6, mb: 3 }}>
        <Metric label={t('kpi.repos')} value={String(metrics.shown)} foot={t('kpi.reposFoot', { total: metrics.total })} />
        <Metric label={t('kpi.changes')} value={gitValue(metrics.withChanges)} foot={gitFoot('kpi.changesFoot')} />
        <Metric label={t('kpi.behind')} value={gitValue(metrics.behind)} foot={gitFoot('kpi.behindFoot')} />
        <Metric
          label={t('kpi.prci')}
          value={`${metrics.openPRs} / ${metrics.activeCI}`}
          foot={metrics.partial ? t('kpi.partial') : t('kpi.complete')}
        />
      </Box>

      <Box sx={{ display: 'flex', gap: 2, alignItems: 'center', justifyContent: 'space-between', mb: 1.5, flexWrap: 'wrap' }}>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={filters.registry}
          onChange={(_, v: RegistryFilter | null) => v && setFilters({ registry: v })}
        >
          {registryFilters.map((f) => (
            <ToggleButton key={f} value={f} sx={{ textTransform: 'none', px: 1.5 }}>
              {t(filterLabel[f])}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
        <Select size="small" value={filters.host} displayEmpty onChange={(e) => setFilters({ host: e.target.value })} sx={{ minWidth: 180 }}>
          <MenuItem value="">{t('filter.allHosts')}</MenuItem>
          {hosts.map((h) => (
            <MenuItem key={h} value={h}>
              {h}
            </MenuItem>
          ))}
        </Select>
      </Box>

      <Paper sx={{ borderRadius: '15px', overflowX: 'auto' }}>
        <Table size="small" sx={{ minWidth: 900, '& td, & th': { borderColor: theme.app.line, py: 1.5 } }}>
          <TableHead sx={{ bgcolor: theme.app.panelAlt }}>
            <TableRow>
              {columns.map((c) => (
                <TableCell key={c.key} sx={{ fontSize: 10, fontWeight: 800, letterSpacing: '1.1px', textTransform: 'uppercase', color: 'text.secondary' }}>
                  <TableSortLabel active={filters.sort === c.key} direction={filters.asc ? 'asc' : 'desc'} onClick={() => sortBy(c.key)}>
                    {t(c.label)}
                  </TableSortLabel>
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {list.map((r) => (
              <TableRow key={r.id} hover>
                <TableCell>
                  <Box sx={{ display: 'flex', gap: 1.4, alignItems: 'center' }}>
                    <RepoIcon repo={r} />
                    <Box sx={{ minWidth: 0 }}>
                      <Typography sx={{ fontSize: 13, fontWeight: 700 }}>{repoName(r)}</Typography>
                      <Typography sx={{ fontSize: 11, color: 'text.secondary' }}>
                        {[repoHost(r) || t('repo.noRemote'), repoNamespace(r)].filter(Boolean).join(' / ')}
                      </Typography>
                    </Box>
                  </Box>
                </TableCell>
                <TableCell>
                  <BranchChip repo={r} />
                </TableCell>
                <TableCell>
                  {r.error ? <StatusText label={{ tone: 'bad', text: t('repo.readError') }} /> : <StatusText label={changesLabel(r)} />}
                </TableCell>
                <TableCell>
                  <StatusText label={syncLabel(r)} />
                </TableCell>
                <TableCell>
                  <StatusText label={prLabel(r)} />
                </TableCell>
                <TableCell>
                  <StatusText label={ciLabel(r)} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {list.length === 0 && <EmptyText text={t('empty.noMatches')} />}
      </Paper>
    </Box>
  )
}
