import { Typography } from '@mui/material'

export function EmptyText({ text }: { text: string }) {
  return <Typography sx={{ color: 'text.secondary', textAlign: 'center', py: 4, fontSize: 13 }}>{text}</Typography>
}
