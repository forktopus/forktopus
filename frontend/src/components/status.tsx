import { Box, useTheme } from '@mui/material'
import type { Repository } from '../data/model'
import type { Label } from '../logic/status'
import { repoProvider } from '../logic/repo'
import { toneColor } from '../theme/theme'
import { monoFontFamily } from '../theme/tokens'

/** Подпись поля: мелкие заглавные. */
export const caption = { fontSize: 10, fontWeight: 800, letterSpacing: '1.3px', textTransform: 'uppercase', color: 'text.secondary' } as const

/** Точка и подпись в цвете тона статуса. wrap — для длинных сообщений (причина блокировки операции). */
export function StatusText({ label, wrap = false }: { label: Label; wrap?: boolean }) {
  const theme = useTheme()
  const color = toneColor(theme, label.tone)
  return (
    <Box
      component="span"
      sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.75, fontSize: 12, color, ...(wrap ? { minWidth: 0, overflowWrap: 'anywhere' } : { whiteSpace: 'nowrap' }) }}
    >
      <Box component="i" sx={{ width: 7, height: 7, borderRadius: '50%', bgcolor: color, flexShrink: 0 }} />
      {label.text}
    </Box>
  )
}

const providerSymbol = { github: 'GH', gitlab: 'GL', gitea: 'GT', other: 'GIT' } as const

export function RepoIcon({ repo, size = 33 }: { repo: Repository; size?: number }) {
  const theme = useTheme()
  const provider = repoProvider(repo)
  const key = provider === 'github' || provider === 'gitlab' || provider === 'gitea' ? provider : 'other'
  const colors = theme.app.provider[key]
  return (
    <Box
      sx={{
        width: size,
        height: size,
        borderRadius: '10px',
        display: 'grid',
        placeItems: 'center',
        flexShrink: 0,
        fontSize: 12,
        fontWeight: 900,
        bgcolor: colors.bg,
        color: colors.fg,
      }}
    >
      {providerSymbol[key]}
    </Box>
  )
}

/** Имя ветки моноширинным шрифтом; detached — короткий SHA. */
export function BranchChip({ repo }: { repo: Repository }) {
  const theme = useTheme()
  const b = repo.branch
  const text = b?.detached ? (b.headSha ?? '').slice(0, 8) : (b?.name ?? '')
  if (!text) return null
  return (
    <Box
      component="span"
      sx={{
        display: 'inline-block',
        maxWidth: 220,
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        verticalAlign: 'middle',
        font: `11px ${monoFontFamily}`,
        border: `1px solid ${theme.app.chipLine}`,
        bgcolor: theme.app.chip,
        borderRadius: '7px',
        px: 1,
        py: 0.5,
      }}
    >
      {text}
    </Box>
  )
}
