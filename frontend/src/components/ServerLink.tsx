import { Link } from '@mui/material'
import type { ReactNode } from 'react'
import { serverURL } from '../logic/serverLinks'
import { useApp } from '../store/context'

/** Ссылка открывается системным браузером. Клик по счётчику не должен также
 * открывать детали строки или Проводник у карточки Радара. */
export function ServerLink({ url, label, children }: { url?: string; label: string; children: ReactNode }) {
  const openURL = useApp((s) => s.openURL)
  const href = serverURL(url)
  if (!href) return <>{children}</>
  return (
    <Link
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      title={label}
      aria-label={label}
      color="inherit"
      underline="hover"
      sx={{
        font: 'inherit',
        verticalAlign: 'baseline',
        '&:focus-visible': { outline: '2px solid', outlineColor: 'primary.main', outlineOffset: 3, borderRadius: 0.5 },
      }}
      onClick={(e) => {
        e.preventDefault()
        e.stopPropagation()
        openURL(href)
      }}
    >
      {children}
    </Link>
  )
}
