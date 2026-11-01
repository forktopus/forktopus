// «Показать в папке» и «Открыть в терминале» для каталога (D-066): карточка
// репозитория, заголовок в проводнике, окно проблемных каталогов.
// «Исключить из сканирования» — для каталога репозитория (D-071).

import FolderOpenIcon from '@mui/icons-material/FolderOpenOutlined'
import TerminalIcon from '@mui/icons-material/Terminal'
import VisibilityOffIcon from '@mui/icons-material/VisibilityOffOutlined'
import { Button, Tooltip } from '@mui/material'
import { t } from '../i18n'
import { useApp } from '../store/context'

export function PathButtons({ path, variant = 'outlined' }: { path: string; variant?: 'outlined' | 'text' }) {
  const openFolder = useApp((s) => s.openFolder)
  const openTerminal = useApp((s) => s.openTerminal)
  const sx = variant === 'text' ? { px: 0.75, minWidth: 0 } : undefined
  return (
    <>
      <Button size="small" variant={variant} startIcon={<FolderOpenIcon sx={{ fontSize: 15 }} />} onClick={() => openFolder(path)} sx={sx}>
        {t('action.openFolder')}
      </Button>
      <Button size="small" variant={variant} startIcon={<TerminalIcon sx={{ fontSize: 15 }} />} onClick={() => openTerminal(path)} sx={sx}>
        {t('action.openTerminal')}
      </Button>
    </>
  )
}

export function SkipRepoButton({ path, variant = 'outlined' }: { path: string; variant?: 'outlined' | 'text' }) {
  const skipPath = useApp((s) => s.skipPath)
  return (
    <Tooltip title={t('action.skipRepoHint')}>
      <Button
        size="small"
        variant={variant}
        color="inherit"
        startIcon={<VisibilityOffIcon sx={{ fontSize: 15 }} />}
        onClick={() => void skipPath(path)}
        sx={{ color: 'text.secondary', ...(variant === 'text' ? { px: 0.75, minWidth: 0 } : { borderColor: 'divider' }) }}
      >
        {t('action.skipRepo')}
      </Button>
    </Tooltip>
  )
}
