// Реестр (экран по умолчанию, макет 01-registry): источники, KPI, вкладки
// статусов, таблица с Pull и меню действий; строка открывает панель деталей.

import { keyframes } from '@emotion/react'
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
import { forwardRef, memo, useCallback, useEffect, useMemo, useRef, useState, type ComponentProps, type CSSProperties, type MouseEvent } from 'react'
import { TableVirtuoso, type TableComponents, type TableVirtuosoHandle } from 'react-virtuoso'
import { CIIndicator, ChangesIndicator, NoAutoCheckMark, PRIndicator, SyncIndicator } from '../components/indicators'
import { FilterBar, filterBarBottom } from '../components/filters'
import { BranchChip, RepoIcon } from '../components/status'
import { Nesting, Operation, type Repository } from '../data/model'
import { formatTime, t, type MessageKey } from '../i18n'
import { opName, pullBlock } from '../logic/ops'
import { kpis, registryRows, type RegistryRow as Row, type SortKey } from '../logic/registry'
import { repoHost, repoName, repoNamespace } from '../logic/repo'
import { remoteCaption } from '../logic/status'
import { useApp } from '../store/context'
import { useRegistryRepos, useRepos } from '../store/selectors'
import { EmptyText } from './EmptyText'

function Metric({ label, value, foot }: { label: string; value: string; foot: string }) {
  return (
    <Paper sx={{ p: 2.5, borderRadius: '14px', minHeight: 104 }}>
      <Typography sx={{ fontSize: 13, color: 'text.secondary', mb: 1 }}>{label}</Typography>
      <Typography component="strong" sx={{ fontSize: 31, fontWeight: 750, letterSpacing: '-1px' }}>
        {value}
      </Typography>
      <Typography component="span" sx={{ fontSize: 12, color: 'text.secondary', ml: 1 }}>
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
            <Button
              size="small"
              variant="outlined"
              disabled={reason !== null}
              onClick={() => requestOp(repo.id, Operation.OperationPull)}
              sx={{ minWidth: 0, px: 1.25, whiteSpace: 'nowrap' }}
            >
              {t('registry.pull')}
            </Button>
          </span>
        </Tooltip>
      )}
      <IconButton
        size="small"
        aria-label={t('action.more')}
        onClick={(e) => setAnchor(e.currentTarget)}
        sx={{ border: 1, borderColor: 'divider', borderRadius: '8px' }}
      >
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
        <Typography sx={{ fontSize: 14, fontWeight: 700, overflowWrap: 'anywhere' }}>
          {repoName(repo)}
          <NoAutoCheckMark repo={repo} />
        </Typography>
        <Typography sx={{ fontSize: 12, color: 'text.secondary', overflowWrap: 'anywhere' }}>
          {[nesting && t(nesting), repoNamespace(repo) || remoteCaption(repo), collapsed && t('registry.hidden', { count: nested })]
            .filter(Boolean)
            .join(' · ')}
        </Typography>
        {/* D-027: до пересканирования строка — из кэша прошлого запуска. */}
        {stale && lastCheckAt && (
          <Typography sx={{ fontSize: 12, color: 'warning.main' }}>{t('registry.cachedAt', { time: formatTime(lastCheckAt) })}</Typography>
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
  onToggle(id: string): void
}

// Ячейки перерисовываются, только когда изменился их репозиторий: при сотнях
// строк перерисовка всей таблицы на каждое событие замораживала UI (11.1).
const RowCells = memo(function RowCells({ repo: r, depth, nested, collapsed, onToggle }: RowProps) {
  return (
    <>
      <TableCell>
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
      <TableCell title={repoHost(r)} sx={{ fontSize: 13.5, color: 'text.secondary', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
        {repoHost(r) || '—'}
      </TableCell>
      <TableCell align="right">
        <RowActions repo={r} />
      </TableCell>
    </>
  )
})

interface TableContext {
  selectedId: string | null
  /** строка, к которой только что перешли из другого режима: коротко вспыхивает */
  flashId: string | null
  openDetails(id: string): void
  sx: ComponentProps<typeof Table>['sx']
}

// Таблица рисует только строки у видимой области (15.3): на 241 репозитории
// DOM всех строк занимал ~330 МБ в WebKit, монтирование — ~340 мс (12.4, 11.1).
const tableComponents: TableComponents<Row, TableContext> = {
  // Горизонтальная прокрутка нужна, только когда таблица (minWidth 1170) не
  // помещается. auto делает обёртку контейнером прокрутки, и заголовок перестаёт
  // прилипать к окну, поэтому на широком окне — clip.
  Scroller: forwardRef<HTMLDivElement>(function Scroller(props, ref) {
    return <Box {...props} ref={ref} sx={{ overflowX: 'auto', '@media (min-width: 1240px)': { overflowX: 'clip' } }} />
  }),
  Table: ({ context, ...props }) => <Table {...props} size="small" sx={context?.sx} />,
  TableHead: forwardRef<HTMLTableSectionElement, { style?: CSSProperties }>(function Head(props, ref) {
    const theme = useTheme()
    // Virtuoso делает заголовок sticky с top: 0 — под шапкой; ставим его под полосу фильтров.
    return <TableHead {...props} ref={ref} style={{ ...props.style, top: `var(${filterBarBottom}, 73px)` }} sx={{ bgcolor: theme.app.panelAlt }} />
  }),
  TableBody: forwardRef<HTMLTableSectionElement>(function Body(props, ref) {
    return <TableBody {...props} ref={ref} />
  }),
  TableRow: ({ item, context, ...props }) => {
    const selected = item.repo.id === context?.selectedId
    return (
      <TableRow
        {...props}
        hover
        selected={selected}
        onClick={() => context?.openDetails(item.repo.id)}
        sx={{
          cursor: 'pointer',
          // Выбранная строка отличается от строки под курсором: полоса слева.
          ...(selected && { '& > td:first-of-type': { boxShadow: (theme) => `inset 3px 0 0 ${theme.palette.primary.main}` } }),
          ...(item.repo.id === context?.flashId && { '& > td': { animation: `${rowFlash} 1.8s ease-in` } }),
        }}
      />
    )
  },
}

const rowFlash = keyframes`
  from { background-color: rgba(117, 141, 255, 0.45); }
`

// Ширины заданы заранее: при виртуализации автоширина колонок менялась бы от
// набора строк в видимой области. Статусы — доля ширины таблицы, чтобы на
// широком окне росли вместе с ней; действия — фиксированно; остаток — у
// колонки репозитория.
const columns: { key: SortKey | null; label: MessageKey; width?: string | number }[] = [
  { key: 'name', label: 'column.repo' },
  { key: 'branch', label: 'column.localBranch', width: '14%' },
  { key: 'changes', label: 'column.worktree', width: '10%' },
  { key: 'sync', label: 'column.syncFull', width: '12%' },
  { key: 'pr', label: 'column.pr', width: '11%' },
  { key: 'ci', label: 'column.ciFull', width: '11%' },
  { key: 'host', label: 'column.server', width: '10%' },
  { key: null, label: 'column.actions', width: 125 },
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
  const openDetails = useApp((s) => s.openDetails)
  const metrics = useMemo(() => kpis(list, all.length), [list, all.length])
  const rows = useMemo(() => registryRows(list, collapsed), [list, collapsed])

  // Пока статус не прочитан ни у одного, 0 был бы неправдой — показываем прочерк.
  const gitValue = (n: number) => (metrics.shown > 0 && metrics.noStatus === metrics.shown ? t('kpi.none') : String(n))
  const gitFoot = (key: 'kpi.changesFoot' | 'kpi.behindFoot') => (metrics.noStatus > 0 ? t('kpi.noStatus', { count: metrics.noStatus }) : t(key))

  // Выбранная в другом режиме строка (переход из радара или проводника) — в центре видимой области.
  // Сразу после монтирования таблица ещё не размечена и прокрутка ни на что не
  // влияет — прокручиваем в следующем кадре, затем строка вспыхивает.
  const virtuoso = useRef<TableVirtuosoHandle>(null)
  const revealed = useRef(false)
  const [flashId, setFlashId] = useState<string | null>(null)
  useEffect(() => {
    if (revealed.current || !selectedId) return
    const index = rows.findIndex((r) => r.repo.id === selectedId)
    if (index < 0) return
    const frame = requestAnimationFrame(() => {
      revealed.current = true
      virtuoso.current?.scrollToIndex({ index, align: 'center' })
      setFlashId(selectedId)
    })
    return () => cancelAnimationFrame(frame)
  }, [rows, selectedId])

  const context = useMemo<TableContext>(
    () => ({
      selectedId,
      flashId,
      openDetails,
      sx: {
        minWidth: 1170,
        tableLayout: 'fixed',
        '& td, & th': { borderColor: theme.app.line, py: 1.5, px: 1 },
        '& td:first-of-type, & th:first-of-type': { pl: 2 },
        '& td:last-of-type, & th:last-of-type': { pr: 2 },
      },
    }),
    [selectedId, flashId, openDetails, theme],
  )

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

      {/* Карточки сверху и уезжают при прокрутке; источники и вкладки — прилипают. */}
      <FilterBar />

      {/* clip, а не hidden: hidden делает карточку контейнером прокрутки, и заголовок таблицы не прилипает. */}
      <Paper sx={{ borderRadius: '15px', overflow: 'clip' }}>
        <TableVirtuoso
          ref={virtuoso}
          useWindowScroll
          data={rows}
          context={context}
          components={tableComponents}
          computeItemKey={(_, row) => row.repo.id}
          increaseViewportBy={600}
          fixedHeaderContent={() => (
            <TableRow>
              {columns.map((c) => (
                <TableCell
                  key={c.label}
                  align={c.key ? 'left' : 'right'}
                  sx={{
                    width: c.width,
                    fontSize: 12,
                    fontWeight: 800,
                    letterSpacing: '0.6px',
                    textTransform: 'uppercase',
                    color: 'text.secondary',
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    '& .MuiTableSortLabel-root:not(.Mui-active) .MuiTableSortLabel-icon': { display: 'none' },
                  }}
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
          )}
          itemContent={(_, { repo: r, depth, children }) => (
            <RowCells repo={r} depth={depth} nested={children} collapsed={collapsed.has(r.id)} onToggle={toggle} />
          )}
        />
        {list.length === 0 && <EmptyText text={t('empty.noMatches')} />}
      </Paper>
      <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 1.5, textAlign: 'right' }}>
        {t('registry.hint')}
      </Typography>
    </Box>
  )
}
