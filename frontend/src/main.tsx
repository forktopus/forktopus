import React from 'react'
import ReactDOM from 'react-dom/client'
import { App } from './app/App'
import { pickSource } from './data'
import { createAppStore } from './store/appStore'
import { AppStoreProvider } from './store/context'

const store = createAppStore(await pickSource())
void store.getState().init()

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <AppStoreProvider store={store}>
      <App />
    </AppStoreProvider>
  </React.StrictMode>,
)
