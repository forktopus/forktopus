import AddIcon from '@mui/icons-material/Add'
import { Alert, AlertTitle, Box, Button, CircularProgress, CssBaseline, ThemeProvider, Typography, useMediaQuery } from '@mui/material'
import { useMemo, useState } from 'react'
import { Notifications } from '../components/Notifications'
import { OpConfirmDialog } from '../components/OpConfirmDialog'
import { RepoDetails } from '../components/RepoDetails'
import { ErrorCode } from '../data/model'
import { formatDateTime, t } from '../i18n'
import { emptyRoots } from '../logic/explorer'
import { integrationIssueCount } from '../logic/registry'
import { gitProblem } from '../logic/tools'
import { useApp } from '../store/context'
import { useAllRepos, useRepos } from '../store/selectors'
import { createAppTheme } from '../theme/theme'
import { ExplorerView } from '../views/ExplorerView'
import { RadarView } from '../views/RadarView'
import { RegistryView } from '../views/RegistryView'
import { ProblemDirsDialog } from './ProblemDirsDialog'
import { RootsDialog } from './RootsDialog'
import { SettingsDialog } from './SettingsDialog'
import { ToolsDialog } from './ToolsDialog'
import { useZoomKeys } from './useZoomKeys'
import { TopBar } from './TopBar'

function EmptyRoots({ onAdd }: { onAdd(): void }) {
  return (
    <Box sx={{ maxWidth: 560, mx: 'auto', mt: 10, textAlign: 'center' }}>
      <Box component="img" src="/forktopus-icon.png" alt={t('app.mascotAlt')} sx={{ width: 96, height: 96, mb: 3 }} />
      <Typography variant="h1" sx={{ mb: 1.5 }}>
        {t('empty.noRoots.title')}
      </Typography>
      <Typography sx={{ color: 'text.secondary', mb: 3 }}>{t('empty.noRoots.text')}</Typography>
      <Button variant="contained" size="large" startIcon={<AddIcon />} onClick={onAdd}>
        {t('action.addRoot')}
      </Button>
    </Box>
  )
}

function Content({ onOpenRoots, onOpenTools, onOpenProblems }: { onOpenRoots(): void; onOpenTools(): void; onOpenProblems(): void }) {
  const ready = useApp((s) => s.ready)
  const roots = useApp((s) => s.roots)
  const mode = useApp((s) => s.mode)
  const scanning = useApp((s) => s.scanning)
  const stale = useApp((s) => s.stale)
  const lastCheckAt = useApp((s) => s.lastCheckAt)
  const filter = useApp((s) => s.filters.registry)
  const setFilters = useApp((s) => s.setFilters)
  const repos = useRepos()
  // Пустой корень — где сканер не нашёл ничего, а не где всё скрыто настройкой (D-070).
  const all = useAllRepos()
  const integrations = useMemo(() => integrationIssueCount(repos), [repos])
  const issues = useApp((s) => s.issues)
  const git = gitProblem(useApp((s) => s.tools))
  const empty = useMemo(() => emptyRoots(roots, all, issues), [roots, all, issues])

  if (!ready) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 2, mt: 12, color: 'text.secondary' }}>
        <CircularProgress size={22} />
        {t('app.loading')}
      </Box>
    )
  }
  // A28: без git статусов не будет ни на одном экране, в том числе после добавления корня.
  const gitBanner = git && (
    <Alert
      severity="error"
      sx={{ mb: 2.5 }}
      action={
        <Button color="inherit" size="small" onClick={onOpenTools}>
          {t('action.tools')}
        </Button>
      }
    >
      <AlertTitle>{t('git.missing.title')}</AlertTitle>
      {t('git.missing.text')}
      {/* Для «не найден» текст выше всё объясняет; прочие сбои (версия, запуск) — с деталью. */}
      {git.error?.message && git.error.code !== ErrorCode.CodeCLIMissing && (
        <Typography variant="caption" component="div" sx={{ mt: 0.5, opacity: 0.85 }}>
          {git.error.message}
        </Typography>
      )}
    </Alert>
  )

  if (roots.length === 0) {
    return (
      <>
        {gitBanner}
        <EmptyRoots onAdd={onOpenRoots} />
      </>
    )
  }

  // A04 и пустые корни: остальное просканировано; нечитаемые каталоги — в своём окне, пустые корни — в диалоге корней.
  const showEmpty = !scanning && !stale && repos.length > 0 ? empty : []
  const scanBanner = (issues.length > 0 || showEmpty.length > 0) && (
    <Alert
      severity="warning"
      sx={{ mb: 2.5 }}
      action={
        <>
          {issues.length > 0 && (
            <Button color="inherit" size="small" onClick={onOpenProblems}>
              {t('problems.title')}
            </Button>
          )}
          {showEmpty.length > 0 && (
            <Button color="inherit" size="small" onClick={onOpenRoots}>
              {t('roots.title')}
            </Button>
          )}
        </>
      }
    >
      {issues.length > 0 && <div>{t('scan.issues', { count: issues.length })}</div>}
      {showEmpty.length > 0 && <div>{t('scan.emptyRoots', { paths: showEmpty.map((r) => r.path).join(', ') })}</div>}
    </Alert>
  )

  return (
    <>
      {gitBanner}
      {stale && lastCheckAt && (
        <Alert severity="warning" sx={{ mb: 2.5 }} icon={scanning ? <CircularProgress size={18} color="inherit" /> : undefined}>
          {t(scanning ? 'scan.stale' : 'scan.staleIdle', { time: formatDateTime(lastCheckAt) })}
        </Alert>
      )}
      {scanBanner}
      {integrations > 0 && (
        <Alert
          severity="info"
          sx={{ mb: 2.5 }}
          action={
            <>
              {/* Отдельный фильтр незавершённой настройки (05 §3) — в реестре и радаре. */}
              {mode !== 'explorer' && filter !== 'integration' && (
                <Button color="inherit" size="small" onClick={() => setFilters({ registry: 'integration' })}>
                  {t('integration.show')}
                </Button>
              )}
              <Button color="inherit" size="small" onClick={onOpenTools}>
                {t('action.tools')}
              </Button>
            </>
          }
        >
          {t('integration.banner', { count: integrations })}
        </Alert>
      )}
      {repos.length === 0 && (
        <Alert
          severity="info"
          action={
            !scanning && (
              <Button color="inherit" size="small" onClick={onOpenRoots}>
                {t('roots.title')}
              </Button>
            )
          }
        >
          {scanning
            ? t('empty.scanning')
            : all.length > 0
              ? t('empty.allLocal', { count: all.length })
              : t('empty.noRepos', { paths: roots.map((r) => r.path).join(', ') })}
        </Alert>
      )}
      {repos.length > 0 && mode === 'registry' && <RegistryView />}
      {repos.length > 0 && mode === 'explorer' && <ExplorerView onOpenTools={onOpenTools} />}
      {repos.length > 0 && mode === 'radar' && <RadarView />}
    </>
  )
}

export function App() {
  const choice = useApp((s) => s.theme)
  const systemDark = useMediaQuery('(prefers-color-scheme: dark)')
  const mode = choice ?? (systemDark ? 'dark' : 'light')
  const theme = useMemo(() => createAppTheme(mode), [mode])
  const [dialog, setDialog] = useState<'tools' | 'roots' | 'settings' | 'problems' | null>(null)
  useZoomKeys()

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <TopBar onOpenTools={() => setDialog('tools')} onOpenRoots={() => setDialog('roots')} onOpenSettings={() => setDialog('settings')} />
      {/* Колонка на всю высоту под шапкой (73 px): проводник растягивается до низа окна. */}
      <Box
        component="main"
        sx={{ px: { xs: 2, md: 4 }, py: 4, display: 'flex', flexDirection: 'column', minHeight: 'calc(100vh - 73px)', boxSizing: 'border-box' }}
      >
        <Content onOpenRoots={() => setDialog('roots')} onOpenTools={() => setDialog('tools')} onOpenProblems={() => setDialog('problems')} />
      </Box>
      <ToolsDialog open={dialog === 'tools'} onClose={() => setDialog(null)} />
      <RootsDialog open={dialog === 'roots'} onClose={() => setDialog(null)} />
      <ProblemDirsDialog open={dialog === 'problems'} onClose={() => setDialog(null)} />
      <SettingsDialog open={dialog === 'settings'} onClose={() => setDialog(null)} />
      <RepoDetails onOpenTools={() => setDialog('tools')} />
      <OpConfirmDialog />
      <Notifications />
    </ThemeProvider>
  )
}
