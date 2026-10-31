// Черновой проводник: дерево корень → хост → namespace → репозиторий и
// краткие сведения о выбранном. Полноценный экран (ветки, Switch) — этап 12.

import { Box, ButtonBase, Paper, Typography, useTheme } from '@mui/material'
import { useMemo } from 'react'
import { BranchChip, RepoIcon, StatusText } from '../components/status'
import { t } from '../i18n'
import { buildTree } from '../logic/explorer'
import { repoName } from '../logic/repo'
import { changesLabel, ciLabel, prLabel, syncLabel } from '../logic/status'
import { useApp } from '../store/context'
import { useSearchedRepos } from '../store/selectors'
import { monoFontFamily } from '../theme/tokens'
import { EmptyText } from './EmptyText'

const caption = { fontSize: 10, fontWeight: 800, letterSpacing: '1.3px', textTransform: 'uppercase', color: 'text.secondary' } as const

export function ExplorerView() {
  const theme = useTheme()
  const roots = useApp((s) => s.roots)
  const selectedId = useApp((s) => s.selectedId)
  const select = useApp((s) => s.select)
  const repos = useSearchedRepos()
  const tree = useMemo(() => buildTree(roots, repos), [roots, repos])
  const selected = repos.find((r) => r.id === selectedId) ?? null

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
            </Box>
            <Typography sx={{ font: `11px ${monoFontFamily}`, color: 'text.secondary', mt: 1, overflowWrap: 'anywhere' }}>
              {selected.path}
            </Typography>
            <Box sx={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: 1.5, mt: 3, alignItems: 'center' }}>
              <Typography sx={caption}>{t('column.branch')}</Typography>
              <Box>
                <BranchChip repo={selected} />
              </Box>
              <Typography sx={caption}>{t('column.changes')}</Typography>
              <StatusText label={changesLabel(selected)} />
              <Typography sx={caption}>{t('column.sync')}</Typography>
              <StatusText label={syncLabel(selected)} />
              <Typography sx={caption}>{t('column.pr')}</Typography>
              <StatusText label={prLabel(selected)} />
              <Typography sx={caption}>{t('column.ci')}</Typography>
              <StatusText label={ciLabel(selected)} />
            </Box>
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
