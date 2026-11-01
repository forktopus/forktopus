// «Показать в папке» и «Открыть в терминале» для каталога (D-066): карточка
// репозитория, заголовок в проводнике, окно проблемных каталогов.

import FolderOpenIcon from '@mui/icons-material/FolderOpenOutlined'
import TerminalIcon from '@mui/icons-material/Terminal'
import { Button } from '@mui/material'
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
