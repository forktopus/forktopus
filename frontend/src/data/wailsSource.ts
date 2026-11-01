import { Browser, Clipboard, Events, Window } from '@wailsio/runtime'
import { AppService } from '../../bindings/github.com/forktopus/forktopus/internal/app'
import { normalizeOpResult, normalizeRepository, normalizeSkipDirs, normalizeSnapshot, type DataSource } from './source'

export const wailsSource: DataSource = {
  kind: 'wails',
  listRoots: async () => (await AppService.ListRoots()) ?? [],
  addRoot: (path) => AppService.AddRoot(path),
  removeRoot: (id) => AppService.RemoveRoot(id),
  getSkipDirs: async () => normalizeSkipDirs(await AppService.GetSkipDirs()),
  setSkipDirs: async (dirs) => normalizeSkipDirs(await AppService.SetSkipDirs(dirs)),
  pickFolder: () => AppService.PickFolder(),
  listRepositories: async () => normalizeSnapshot(await AppService.ListRepositories()),
  scanRoots: async () => normalizeSnapshot(await AppService.ScanRoots()),
  selectRemote: (repoId, remote) => AppService.SelectRemote(repoId, remote),
  getBranches: async (repoId) => (await AppService.GetBranches(repoId)) ?? [],
  refreshRepo: async (repoId) => normalizeRepository(await AppService.RefreshRepo(repoId)),
  fetchRepo: async (repoId, expect) => normalizeOpResult(await AppService.FetchRepo(repoId, expect)),
  pullRepo: async (repoId, expect) => normalizeOpResult(await AppService.FastForwardPull(repoId, expect)),
  switchBranch: async (repoId, branch, expect) => normalizeOpResult(await AppService.SwitchBranch(repoId, branch, expect)),
  getRemoteStatus: (repoId) => AppService.GetRemoteStatus(repoId),
  refreshRemote: (repoId) => AppService.RefreshRemote(repoId),
  listOperations: async () => (await AppService.ListOperations()) ?? [],
  checkTools: async () => (await AppService.CheckTools()) ?? [],
  getDiagnostics: async () => {
    const d = await AppService.GetDiagnostics()
    return { ...d, errors: d.errors ?? [] }
  },
  openURL: (url) => Browser.OpenURL(url),
  copyText: (text) => Clipboard.SetText(text),
  getUI: () => AppService.GetUI(),
  saveUI: (ui) => AppService.SaveUI(ui),
  // Нативный зум WebView: масштабируется всё, включая размеры в px.
  setZoom: (zoom) => Window.SetZoom(zoom),
  onScanRepository: (cb) => Events.On('scan:repository', (event) => cb(normalizeRepository(event.data))),
  onRemoteStatus: (cb) => Events.On('remote:status', (event) => cb(event.data)),
  onRepository: (cb) => Events.On('repo:updated', (event) => cb(normalizeRepository(event.data))),
  onChecked: (cb) => Events.On('background:checked', (event) => cb(event.data)),
}
