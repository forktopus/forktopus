import { Events } from '@wailsio/runtime'
import { AppService } from '../../bindings/github.com/forktopus/forktopus/internal/app'
import { normalizeRepository, normalizeSnapshot, type DataSource } from './source'

export const wailsSource: DataSource = {
  kind: 'wails',
  listRoots: async () => (await AppService.ListRoots()) ?? [],
  addRoot: (path) => AppService.AddRoot(path),
  removeRoot: (id) => AppService.RemoveRoot(id),
  pickFolder: () => AppService.PickFolder(),
  listRepositories: async () => normalizeSnapshot(await AppService.ListRepositories()),
  scanRoots: async () => normalizeSnapshot(await AppService.ScanRoots()),
  selectRemote: (repoId, remote) => AppService.SelectRemote(repoId, remote),
  checkTools: async () => (await AppService.CheckTools()) ?? [],
  getUI: () => AppService.GetUI(),
  saveUI: (ui) => AppService.SaveUI(ui),
  onScanRepository: (cb) => Events.On('scan:repository', (event) => cb(normalizeRepository(event.data))),
}
