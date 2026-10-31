// Черновой проводник: дерево корень → хост → namespace → репозиторий,
// сведения о выбранном и безопасные операции (fetch, ff-pull, switch).
// Полноценный экран — этап 10.

import RefreshIcon from '@mui/icons-material/Refresh'
import { Box, Button, ButtonBase, CircularProgress, IconButton, Paper, Tooltip, Typography, useTheme } from '@mui/material'
import { useEffect, useMemo, useState } from 'react'
import { CIIndicator, ChangesIndicator, PRIndicator, SyncIndicator } from '../components/indicators'
import { BranchChip, RepoIcon, StatusText, caption } from '../components/status'
import { Operation, type BranchInfo, type Repository } from '../data/model'
import { formatRelative, t } from '../i18n'
import { buildTree } from '../logic/explorer'
import { opName, opResultLabel } from '../logic/ops'
import { repoName } from '../logic/repo'
import { syncLabel } from '../logic/status'
import { useApp } from '../store/context'
import { useSearchedRepos } from '../store/selectors'
import { monoFontFamily } from '../theme/tokens'
import { EmptyText } from './EmptyText'

/** Локальные ветки выбранного репозитория; перечитываются при смене выбора и после обновления статуса. */
function Branches({ repo }: { repo: Repository }) {
  const loadBranches = useApp((s) => s.loadBranches)
  const requestOp = useApp((s) => s.requestOp)
  const busy = useApp((s) => Boolean(s.running[repo.id]))
  const [branches, setBranches] = useState<BranchInfo[] | null>(null)
  useEffect(() => {
    let alive = true
    setBranches(null)
    void loadBranches(repo.id).then((list) => alive && setBranches(list ?? []))
    return () => {
      alive = false
    }
  }, [repo.id, repo.updatedAt, loadBranches])

  if (branches === null) return <CircularProgress size={16} />
  if (branches.length === 0) return <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>{t('explorer.noBranches')}</Typography>
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: 'auto auto 1fr auto', columnGap: 2, rowGap: 1, alignItems: 'center' }}>
      {branches.map((b) => (
        <Box key={b.name} sx={{ display: 'contents' }}>
          <Typography sx={{ font: `12px ${monoFontFamily}`, fontWeight: b.current ? 760 : 400, color: b.current ? 'primary.main' : 'text.primary' }}>
            {b.name}
          </Typography>
          <StatusText label={syncLabel({ ...repo, branch: { ...b, detached: false } })} />
          <Typography sx={{ fontSize: 11, color: 'text.secondary', overflowWrap: 'anywhere' }}>
            {[
              b.current && t('explorer.branchCurrent'),
              b.worktreePath && t('explorer.branchBusy', { path: b.worktreePath }),
              b.commitAt && formatRelative(b.commitAt),
            ]
              .filter(Boolean)
              .join(' · ')}
          </Typography>
          <Box>
            {!b.current && (
              // Причину блокировки (dirty, ветка в другом worktree…) сообщает backend до вызова Git.
              <Button size="small" variant="text" disabled={busy} onClick={() => requestOp(repo.id, Operation.OperationSwitch, b.name)}>
                {t('action.switch')}
              </Button>
            )}
          </Box>
        </Box>
      ))}
    </Box>
  )
}

export function ExplorerView() {
  const theme = useTheme()
  const roots = useApp((s) => s.roots)
  const selectedId = useApp((s) => s.selectedId)
  const select = useApp((s) => s.select)
  const repos = useSearchedRepos()
  const tree = useMemo(() => buildTree(roots, repos), [roots, repos])
  const selected = repos.find((r) => r.id === selectedId) ?? null
  const refreshRepo = useApp((s) => s.refreshRepo)
  const requestOp = useApp((s) => s.requestOp)
  const running = useApp((s) => (selectedId ? s.running[selectedId] : undefined))
  const lastOp = useApp((s) => (selectedId ? s.lastOps[selectedId] : undefined))
  const [refreshing, setRefreshing] = useState(false)
  const refresh = async (id: string) => {
    setRefreshing(true)
    await refreshRepo(id)
    setRefreshing(false)
  }

  return (
    <Paper sx={{ display: 'grid', gridTemplateColumns: '300px minmax(0, 1fr)', minHeight: 560, borderRadius: '15px', overflow: 'hidden' }}>
      <Box sx={{ borderRight: `1px solid ${theme.app.line}`, bgcolor: theme.app.panelAlt, p: 1.25, overflowY: 'auto' }}>
        {tree.map((node) => (
          <Box key={node.root.id} sx={{ mb: 2 }}>
            <Typography sx={{ ...caption, px: 1.25, py: 1, font: `11px ${monoFontFamily}`, textTransform: 'none', letterSpacing: 0, overflowWrap: 'anywhere' }}>{node.root.path}</Typography>
            {node.hosts.map((h) => (
              <Box key={h.host}>
                <Typography sx={{ fontSize: 12, fontWeight: 730, px: 1.25, pt: 1 }}>{h.host || t('explorer.noRemote')}</Typography>
                {h.namespaces.map((ns) => (
                  <Box key={ns.namespace}>
                    {ns.namespace && <Typography sx={{ ...caption, pl: 3, pt: 0.75 }}>{ns.namespace}</Typography>}
                    {ns.repos.map((r) => (
                      <ButtonBase
                        key={r.id}
                        onClick={() => select(r.id)}
                        sx={{
                          display: 'flex',
                          width: '100%',
                          justifyContent: 'flex-start',
                          pl: 3.5,
                          pr: 1.25,
                          py: 0.9,
                          borderRadius: '7px',
                          fontSize: 12,
                          color: r.id === selectedId ? 'primary.main' : 'text.secondary',
                          bgcolor: r.id === selectedId ? theme.app.tabActive : 'transparent',
                          fontWeight: r.id === selectedId ? 760 : 400,
                        }}
                      >
                        {repoName(r)}
                      </ButtonBase>
                    ))}
                  </Box>
                ))}
              </Box>
            ))}
          </Box>
        ))}
      </Box>
      <Box sx={{ p: 3.5 }}>
        {!selected ? (
          <EmptyText text={t('explorer.pickRepo')} />
        ) : (
          <Box>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
              <RepoIcon repo={selected} size={40} />
              <Typography variant="h1">{repoName(selected)}</Typography>
              <Tooltip title={t('action.refreshRepo')}>
                <span>
                  <IconButton size="small" aria-label={t('action.refreshRepo')} disabled={refreshing} onClick={() => void refresh(selected.id)}>
                    {refreshing ? <CircularProgress size={18} /> : <RefreshIcon fontSize="small" />}
                  </IconButton>
                </span>
              </Tooltip>
            </Box>
            <Typography sx={{ font: `11px ${monoFontFamily}`, color: 'text.secondary', mt: 1, overflowWrap: 'anywhere' }}>
              {selected.path}
            </Typography>
            <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mt: 2 }}>
              <Button size="small" variant="outlined" disabled={Boolean(running)} onClick={() => requestOp(selected.id, Operation.OperationFetch)}>
                {t('action.fetch')}
              </Button>
              <Button size="small" variant="outlined" disabled={Boolean(running)} onClick={() => requestOp(selected.id, Operation.OperationPull)}>
                {t('action.pull')}
              </Button>
              {running ? (
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <CircularProgress size={14} />
                  <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>{t('op.running', { op: opName(running) })}</Typography>
                </Box>
              ) : (
                lastOp && <StatusText wrap label={opResultLabel(lastOp)} />
              )}
            </Box>
            <Box sx={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: 1.5, mt: 3, alignItems: 'center' }}>
              <Typography sx={caption}>{t('column.branch')}</Typography>
              <Box>
                <BranchChip repo={selected} />
              </Box>
              <Typography sx={caption}>{t('column.changes')}</Typography>
              <ChangesIndicator repo={selected} mode="inline" />
              <Typography sx={caption}>{t('column.sync')}</Typography>
              <SyncIndicator repo={selected} mode="inline" />
              {/* A09: ahead/behind — по последнему fetch, а не живое состояние сервера. */}
              <Typography sx={caption}>{t('explorer.fetch')}</Typography>
              <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>
                {selected.branch?.lastFetchAt
                  ? t('explorer.fetchedAt', { time: formatRelative(selected.branch.lastFetchAt) })
                  : t('explorer.neverFetched')}
              </Typography>
              <Typography sx={caption}>{t('explorer.status')}</Typography>
              <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>{t('explorer.statusAt', { time: formatRelative(selected.updatedAt) })}</Typography>
              <Typography sx={caption}>{t('column.pr')}</Typography>
              <PRIndicator repo={selected} mode="inline" />
              <Typography sx={caption}>{t('column.ci')}</Typography>
              <CIIndicator repo={selected} mode="inline" />
            </Box>
            <Typography sx={{ ...caption, mt: 3.5, mb: 1.5 }}>{t('explorer.branches')}</Typography>
            <Branches repo={selected} />
            {selected.error && (
              <Typography sx={{ mt: 3, color: 'error.main', fontSize: 12 }}>
                {t('repo.readError')}: {selected.error.message}
              </Typography>
            )}
          </Box>
        )}
      </Box>
    </Paper>
  )
}
