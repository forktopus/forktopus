// Проводник (02-explorer.png): слева дерево корень → хост → namespace →
// репозиторий, в центре выбранный репозиторий (ветка, состояние, рабочая
// копия, локальные ветки с безопасным switch), справа — сервер, PR/MR и CI.

import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined'
import RefreshIcon from '@mui/icons-material/Refresh'
import SubdirectoryArrowRightIcon from '@mui/icons-material/SubdirectoryArrowRight'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import { Alert, Box, Button, ButtonBase, CircularProgress, IconButton, Paper, Tooltip, Typography, useTheme } from '@mui/material'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useNow } from '../app/useNow'
import { CIIndicator, NoAutoCheckMark, PRIndicator } from '../components/indicators'
import { CliStatus, GuardedButton, nestingName, providerName } from '../components/RepoDetails'
import { ProviderIcon, RepoIcon, StatusText, caption } from '../components/status'
import { Availability, ErrorCode, Operation, type BranchInfo, type ChangedFile, type Repository } from '../data/model'
import { formatRelative, t } from '../i18n'
import { buildTree, healthTone, treeRepos, type RepoNode, type RootNode } from '../logic/explorer'
import { opName, opResultLabel, pullBlock, switchBlock } from '../logic/ops'
import { branchOf, repoHost, repoName, repoNamespace, repoProvider } from '../logic/repo'
import { changeCount, ciLabel, fileLabel, hasStatus, integrationIssue, prLabel, remoteCaption, remoteCount, syncFreshness, syncLabel } from '../logic/status'
import { useApp } from '../store/context'
import { useSearchedRepos } from '../store/selectors'
import { toneColor } from '../theme/theme'
import { monoFontFamily } from '../theme/tokens'
import { EmptyText } from './EmptyText'
import { PathButtons } from '../components/PathButtons'

const sectionTitle = { ...caption, fontSize: 12, letterSpacing: '1.4px', mb: 1.5 } as const

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Box sx={{ mt: 3 }}>
      <Typography sx={sectionTitle}>{title}</Typography>
      {children}
    </Box>
  )
}

function Card({ children, sx }: { children: ReactNode; sx?: object }) {
  const theme = useTheme()
  return <Box sx={{ border: `1px solid ${theme.app.line}`, borderRadius: '11px', p: 1.75, minWidth: 0, ...sx }}>{children}</Box>
}

// --- дерево ---

function Dot({ repo }: { repo: Repository }) {
  const theme = useTheme()
  return <Box component="i" sx={{ width: 6, height: 6, borderRadius: '50%', flexShrink: 0, ml: 'auto', bgcolor: toneColor(theme, healthTone(repo)) }} />
}

function TreeRepo({ node, depth, selectedId }: { node: RepoNode; depth: number; selectedId: string | null }) {
  const theme = useTheme()
  const select = useApp((s) => s.select)
  const r = node.repo
  const active = r.id === selectedId
  const Icon = depth > 0 ? SubdirectoryArrowRightIcon : FolderOutlinedIcon
  return (
    <>
      <ButtonBase
        onClick={() => select(r.id)}
        aria-current={active || undefined}
        sx={{
          display: 'flex',
          gap: 0.875,
          width: '100%',
          justifyContent: 'flex-start',
          pl: 3.5 + depth * 1.75,
          pr: 1.25,
          py: 0.9,
          my: '1px',
          borderRadius: '7px',
          fontSize: 13,
          textAlign: 'left',
          color: active ? 'primary.main' : 'text.secondary',
          bgcolor: active ? theme.app.tabActive : 'transparent',
          fontWeight: active ? 760 : 400,
          '&:hover': { bgcolor: active ? theme.app.tabActive : theme.app.chip },
        }}
      >
        <Icon sx={{ fontSize: 15, flexShrink: 0 }} />
        <Box component="span" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {repoName(r)}
        </Box>
        <NoAutoCheckMark repo={r} size={13} />
        <Dot repo={r} />
      </ButtonBase>
      {node.children.map((c) => (
        <TreeRepo key={c.repo.id} node={c} depth={depth + 1} selectedId={selectedId} />
      ))}
    </>
  )
}

function Toggle({ open, onClick, indent = 0, children }: { open: boolean; onClick(): void; indent?: number; children: ReactNode }) {
  const theme = useTheme()
  const Icon = open ? ExpandMoreIcon : ChevronRightIcon
  return (
    <ButtonBase
      onClick={onClick}
      aria-expanded={open}
      sx={{
        display: 'flex',
        width: '100%',
        justifyContent: 'flex-start',
        alignItems: 'center',
        gap: 0.75,
        pl: 0.5 + indent,
        pr: 0.5,
        py: 0.9,
        borderRadius: '7px',
        textAlign: 'left',
        '&:hover': { bgcolor: theme.app.chip },
      }}
    >
      <Icon sx={{ fontSize: 17, color: 'text.secondary', flexShrink: 0 }} />
      {children}
    </ButtonBase>
  )
}

function Tree({ tree, selectedId, rootsShown }: { tree: RootNode[]; selectedId: string | null; rootsShown: boolean }) {
  const collapsedKeys = useApp((s) => s.explorerCollapsed)
  const collapsed = useMemo(() => new Set(collapsedKeys), [collapsedKeys])
  const toggle = useApp((s) => s.toggleExplorerNode)

  return tree.map(({ root, hosts }) => {
    const rootOpen = !collapsed.has(root.id)
    const body =
      hosts.length === 0 ? (
        <Typography sx={{ fontSize: 12, color: 'text.secondary', px: 3.5, py: 0.5 }}>{t('explorer.emptyRoot')}</Typography>
      ) : (
        hosts.map((h) => {
          const key = `${root.id}\n${h.host}`
          const open = !collapsed.has(key)
          return (
            <Box key={h.host}>
              <Toggle open={open} onClick={() => toggle(key)}>
                {h.host && <ProviderIcon provider={h.provider} size={18} />}
                <Typography component="span" sx={{ flex: 1, minWidth: 0, fontSize: 13, fontWeight: 730, overflowWrap: 'anywhere' }}>
                  {h.host || t('explorer.noRemote')}
                </Typography>
                {h.issue && (
                  <Tooltip title={t('registry.sourceIssue')}>
                    <WarningAmberIcon sx={{ fontSize: 16, color: 'warning.main', mr: 0.75 }} />
                  </Tooltip>
                )}
              </Toggle>
              {open &&
                h.namespaces.map((ns) => {
                  // Без namespace (локальный путь, remote без группы) сворачивать нечего — репозитории сразу под хостом.
                  const nsKey = `${key}\n${ns.namespace}`
                  const nsOpen = !ns.namespace || !collapsed.has(nsKey)
                  return (
                    <Box key={ns.namespace}>
                      {ns.namespace && (
                        <Toggle open={nsOpen} onClick={() => toggle(nsKey)} indent={1.75}>
                          <Typography
                            component="span"
                            sx={{ ...caption, fontWeight: 600, letterSpacing: '1px', flex: 1, minWidth: 0, overflowWrap: 'anywhere' }}
                          >
                            {ns.namespace}
                          </Typography>
                        </Toggle>
                      )}
                      {nsOpen && ns.repos.map((n) => <TreeRepo key={n.repo.id} node={n} depth={0} selectedId={selectedId} />)}
                    </Box>
                  )
                })}
            </Box>
          )
        })
      )
    // Один корень — без лишнего уровня: сразу хосты.
    if (!rootsShown) return <Box key={root.id}>{body}</Box>
    return (
      <Box key={root.id} sx={{ mb: 1 }}>
        <Toggle open={rootOpen} onClick={() => toggle(root.id)}>
          <Typography component="span" sx={{ font: `12px ${monoFontFamily}`, color: 'text.secondary', overflowWrap: 'anywhere' }}>
            {root.path}
          </Typography>
        </Toggle>
        {rootOpen && <Box sx={{ pl: 1 }}>{body}</Box>}
      </Box>
    )
  })
}

// --- выбранный репозиторий ---

function MiniCard({ name, value, note }: { name: string; value: number | null; note: string }) {
  return (
    <Card>
      <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>{name}</Typography>
      <Typography sx={{ fontSize: 25, fontWeight: 760, mt: 0.5 }}>{value ?? t('kpi.none')}</Typography>
      <Typography sx={{ fontSize: 12, color: 'text.secondary', mt: 0.25, overflowWrap: 'anywhere' }}>{note}</Typography>
    </Card>
  )
}

function CurrentBranch({ repo }: { repo: Repository }) {
  const now = useNow()
  const b = repo.branch
  if (!b) return <StatusText label={syncLabel(repo)} />
  const name = b.detached ? t('explorer.detachedAt', { sha: (b.headSha ?? '').slice(0, 8) }) : (b.name ?? '')
  const counts = b.upstream && !b.detached && !b.upstreamGone && b.ahead != null && b.behind != null
  return (
    <Card sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 2, p: 2 }}>
      <Box sx={{ minWidth: 0 }}>
        <Typography sx={{ font: `700 14px ${monoFontFamily}`, color: 'primary.main', overflowWrap: 'anywhere' }}>⑂ {name}</Typography>
        <Typography sx={{ fontSize: 12, color: 'text.secondary', mt: 0.5, overflowWrap: 'anywhere' }}>
          {b.upstream ? t('details.tracks', { upstream: b.upstream }) : t('explorer.noUpstream')}
        </Typography>
      </Box>
      <Box sx={{ textAlign: 'right', flexShrink: 0 }}>
        <StatusText label={syncLabel(repo)} />
        {counts && (
          <Typography sx={{ fontSize: 12, color: 'text.secondary', mt: 0.5 }}>{t('explorer.aheadBehind', { ahead: b.ahead!, behind: b.behind! })}</Typography>
        )}
        {/* A09: ahead/behind — по последнему fetch, а не живое состояние сервера. */}
        <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>{syncFreshness(repo, now)}</Typography>
      </Box>
    </Card>
  )
}

function Worktree({ repo }: { repo: Repository }) {
  const theme = useTheme()
  const w = repo.worktree
  if (!w) return <StatusText label={{ tone: 'muted', text: t('status.unknown') }} />
  const rows = [
    { key: 'explorer.wt.staged' as const, count: w.staged, tone: 'info' as const },
    { key: 'explorer.wt.unstaged' as const, count: w.unstaged, tone: 'warn' as const },
    { key: 'explorer.wt.untracked' as const, count: w.untracked, tone: 'warn' as const },
    { key: 'explorer.wt.conflicts' as const, count: w.conflicts, tone: 'bad' as const },
  ].filter((r) => r.count > 0)
  const row = {
    display: 'flex',
    justifyContent: 'space-between',
    gap: 1.5,
    px: 1.5,
    py: 1.25,
    fontSize: 13,
    '& + &': { borderTop: `1px solid ${theme.app.line}` },
  }
  return (
    <Box sx={{ border: `1px solid ${theme.app.line}`, borderRadius: '10px', overflow: 'hidden' }}>
      {w.operation && (
        <Box sx={row}>
          <span>{t('explorer.wt.operation')}</span>
          <StatusText label={{ tone: 'bad', text: w.operation }} />
        </Box>
      )}
      {rows.map((r) => (
        <Box key={r.key} sx={row}>
          <span>{t(r.key)}</span>
          <StatusText label={{ tone: r.tone, text: String(r.count) }} />
        </Box>
      ))}
      {rows.length === 0 && !w.operation && <Box sx={{ ...row, color: toneColor(theme, 'good') }}>{t('explorer.wt.clean')}</Box>}
      {(w.files?.length ?? 0) > 0 && <ChangedFiles files={w.files!} more={w.moreFiles ?? 0} />}
    </Box>
  )
}

/** Изменённые файлы (как `git status --short`): код, путь, подпись. Только просмотр. */
function ChangedFiles({ files, more }: { files: ChangedFile[]; more: number }) {
  const theme = useTheme()
  return (
    <Box sx={{ borderTop: `1px solid ${theme.app.line}` }}>
      <Typography sx={{ ...caption, px: 1.5, pt: 1.25, pb: 0.5 }}>{t('explorer.wt.files')}</Typography>
      <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0, maxHeight: 360, overflowY: 'auto' }}>
        {files.map((f) => (
          <Box
            component="li"
            key={f.path}
            title={f.origPath ? `${f.origPath} → ${f.path}` : f.path}
            sx={{ display: 'flex', alignItems: 'baseline', gap: 1.25, px: 1.5, py: 0.75, fontSize: 13, '& + &': { borderTop: `1px solid ${theme.app.line}` } }}
          >
            <Box component="code" sx={{ font: `12px ${monoFontFamily}`, whiteSpace: 'pre', color: 'text.secondary', flexShrink: 0 }}>
              {f.code}
            </Box>
            <Box component="span" sx={{ font: `13px ${monoFontFamily}`, minWidth: 0, flex: 1, overflowWrap: 'anywhere' }}>
              {f.origPath && (
                <Box component="span" sx={{ color: 'text.secondary' }}>
                  {f.origPath} →{' '}
                </Box>
              )}
              {f.path}
            </Box>
            <Box sx={{ flexShrink: 0 }}>
              <StatusText label={fileLabel(f)} />
            </Box>
          </Box>
        ))}
      </Box>
      {more > 0 && <Typography sx={{ fontSize: 12, color: 'text.secondary', px: 1.5, py: 1 }}>{t('explorer.wt.moreFiles', { count: more })}</Typography>}
    </Box>
  )
}

/** Локальные ветки; перечитываются при смене выбора и после обновления статуса. */
function Branches({ repo, busy }: { repo: Repository; busy: boolean }) {
  const theme = useTheme()
  const now = useNow()
  const loadBranches = useApp((s) => s.loadBranches)
  const requestOp = useApp((s) => s.requestOp)
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
  if (branches.length === 0) return <Typography sx={{ fontSize: 13, color: 'text.secondary' }}>{t('explorer.noBranches')}</Typography>
  const blocked = switchBlock(repo)
  return (
    <Box sx={{ border: `1px solid ${theme.app.line}`, borderRadius: '10px', overflow: 'hidden' }}>
      {branches.map((b) => {
        // A15: ветку другого worktree переключить нельзя — предупреждаем заранее.
        const reason = blocked ?? (b.worktreePath ? t('explorer.branchBusy', { path: b.worktreePath }) : null)
        return (
          <Box
            key={b.name}
            sx={{
              display: 'grid',
              gridTemplateColumns: 'minmax(0, 1fr) auto auto',
              gap: 1.5,
              alignItems: 'center',
              px: 1.5,
              py: 1,
              '& + &': { borderTop: `1px solid ${theme.app.line}` },
            }}
          >
            <Box sx={{ minWidth: 0 }}>
              <Typography
                sx={{
                  font: `13px ${monoFontFamily}`,
                  fontWeight: b.current ? 760 : 400,
                  color: b.current ? 'primary.main' : 'text.primary',
                  overflowWrap: 'anywhere',
                }}
              >
                {b.name}
              </Typography>
              <Typography sx={{ fontSize: 12, color: 'text.secondary', overflowWrap: 'anywhere' }}>
                {[
                  b.current && t('explorer.branchCurrent'),
                  b.upstream ? t('details.tracks', { upstream: b.upstream }) : t('explorer.noUpstream'),
                  !b.current && b.worktreePath && t('explorer.branchBusy', { path: b.worktreePath }),
                  b.commitAt && t('explorer.commitAt', { time: formatRelative(b.commitAt, now) }),
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </Typography>
            </Box>
            <StatusText label={syncLabel({ ...repo, branch: { ...b, detached: false } })} />
            <Box sx={{ minWidth: 104, textAlign: 'right' }}>
              {!b.current && (
                <GuardedButton variant="text" reason={reason} disabled={busy} onClick={() => requestOp(repo.id, Operation.OperationSwitch, b.name)}>
                  {t('action.switch')}
                </GuardedButton>
              )}
            </Box>
          </Box>
        )
      })}
    </Box>
  )
}

function RepoMain({ repo }: { repo: Repository }) {
  const now = useNow()
  const requestOp = useApp((s) => s.requestOp)
  const refreshRepo = useApp((s) => s.refreshRepo)
  const copyText = useApp((s) => s.copyText)
  const running = useApp((s) => s.running[repo.id])
  const lastOp = useApp((s) => s.lastOps[repo.id])
  const [refreshing, setRefreshing] = useState(false)
  const busy = Boolean(running)
  const nesting = repo.nesting ? nestingName[repo.nesting] : undefined
  const refresh = async () => {
    setRefreshing(true)
    await refreshRepo(repo.id)
    setRefreshing(false)
  }
  const crumbs = [repoHost(repo) || t('explorer.noRemote'), repoNamespace(repo), repoName(repo)].filter(Boolean)
  const pr = remoteCount(repo.pr, repo.pr.open)
  const ci = remoteCount(repo.ci, repo.ci.active)

  return (
    <Box sx={{ p: { xs: 2.5, md: 3.5 }, minWidth: 0 }}>
      <Typography sx={{ fontSize: 12, color: 'text.secondary', overflowWrap: 'anywhere' }}>
        {crumbs.map((c, i) => (
          <Box component="span" key={i} sx={i === crumbs.length - 1 ? { color: 'primary.main' } : undefined}>
            {i > 0 && ' › '}
            {c}
          </Box>
        ))}
      </Typography>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mt: 1.5 }}>
        <RepoIcon repo={repo} size={36} />
        <Typography variant="h1" sx={{ overflowWrap: 'anywhere', minWidth: 0 }}>
          {repoName(repo)}
          <NoAutoCheckMark repo={repo} size={22} />
        </Typography>
      </Box>
      <Typography sx={{ font: `12px ${monoFontFamily}`, color: 'text.secondary', mt: 1, overflowWrap: 'anywhere' }}>{repo.path}</Typography>
      {nesting && <Typography sx={{ fontSize: 13, color: 'text.secondary', mt: 0.5 }}>{t(nesting)}</Typography>}

      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mt: 2.5 }}>
        <GuardedButton variant="contained" reason={pullBlock(repo)} disabled={busy} onClick={() => requestOp(repo.id, Operation.OperationPull)}>
          {t('explorer.pull')}
        </GuardedButton>
        <GuardedButton reason={repo.remote ? null : t('details.noRemote')} disabled={busy} onClick={() => requestOp(repo.id, Operation.OperationFetch)}>
          {t('action.fetch')}
        </GuardedButton>
        <Button size="small" variant="outlined" startIcon={<ContentCopyIcon sx={{ fontSize: 15 }} />} onClick={() => void copyText(repo.path)}>
          {t('action.copyPath')}
        </Button>
        <PathButtons path={repo.path} />
        <Tooltip title={t('action.refreshRepo')}>
          <span>
            <IconButton size="small" aria-label={t('action.refreshRepo')} disabled={refreshing} onClick={() => void refresh()}>
              {refreshing ? <CircularProgress size={18} /> : <RefreshIcon fontSize="small" />}
            </IconButton>
          </span>
        </Tooltip>
      </Box>
      <Box sx={{ mt: 1.25, minHeight: 20 }}>
        {running ? (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <CircularProgress size={14} />
            <Typography sx={{ fontSize: 13, color: 'text.secondary' }}>{t('op.running', { op: opName(running) })}</Typography>
          </Box>
        ) : (
          lastOp && <StatusText wrap label={opResultLabel(lastOp)} />
        )}
      </Box>
      {repo.error && (
        <Alert severity="error" sx={{ mt: 1.5 }}>
          {t('repo.readError')}: {repo.error.message}
        </Alert>
      )}

      {/* На очень широком окне разделы — в две колонки, иначе строки тянутся на 2000+ px. */}
      <Box sx={{ display: 'grid', columnGap: 4, alignItems: 'start', '@media (min-width: 2200px)': { gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' } }}>
        <Section title={t('explorer.currentBranch')}>
          <CurrentBranch repo={repo} />
        </Section>

        <Section title={t('explorer.project')}>
          <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: 1.25 }}>
            <MiniCard name={t('explorer.changedFiles')} value={hasStatus(repo) ? changeCount(repo) : null} note={hasStatus(repo) ? '' : t('status.unknown')} />
            <MiniCard name={t('column.pr')} value={pr} note={pr == null || repo.pr.last ? prLabel(repo, now).text : ''} />
            <MiniCard name={t('explorer.activeCI')} value={ci} note={ci == null ? ciLabel(repo).text : ''} />
          </Box>
        </Section>

        <Section
          title={`${t('explorer.worktree')}${repo.worktree ? ` · ${t(changeCount(repo) + repo.worktree.conflicts > 0 ? 'explorer.wt.dirty' : 'explorer.wt.cleanShort')}` : ''}`}
        >
          <Worktree repo={repo} />
          <Typography sx={{ fontSize: 12, color: 'text.secondary', mt: 0.75 }}>
            {t('explorer.statusRead', { time: formatRelative(repo.updatedAt, now) })}
          </Typography>
        </Section>

        <Section title={t('explorer.branches')}>
          <Branches repo={repo} busy={busy} />
        </Section>
      </Box>
    </Box>
  )
}

function Inspector({ repo, onOpenTools }: { repo: Repository; onOpenTools(): void }) {
  const theme = useTheme()
  const host = repoHost(repo)
  const branch = branchOf(repo)
  const issue = integrationIssue(repo)
  // Сервер не поддерживается или remote нет — PR/CI не будет, это не проблема настройки.
  // Нет прав (D-068) — сервер есть, причина видна у PR и CI.
  const unsupported = (s: { availability?: Availability; error?: { code: ErrorCode } | null }) =>
    s.availability === Availability.AvailabilityNotSupported && s.error?.code !== ErrorCode.CodeAccessDenied
  const noServer = unsupported(repo.pr) && unsupported(repo.ci)
  const h3 = { ...sectionTitle, mt: 3, mb: 1.25 }
  return (
    <Box sx={{ p: 2.25, minWidth: 0 }}>
      <Typography sx={{ ...h3, mt: 1 }}>{t('explorer.remoteServer')}</Typography>
      <Card sx={{ bgcolor: theme.app.panel }}>
        <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>{t('explorer.connection')}</Typography>
        <Typography sx={{ fontSize: 14, fontWeight: 760, mt: 0.5, overflowWrap: 'anywhere' }}>
          {host ? `${t(providerName[repoProvider(repo)])} · ${host}` : remoteCaption(repo)}
        </Typography>
        <Box sx={{ mt: 0.75 }}>
          <CliStatus repo={repo} />
        </Box>
      </Card>

      <Typography sx={h3}>{t('explorer.prs')}</Typography>
      <Card sx={{ bgcolor: theme.app.panel }}>
        <Typography sx={{ fontSize: 12, color: 'text.secondary', mb: 0.75, overflowWrap: 'anywhere' }}>
          {branch ? t('explorer.forBranch', { branch }) : t('sync.detached')}
        </Typography>
        <PRIndicator repo={repo} mode="inline" />
      </Card>

      <Typography sx={h3}>{t('explorer.pipelines')}</Typography>
      <Card sx={{ bgcolor: theme.app.panel }}>
        <Typography sx={{ fontSize: 12, color: 'text.secondary', mb: 0.75 }}>{t('explorer.branchOnly')}</Typography>
        <CIIndicator repo={repo} mode="inline" />
      </Card>

      <Box
        sx={{
          mt: 2.5,
          p: 1.75,
          borderRadius: '11px',
          border: '1px dashed',
          borderColor: issue ? 'warning.main' : theme.app.line,
          color: issue ? 'warning.main' : 'text.secondary',
          fontSize: 13,
        }}
      >
        {issue ? t('explorer.cliIssue') : noServer ? t('explorer.noServer') : t('explorer.cliReady')}
        {/* Диагностика — только когда есть что чинить; в остальных случаях она в шапке. */}
        {issue && (
          <Button size="small" sx={{ display: 'block', mt: 1, px: 0 }} onClick={onOpenTools}>
            {t('explorer.openTools')}
          </Button>
        )}
      </Box>
    </Box>
  )
}

// Отступы липкого дерева от краёв окна: шапка (73 px, app/TopBar.tsx) и поля main (32 px).
const treeTop = 73 + 32
const treeBottom = 32

export function ExplorerView({ onOpenTools }: { onOpenTools(): void }) {
  const theme = useTheme()
  const roots = useApp((s) => s.roots)
  const selectedId = useApp((s) => s.selectedId)
  const repos = useSearchedRepos()
  const tree = useMemo(() => buildTree(roots, repos), [roots, repos])
  const ordered = useMemo(() => treeRepos(tree), [tree])
  // Без явного выбора (или выбранный скрыт поиском) — первый в дереве.
  const selected = ordered.find((r) => r.id === selectedId) ?? ordered[0] ?? null
  const issueHosts = new Set(tree.flatMap((n) => n.hosts.filter((h) => h.issue).map((h) => h.host))).size
  const line = `1px solid ${theme.app.line}`

  // Вход в проводник (вкладка, карточка радара): выбранный репозиторий — в
  // видимой части дерева, страница — сверху, а не на позиции прошлого режима.
  const treeRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    treeRef.current?.querySelector('[aria-current]')?.scrollIntoView({ block: 'center' })
    window.scrollTo(0, 0)
  }, [])

  return (
    <Paper
      sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: '280px minmax(0, 1fr)', lg: '300px minmax(0, 1fr) 320px', xl: '360px minmax(0, 1fr) 400px' },
        minHeight: 620,
        // До низа окна (main — flex-колонка): без пустоты под коротким содержимым.
        flex: '1 0 auto',
        borderRadius: '15px',
        // clip, а не hidden: hidden делает карточку контейнером прокрутки, и sticky дерева перестаёт работать.
        overflow: 'clip',
      }}
    >
      <Box
        sx={{
          gridRow: { md: 'span 2', lg: 'auto' },
          borderRight: { md: line },
          borderBottom: { xs: line, md: 'none' },
          bgcolor: theme.app.panelAlt,
          // Высота дерева не растягивает страницу: строку задают соседние колонки, дерево прокручивается само.
          contain: { md: 'size' },
        }}
      >
        <Box
          ref={treeRef}
          sx={{
            p: 1.25,
            display: 'flex',
            flexDirection: 'column',
            // Дерево — как боковая панель VS Code: стоит на месте при прокрутке страницы и прокручивается само.
            height: { md: '100%' },
            maxHeight: { xs: 320, md: `calc(100vh - ${treeTop + treeBottom}px)` },
            position: { md: 'sticky' },
            top: { md: treeTop },
            overflowY: 'auto',
          }}
        >
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', px: 1.25, pt: 1, pb: 1.5 }}>
            <Typography sx={caption}>{t('explorer.roots')}</Typography>
            <Typography sx={{ ...caption, textTransform: 'none', letterSpacing: 0 }}>{t('explorer.rootCount', { count: roots.length })}</Typography>
          </Box>
          <Box sx={{ flex: 1 }}>
            <Tree tree={tree} selectedId={selected?.id ?? null} rootsShown={roots.length > 1} />
          </Box>
          <Box sx={{ mt: 3, pt: 1.5, px: 1.25, borderTop: line, fontSize: 12, color: 'text.secondary' }}>
            <div>✓ {t('explorer.repoCount', { count: repos.length })}</div>
            {issueHosts > 0 && (
              <>
                <div>⚠ {t('explorer.issueHosts', { count: issueHosts })}</div>
                <Button size="small" sx={{ px: 0, mt: 0.5, fontSize: 12 }} onClick={onOpenTools}>
                  {t('explorer.openTools')}
                </Button>
              </>
            )}
          </Box>
        </Box>
      </Box>
      {selected ? (
        <>
          <RepoMain repo={selected} />
          <Box sx={{ bgcolor: theme.app.panelAlt, borderLeft: { lg: line }, borderTop: { xs: line, lg: 'none' } }}>
            <Inspector repo={selected} onOpenTools={onOpenTools} />
          </Box>
        </>
      ) : (
        <Box sx={{ p: 3.5, gridColumn: { lg: 'span 2' } }}>
          <EmptyText text={t('explorer.pickRepo')} />
        </Box>
      )}
    </Paper>
  )
}
