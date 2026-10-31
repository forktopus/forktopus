// Панель деталей репозитория (выезжает справа): локальное состояние,
// безопасные операции и серверный блок. Открывается из любого режима
// через store.openDetails.

import CloseIcon from '@mui/icons-material/Close'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import RefreshIcon from '@mui/icons-material/Refresh'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Divider,
  Drawer,
  IconButton,
  MenuItem,
  Select,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material'
import { useEffect, useState, type ReactNode } from 'react'
import { useNow } from '../app/useNow'
import { Nesting, Operation, Provider, type BranchInfo, type Repository } from '../data/model'
import { formatRelative, t, type MessageKey } from '../i18n'
import { opName, opResultLabel, pullBlock, switchBlock } from '../logic/ops'
import { repoHost, repoName, repoNamespace, repoProvider } from '../logic/repo'
import { integrationIssue } from '../logic/status'
import { availabilityTone, toolHint } from '../logic/tools'
import { useApp } from '../store/context'
import { monoFontFamily } from '../theme/tokens'
import { CIIndicator, ChangesIndicator, PRIndicator, SyncIndicator } from './indicators'
import { BranchChip, RepoIcon, StatusText, caption } from './status'

const providerName: Record<Provider, MessageKey> = {
  [Provider.$zero]: 'provider.other',
  [Provider.ProviderGitHub]: 'provider.github',
  [Provider.ProviderGitLab]: 'provider.gitlab',
  [Provider.ProviderGitea]: 'provider.gitea',
  [Provider.ProviderOther]: 'provider.other',
}

const nestingName: Partial<Record<Nesting, MessageKey>> = {
  [Nesting.NestingSubmodule]: 'nesting.submodule',
  [Nesting.NestingNested]: 'nesting.nested',
}

function Row({ name, children }: { name: string; children: ReactNode }) {
  return (
    <>
      <Typography sx={{ ...caption, pt: 0.4 }}>{name}</Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  const theme = useTheme()
  return (
    <Box sx={{ border: `1px solid ${theme.app.line}`, borderRadius: '12px', p: 2, mt: 2 }}>
      <Typography sx={{ ...caption, mb: 1.5 }}>{title}</Typography>
      {children}
    </Box>
  )
}

/** Кнопка, которая при блокировке объясняет причину подсказкой. */
function GuardedButton({ reason, disabled, onClick, children }: { reason: string | null; disabled: boolean; onClick(): void; children: ReactNode }) {
  const button = (
    <span>
      <Button size="small" variant="outlined" disabled={disabled || reason !== null} onClick={onClick}>
        {children}
      </Button>
    </span>
  )
  return reason ? <Tooltip title={reason}>{button}</Tooltip> : button
}

function SwitchControl({ repo, busy }: { repo: Repository; busy: boolean }) {
  const loadBranches = useApp((s) => s.loadBranches)
  const requestOp = useApp((s) => s.requestOp)
  const [branches, setBranches] = useState<BranchInfo[] | null>(null)
  const [target, setTarget] = useState('')
  useEffect(() => {
    let alive = true
    setBranches(null)
    setTarget('')
    void loadBranches(repo.id).then((list) => alive && setBranches(list ?? []))
    return () => {
      alive = false
    }
  }, [repo.id, repo.updatedAt, loadBranches])

  if (branches === null) return <CircularProgress size={16} />
  const others = branches.filter((b) => !b.current)
  if (others.length === 0) return <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>{t('details.noOtherBranches')}</Typography>
  const chosen = others.find((b) => b.name === target)
  // A15: ветку другого worktree переключить нельзя — предупреждаем заранее.
  const reason = switchBlock(repo) ?? (chosen?.worktreePath ? t('explorer.branchBusy', { path: chosen.worktreePath }) : null)
  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
      <Select size="small" value={target} displayEmpty onChange={(e) => setTarget(e.target.value)} sx={{ flex: 1, minWidth: 0, font: `12px ${monoFontFamily}` }}>
        <MenuItem value="" disabled>
          {t('details.pickBranch')}
        </MenuItem>
        {others.map((b) => (
          <MenuItem key={b.name} value={b.name} sx={{ font: `12px ${monoFontFamily}` }}>
            {b.name}
            {b.worktreePath ? ` — ${t('explorer.branchBusy', { path: b.worktreePath })}` : ''}
          </MenuItem>
        ))}
      </Select>
      <GuardedButton reason={target ? reason : null} disabled={busy || !target} onClick={() => requestOp(repo.id, Operation.OperationSwitch, target)}>
        {t('action.switch')}
      </GuardedButton>
    </Box>
  )
}

function LocalSection({ repo }: { repo: Repository }) {
  const now = useNow()
  const requestOp = useApp((s) => s.requestOp)
  const refreshRepo = useApp((s) => s.refreshRepo)
  const running = useApp((s) => s.running[repo.id])
  const lastOp = useApp((s) => s.lastOps[repo.id])
  const [refreshing, setRefreshing] = useState(false)
  const busy = Boolean(running)
  const b = repo.branch

  const refresh = async () => {
    setRefreshing(true)
    await refreshRepo(repo.id)
    setRefreshing(false)
  }

  return (
    <Section title={t('details.local')}>
      <Box sx={{ display: 'grid', gridTemplateColumns: '120px minmax(0, 1fr)', gap: 1.25, alignItems: 'start' }}>
        <Row name={t('column.branch')}>
          <BranchChip repo={repo} />
          {b?.upstream && (
            <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 0.5 }}>
              {t('details.tracks', { upstream: b.upstream })}
            </Typography>
          )}
        </Row>
        <Row name={t('column.changes')}>
          <ChangesIndicator repo={repo} mode="inline" />
        </Row>
        <Row name={t('column.sync')}>
          <SyncIndicator repo={repo} mode="inline" />
        </Row>
        <Row name={t('explorer.status')}>
          <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>{t('explorer.statusAt', { time: formatRelative(repo.updatedAt, now) })}</Typography>
        </Row>
      </Box>
      {repo.error && (
        <Alert severity="error" sx={{ mt: 1.5 }}>
          {t('repo.readError')}: {repo.error.message}
        </Alert>
      )}
      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mt: 2 }}>
        <GuardedButton reason={repo.remote ? null : t('details.noRemote')} disabled={busy} onClick={() => requestOp(repo.id, Operation.OperationFetch)}>
          {t('action.fetch')}
        </GuardedButton>
        <GuardedButton reason={pullBlock(repo)} disabled={busy} onClick={() => requestOp(repo.id, Operation.OperationPull)}>
          {t('action.pull')}
        </GuardedButton>
        <Tooltip title={t('action.refreshRepo')}>
          <span>
            <IconButton size="small" aria-label={t('action.refreshRepo')} disabled={refreshing} onClick={() => void refresh()}>
              {refreshing ? <CircularProgress size={18} /> : <RefreshIcon fontSize="small" />}
            </IconButton>
          </span>
        </Tooltip>
      </Box>
      <Typography sx={{ ...caption, mt: 2, mb: 1 }}>{t('details.switch')}</Typography>
      <SwitchControl repo={repo} busy={busy} />
      <Box sx={{ mt: 1.5, minHeight: 20 }}>
        {running ? (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <CircularProgress size={14} />
            <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>{t('op.running', { op: opName(running) })}</Typography>
          </Box>
        ) : (
          lastOp && <StatusText wrap label={opResultLabel(lastOp)} />
        )}
      </Box>
    </Section>
  )
}

function RemoteSection({ repo, onOpenTools }: { repo: Repository; onOpenTools(): void }) {
  const theme = useTheme()
  const tools = useApp((s) => s.tools)
  const host = repoHost(repo)
  const provider = repoProvider(repo)
  const tool = host ? tools.find((d) => d.host === host) : undefined
  const hint = tool ? toolHint(tool) : null

  return (
    <Section title={t('details.remote')}>
      <Box sx={{ display: 'grid', gridTemplateColumns: '120px minmax(0, 1fr)', gap: 1.25, alignItems: 'start' }}>
        <Row name={t('details.server')}>
          <Typography sx={{ fontSize: 13, fontWeight: 700 }}>{host ? `${t(providerName[provider])} · ${host}` : t('repo.noRemote')}</Typography>
        </Row>
        {tool && (
          <Row name="CLI">
            <StatusText label={{ tone: availabilityTone[tool.availability] ?? 'muted', text: `${tool.name} · ${t(`availability.${tool.availability || 'unavailable'}`)}` }} />
            {hint && (
              <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 0.5 }}>
                {hint.text}
                {hint.command && (
                  <Box component="code" sx={{ display: 'block', font: `11px ${monoFontFamily}`, bgcolor: theme.app.chip, borderRadius: '6px', px: 1, py: 0.5, mt: 0.5 }}>
                    {hint.command}
                  </Box>
                )}
              </Typography>
            )}
          </Row>
        )}
        <Row name={t('column.pr')}>
          <PRIndicator repo={repo} mode="inline" />
        </Row>
        <Row name={t('column.ci')}>
          <CIIndicator repo={repo} mode="inline" />
        </Row>
      </Box>
      {integrationIssue(repo) && (
        <Typography variant="caption" sx={{ display: 'block', color: 'text.secondary', mt: 1.5 }}>
          {t('details.zeroNote')}
        </Typography>
      )}
      <Button size="small" sx={{ mt: 1.5 }} onClick={onOpenTools}>
        {t('action.tools')}
      </Button>
    </Section>
  )
}

export function RepoDetails({ onOpenTools }: { onOpenTools(): void }) {
  const theme = useTheme()
  const detailsId = useApp((s) => s.detailsId)
  const repo = useApp((s) => (s.detailsId ? s.repos[s.detailsId] : undefined))
  const openDetails = useApp((s) => s.openDetails)
  const copyText = useApp((s) => s.copyText)
  const close = () => openDetails(null)
  const nesting = repo?.nesting ? nestingName[repo.nesting] : undefined

  return (
    <Drawer anchor="right" open={detailsId !== null && repo !== undefined} onClose={close} slotProps={{ paper: { sx: { width: 460, maxWidth: '100vw', bgcolor: theme.app.panel } } }}>
      {repo && (
        <Box sx={{ p: 3 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <RepoIcon repo={repo} />
            <Typography sx={{ flex: 1, fontSize: 11, color: 'text.secondary' }}>
              {[repoHost(repo) || t('repo.noRemote'), repoNamespace(repo)].filter(Boolean).join(' / ')}
            </Typography>
            <IconButton aria-label={t('action.close')} onClick={close}>
              <CloseIcon />
            </IconButton>
          </Box>
          <Typography variant="h2" sx={{ mt: 1.5, overflowWrap: 'anywhere' }}>
            {repoName(repo)}
          </Typography>
          <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 0.5, mt: 0.5 }}>
            <Typography sx={{ flex: 1, font: `11px ${monoFontFamily}`, color: 'text.secondary', overflowWrap: 'anywhere', pt: 0.5 }}>{repo.path}</Typography>
            <Tooltip title={t('action.copyPath')}>
              <IconButton size="small" aria-label={t('action.copyPath')} onClick={() => void copyText(repo.path)}>
                <ContentCopyIcon sx={{ fontSize: 15 }} />
              </IconButton>
            </Tooltip>
          </Box>
          {nesting && <Typography sx={{ fontSize: 12, color: 'text.secondary', mt: 0.5 }}>{t(nesting)}</Typography>}
          <LocalSection repo={repo} />
          <RemoteSection repo={repo} onOpenTools={onOpenTools} />
          <Divider sx={{ my: 2 }} />
          <Typography variant="caption" sx={{ color: 'text.secondary' }}>
            {t('details.safety')}
          </Typography>
        </Box>
      )}
    </Drawer>
  )
}
