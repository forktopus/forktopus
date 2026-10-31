import AddIcon from '@mui/icons-material/Add'
import { Alert, Box, Button, CircularProgress, CssBaseline, ThemeProvider, Typography, useMediaQuery } from '@mui/material'
import { useMemo, useState } from 'react'
import { Notifications } from '../components/Notifications'
import { OpConfirmDialog } from '../components/OpConfirmDialog'
import { RepoDetails } from '../components/RepoDetails'
import { formatDateTime, t } from '../i18n'
import { integrationIssueCount } from '../logic/registry'
import { useApp } from '../store/context'
import { useRepos } from '../store/selectors'
import { createAppTheme } from '../theme/theme'
import { ExplorerView } from '../views/ExplorerView'
import { RadarView } from '../views/RadarView'
import { RegistryView } from '../views/RegistryView'
import { RootsDialog } from './RootsDialog'
import { ToolsDialog } from './ToolsDialog'
import { TopBar } from './TopBar'

function EmptyRoots({ onAdd }: { onAdd(): void }) {
  return (
    <Box sx={{ maxWidth: 560, mx: 'auto', mt: 10, textAlign: 'center' }}>
      <Box component="img" src="/forktopus-icon.png" alt={t('app.mascotAlt')} sx={{ width: 96, height: 96, borderRadius: '24px', mb: 3 }} />
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

function Content({ onOpenRoots, onOpenTools }: { onOpenRoots(): void; onOpenTools(): void }) {
  const ready = useApp((s) => s.ready)
  const roots = useApp((s) => s.roots)
  const mode = useApp((s) => s.mode)
  const scanning = useApp((s) => s.scanning)
  const stale = useApp((s) => s.stale)
  const lastCheckAt = useApp((s) => s.lastCheckAt)
  const filter = useApp((s) => s.filters.registry)
  const setFilters = useApp((s) => s.setFilters)
  const repos = useRepos()
  const issues = useMemo(() => integrationIssueCount(repos), [repos])

  if (!ready) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 2, mt: 12, color: 'text.secondary' }}>
        <CircularProgress size={22} />
        {t('app.loading')}
      </Box>
    )
  }
  if (roots.length === 0) return <EmptyRoots onAdd={onOpenRoots} />

  return (
    <>
      {stale && lastCheckAt && (
        <Alert severity="warning" sx={{ mb: 2.5 }} icon={scanning ? <CircularProgress size={18} color="inherit" /> : undefined}>
          {t(scanning ? 'scan.stale' : 'scan.staleIdle', { time: formatDateTime(lastCheckAt) })}
        </Alert>
      )}
      {issues > 0 && (
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
          {t('integration.banner', { count: issues })}
        </Alert>
      )}
      {repos.length === 0 && <Alert severity="info">{t(scanning ? 'empty.scanning' : 'empty.noRepos')}</Alert>}
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
  const [dialog, setDialog] = useState<'tools' | 'roots' | null>(null)

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <TopBar onOpenTools={() => setDialog('tools')} onOpenRoots={() => setDialog('roots')} />
      <Box component="main" sx={{ maxWidth: 1550, mx: 'auto', px: 4, py: 4 }}>
        <Content onOpenRoots={() => setDialog('roots')} onOpenTools={() => setDialog('tools')} />
      </Box>
      <ToolsDialog open={dialog === 'tools'} onClose={() => setDialog(null)} />
      <RootsDialog open={dialog === 'roots'} onClose={() => setDialog(null)} />
      <RepoDetails onOpenTools={() => setDialog('tools')} />
      <OpConfirmDialog />
      <Notifications />
    </ThemeProvider>
  )
}
