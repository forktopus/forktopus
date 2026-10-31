// Реестр (экран по умолчанию, макет 01-registry): источники, KPI, вкладки
// статусов, таблица с Pull и меню действий; строка открывает панель деталей.

import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import MoreHorizIcon from '@mui/icons-material/MoreHoriz'
import {
  Box,
  Button,
  CircularProgress,
  IconButton,
  Menu,
  MenuItem,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TableSortLabel,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material'
import { memo, useCallback, useMemo, useState, type MouseEvent } from 'react'
import { CIIndicator, ChangesIndicator, PRIndicator, SyncIndicator } from '../components/indicators'
import { Sources, StatusTabs } from '../components/filters'
import { BranchChip, RepoIcon } from '../components/status'
import { Nesting, Operation, type Repository } from '../data/model'
import { formatTime, t, type MessageKey } from '../i18n'
import { opName, pullBlock } from '../logic/ops'
import { kpis, registryRows, type SortKey } from '../logic/registry'
import { repoHost, repoName, repoNamespace } from '../logic/repo'
import { remoteCaption } from '../logic/status'
import { useApp } from '../store/context'
import { useRegistryRepos, useRepos } from '../store/selectors'
import { EmptyText } from './EmptyText'

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

/** Pull и меню действий строки. Клик здесь не открывает панель деталей. */
function RowActions({ repo }: { repo: Repository }) {
  const requestOp = useApp((s) => s.requestOp)
  const openDetails = useApp((s) => s.openDetails)
  const copyText = useApp((s) => s.copyText)
  const running = useApp((s) => s.running[repo.id])
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  const reason = pullBlock(repo)
  const stop = (e: MouseEvent) => e.stopPropagation()
  const pick = (action: () => void) => () => {
    setAnchor(null)
    action()
  }

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, justifyContent: 'flex-end' }} onClick={stop}>
      {running ? (
        <Tooltip title={t('op.running', { op: opName(running) })}>
          <CircularProgress size={18} sx={{ mx: 2 }} />
        </Tooltip>
      ) : (
        <Tooltip title={reason ?? ''}>
          <span>
            <Button size="small" variant="outlined" disabled={reason !== null} onClick={() => requestOp(repo.id, Operation.OperationPull)} sx={{ minWidth: 0, px: 1.25, whiteSpace: 'nowrap' }}>
              {t('registry.pull')}
            </Button>
          </span>
        </Tooltip>
      )}
      <IconButton size="small" aria-label={t('action.more')} onClick={(e) => setAnchor(e.currentTarget)} sx={{ border: 1, borderColor: 'divider', borderRadius: '8px' }}>
        <MoreHorizIcon fontSize="small" />
      </IconButton>
      <Menu anchorEl={anchor} open={anchor !== null} onClose={() => setAnchor(null)}>
        <MenuItem disabled={Boolean(running) || !repo.remote} onClick={pick(() => requestOp(repo.id, Operation.OperationFetch))}>
          {t('action.fetch')}
        </MenuItem>
        <MenuItem disabled={Boolean(running)} onClick={pick(() => openDetails(repo.id))}>
          {t('action.switchBranch')}
        </MenuItem>
        <MenuItem onClick={pick(() => void copyText(repo.path))}>{t('action.copyPath')}</MenuItem>
        <MenuItem onClick={pick(() => openDetails(repo.id))}>{t('action.details')}</MenuItem>
      </Menu>
    </Box>
  )
}

const nestingKey: Partial<Record<Nesting, MessageKey>> = {
  [Nesting.NestingSubmodule]: 'registry.submodule',
  [Nesting.NestingNested]: 'registry.nested',
}

function RepoCell({ repo, depth, nested, collapsed, onToggle }: { repo: Repository; depth: number; nested: number; collapsed: boolean; onToggle(): void }) {
  const stale = useApp((s) => s.stale)
  const lastCheckAt = useApp((s) => s.lastCheckAt)
  const nesting = repo.nesting ? nestingKey[repo.nesting] : undefined
  return (
    <Box sx={{ display: 'flex', gap: 1.4, alignItems: 'center', pl: depth * 3.5 }}>
      <Box sx={{ width: 24, flexShrink: 0 }}>
        {nested > 0 && (
          <IconButton
            size="small"
            aria-label={collapsed ? t('registry.expand', { count: nested }) : t('registry.collapse')}
            aria-expanded={!collapsed}
            onClick={(e) => {
              e.stopPropagation()
              onToggle()
            }}
            sx={{ p: 0.25 }}
          >
            {collapsed ? <ChevronRightIcon fontSize="small" /> : <ExpandMoreIcon fontSize="small" />}
          </IconButton>
        )}
      </Box>
      <RepoIcon repo={repo} />
      <Box sx={{ minWidth: 0 }}>
        <Typography sx={{ fontSize: 13, fontWeight: 700, overflowWrap: 'anywhere' }}>{repoName(repo)}</Typography>
        <Typography sx={{ fontSize: 11, color: 'text.secondary', overflowWrap: 'anywhere' }}>
          {[nesting && t(nesting), repoNamespace(repo) || remoteCaption(repo), collapsed && t('registry.hidden', { count: nested })]
            .filter(Boolean)
            .join(' · ')}
        </Typography>
        {/* D-027: до пересканирования строка — из кэша прошлого запуска. */}
        {stale && lastCheckAt && (
          <Typography sx={{ fontSize: 10.5, color: 'warning.main' }}>{t('registry.cachedAt', { time: formatTime(lastCheckAt) })}</Typography>
        )}
      </Box>
    </Box>
  )
}

interface RowProps {
  repo: Repository
  depth: number
  nested: number
  collapsed: boolean
  selected: boolean
  onToggle(id: string): void
}

// Строка перерисовывается, только когда изменился её репозиторий: при сотнях
// строк перерисовка всей таблицы на каждое событие замораживала UI (11.1).
const RegistryRow = memo(function RegistryRow({ repo: r, depth, nested, collapsed, selected, onToggle }: RowProps) {
  const openDetails = useApp((s) => s.openDetails)
  // Выбранная в другом режиме строка прокручивается в видимую область.
  const reveal = useCallback((el: HTMLTableRowElement | null) => el?.scrollIntoView({ block: 'center' }), [])
  return (
    <TableRow hover selected={selected} ref={selected ? reveal : undefined} onClick={() => openDetails(r.id)} sx={{ cursor: 'pointer' }}>
      <TableCell sx={{ minWidth: 260 }}>
        <RepoCell repo={r} depth={depth} nested={nested} collapsed={collapsed} onToggle={() => onToggle(r.id)} />
      </TableCell>
      <TableCell>
        <BranchChip repo={r} />
      </TableCell>
      <TableCell>
        <ChangesIndicator repo={r} />
      </TableCell>
      <TableCell>
        <SyncIndicator repo={r} />
      </TableCell>
      <TableCell>
        <PRIndicator repo={r} />
      </TableCell>
      <TableCell>
        <CIIndicator repo={r} />
      </TableCell>
      <TableCell sx={{ fontSize: 12.5, color: 'text.secondary', whiteSpace: 'nowrap' }}>{repoHost(r) || '—'}</TableCell>
      <TableCell align="right">
        <RowActions repo={r} />
      </TableCell>
    </TableRow>
  )
})

const columns: { key: SortKey | null; label: MessageKey; width?: number }[] = [
  { key: 'name', label: 'column.repo' },
  { key: 'branch', label: 'column.localBranch' },
  { key: 'changes', label: 'column.worktree' },
  { key: 'sync', label: 'column.syncFull' },
  { key: 'pr', label: 'column.pr' },
  { key: 'ci', label: 'column.ciFull' },
  { key: 'host', label: 'column.server' },
  { key: null, label: 'column.actions' },
]

export function RegistryView() {
  const theme = useTheme()
  const all = useRepos()
  const list = useRegistryRepos()
  const filters = useApp((s) => s.filters)
  const setFilters = useApp((s) => s.setFilters)
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set())
  // Выбранный в другом режиме репозиторий (переход из радара или проводника) подсвечен.
  const selectedId = useApp((s) => s.selectedId)
  const metrics = useMemo(() => kpis(list, all.length), [list, all.length])
  const rows = useMemo(() => registryRows(list, collapsed), [list, collapsed])

  // Пока статус не прочитан ни у одного, 0 был бы неправдой — показываем прочерк.
  const gitValue = (n: number) => (metrics.shown > 0 && metrics.noStatus === metrics.shown ? t('kpi.none') : String(n))
  const gitFoot = (key: 'kpi.changesFoot' | 'kpi.behindFoot') =>
    metrics.noStatus > 0 ? t('kpi.noStatus', { count: metrics.noStatus }) : t(key)

  const sortBy = (key: SortKey) => setFilters(filters.sort === key ? { asc: !filters.asc } : { sort: key, asc: true })
  const toggle = useCallback(
    (id: string) =>
      setCollapsed((prev) => {
        const next = new Set(prev)
        if (!next.delete(id)) next.add(id)
        return next
      }),
    [],
  )

  return (
    <Box>
      <Sources />
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'repeat(2, minmax(0, 1fr))', lg: 'repeat(4, minmax(0, 1fr))' }, gap: 1.6, mb: 3 }}>
        <Metric label={t('kpi.repos')} value={String(metrics.shown)} foot={t('kpi.reposFoot', { total: metrics.total })} />
        <Metric label={t('kpi.changes')} value={gitValue(metrics.withChanges)} foot={gitFoot('kpi.changesFoot')} />
        <Metric label={t('kpi.behind')} value={gitValue(metrics.behind)} foot={gitFoot('kpi.behindFoot')} />
        <Metric
          label={t('kpi.prci')}
          value={metrics.shown > 0 && metrics.remoteKnown === 0 ? t('kpi.none') : `${metrics.openPRs} / ${metrics.activeCI}`}
          foot={metrics.shown > 0 && metrics.remoteKnown === 0 ? t('kpi.noSource') : metrics.partial ? t('kpi.partial') : t('kpi.complete')}
        />
      </Box>

      <StatusTabs />

      <Paper sx={{ borderRadius: '15px', overflowX: 'auto' }}>
        <Table size="small" sx={{ minWidth: 1100, '& td, & th': { borderColor: theme.app.line, py: 1.5 } }}>
          <TableHead sx={{ bgcolor: theme.app.panelAlt }}>
            <TableRow>
              {columns.map((c) => (
                <TableCell
                  key={c.label}
                  align={c.key ? 'left' : 'right'}
                  sx={{ fontSize: 10, fontWeight: 800, letterSpacing: '1.1px', textTransform: 'uppercase', color: 'text.secondary', whiteSpace: 'nowrap' }}
                >
                  {c.key ? (
                    <TableSortLabel active={filters.sort === c.key} direction={filters.asc ? 'asc' : 'desc'} onClick={() => sortBy(c.key!)}>
                      {t(c.label)}
                    </TableSortLabel>
                  ) : (
                    t(c.label)
                  )}
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {rows.map(({ repo: r, depth, children }) => (
              <RegistryRow key={r.id} repo={r} depth={depth} nested={children} collapsed={collapsed.has(r.id)} selected={r.id === selectedId} onToggle={toggle} />
            ))}
          </TableBody>
        </Table>
        {list.length === 0 && <EmptyText text={t('empty.noMatches')} />}
      </Paper>
      <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 1.5, textAlign: 'right' }}>
        {t('registry.hint')}
      </Typography>
    </Box>
  )
}
