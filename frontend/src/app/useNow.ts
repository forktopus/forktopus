import { useEffect, useState } from 'react'

/** Текущее время, обновляемое с заданным периодом: для «N мин. назад». */
export function useNow(periodMs = 30_000): Date {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), periodMs)
    return () => clearInterval(id)
  }, [periodMs])
  return now
}
