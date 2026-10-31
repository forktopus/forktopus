import { createContext, useContext, type ReactNode } from 'react'
import { useStore } from 'zustand'
import type { AppState, AppStore } from './appStore'

const StoreContext = createContext<AppStore | null>(null)

export function AppStoreProvider({ store, children }: { store: AppStore; children: ReactNode }) {
  return <StoreContext.Provider value={store}>{children}</StoreContext.Provider>
}

/** Подписка на часть store. Селектор должен возвращать стабильное значение. */
export function useApp<T>(selector: (s: AppState) => T): T {
  const store = useContext(StoreContext)
  if (!store) throw new Error('AppStoreProvider не найден')
  return useStore(store, selector)
}
