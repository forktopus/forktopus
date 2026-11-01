package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Error — отказ выпуска с понятной причиной; печатается как есть.
type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func fail(format string, args ...any) error { return &Error{fmt.Sprintf(format, args...)} }

func require(ok bool, format string, args ...any) error {
	if ok {
		return nil
	}
	return fail(format, args...)
}

const (
	releaseBase = "https://github.com/" + repository + "/releases/"
	// emptyTree — для diff первого выпуска: изменения от пустого дерева.
	emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
)

var stableVersion = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// versionFiles — файлы build/ с версией приложения (Wails хранит её в каждом
// манифесте). В MSIX она четырёхзначная (0.2.0.0), в Info.dev.plist — с
// суффиксом -dev: замена по границе слова покрывает оба случая.
var versionFiles = []string{
	"build/config.yml",
	"build/linux/nfpm/nfpm.yaml",
	"build/windows/info.json",
	"build/windows/wails.exe.manifest",
	"build/windows/nsis/wails_tools.nsh",
	"build/windows/msix/app_manifest.xml",
	"build/windows/msix/template.xml",
	"build/darwin/Info.plist",
	"build/darwin/Info.dev.plist",
	"build/ios/Info.plist",
	"build/ios/Info.dev.plist",
}

// artifacts — файлы выпуска по артефактам CI (forktopus-<ос>), имена — как у scripts/release.sh.
func artifacts(version string) map[string][]string {
	return map[string][]string{
		"forktopus-linux":   {"forktopus_" + version + "_amd64.deb", "Forktopus-" + version + "-x86_64.AppImage"},
		"forktopus-windows": {"forktopus-" + version + "-windows-amd64-setup.exe", "forktopus-" + version + "-windows-amd64.exe"},
		"forktopus-macos":   {"Forktopus-" + version + "-macos-universal.zip"},
	}
}

// packages — поставки выпуска по порядку; вложения — они и SHA256SUMS.
func packages(version string) []string {
	a := artifacts(version)
	return slices.Concat(a["forktopus-linux"], a["forktopus-windows"], a["forktopus-macos"])
}

func assetNames(version string) []string { return append(packages(version), "SHA256SUMS") }

func checkVersion(v string) error {
	return require(stableVersion.MatchString(v), "Укажите версию X.Y.Z без префикса v, например 0.3.0.")
}

func parts(v string) [3]uint64 {
	m := stableVersion.FindStringSubmatch(strings.TrimPrefix(v, "v"))
	var p [3]uint64
	for i := range p {
		p[i], _ = strconv.ParseUint(m[i+1], 10, 64)
	}
	return p
}

// compareVersions — численно: 0.10.0 новее 0.9.0.
func compareVersions(a, b string) int {
	pa, pb := parts(a), parts(b)
	for i := range pa {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func nextPatch(v string) string {
	p := parts(v)
	return fmt.Sprintf("%d.%d.%d", p[0], p[1], p[2]+1)
}

var configVersion = regexp.MustCompile(`(?m)^  version: "([0-9.]+)"`)

func digest(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Tool — выпуск в рабочей копии Dir. git — команда git в Dir, gh — GitHub.
type Tool struct {
	Dir string
	Git func(args ...string) (string, error)
	GH  GitHub
}

func (t *Tool) git(args ...string) (string, error) { return t.Git(args...) }

// AppVersion — версия приложения в build/config.yml.
func (t *Tool) AppVersion() (string, error) {
	b, err := os.ReadFile(filepath.Join(t.Dir, "build/config.yml"))
	if err != nil {
		return "", err
	}
	m := configVersion.FindSubmatch(b)
	if m == nil {
		return "", fail("Не нашёл версию в build/config.yml.")
	}
	return string(m[1]), nil
}

// Bump меняет версию во всех файлах build/ и коммитит «build: версия X».
func (t *Tool) Bump(version string) (map[string]any, error) {
	if err := checkVersion(version); err != nil {
		return nil, err
	}
	status, err := t.git("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if err := require(status == "", "Смена версии требует чистого дерева: её коммит не должен захватить чужие изменения."); err != nil {
		return nil, err
	}
	old, err := t.AppVersion()
	if err != nil {
		return nil, err
	}
	if err := require(old != version, "Версия в build/ уже %s.", version); err != nil {
		return nil, err
	}
	word := regexp.MustCompile(`\b` + regexp.QuoteMeta(old) + `\b`)
	changed := map[string][]byte{}
	for _, f := range versionFiles {
		b, err := os.ReadFile(filepath.Join(t.Dir, f))
		if err != nil {
			return nil, err
		}
		if err := require(word.Match(b), "В %s нет версии %s — список файлов с версией устарел.", f, old); err != nil {
			return nil, err
		}
		changed[f] = word.ReplaceAll(b, []byte(version))
	}
	for f, b := range changed {
		if err := os.WriteFile(filepath.Join(t.Dir, f), b, 0o644); err != nil {
			return nil, err
		}
	}
	if _, err := t.git(append([]string{"add", "--"}, versionFiles...)...); err != nil {
		return nil, err
	}
	if _, err := t.git("commit", "-q", "-m", "build: версия "+version); err != nil {
		return nil, err
	}
	rev, err := t.git("rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	return map[string]any{"previous": old, "version": version, "revision": rev, "next": "git push origin main — CI соберёт файлы этой версии"}, nil
}

// state — общее для всех команд: Git, опубликованные релизы, выбранная версия.
type state struct {
	revision, version, tag string
	clean                  bool
	refs                   map[string]string
	newest, previous       *Release
	previousRevision       string
	existing               *Release
	remoteTag              string
}

func (s *state) mainMatches() bool { return s.refs["refs/heads/main"] == s.revision }

func (t *Tool) state(version string) (*state, error) {
	if version != "" {
		if err := require(!strings.HasPrefix(version, "v"), "Передайте --version без префикса v."); err != nil {
			return nil, err
		}
		if err := checkVersion(version); err != nil {
			return nil, err
		}
	}
	// Буквальный адрес из настроек, без подстановок insteadOf.
	origin, err := t.git("config", "--get", "remote.origin.url")
	if err != nil {
		return nil, fail("Нет remote origin.")
	}
	ok := false
	for _, u := range []string{"git@github.com:" + repository, "https://github.com/" + repository, "ssh://git@github.com/" + repository} {
		ok = ok || origin == u || origin == u+".git"
	}
	if err := require(ok, "origin должен указывать на %s на GitHub.", repository); err != nil {
		return nil, err
	}
	s := &state{refs: map[string]string{}}
	if s.revision, err = t.git("rev-parse", "HEAD"); err != nil {
		return nil, err
	}
	status, err := t.git("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	s.clean = status == ""
	remote, err := t.git("ls-remote", "--heads", "--tags", "origin")
	if err != nil {
		return nil, err
	}
	for line := range strings.Lines(remote) {
		if f := strings.Fields(line); len(f) == 2 {
			s.refs[f[1]] = f[0]
		}
	}
	tagRevision := func(tag string) string {
		if r := s.refs["refs/tags/"+tag+"^{}"]; r != "" {
			return r
		}
		return s.refs["refs/tags/"+tag]
	}
	releases, err := t.GH.Releases()
	if err != nil {
		return nil, err
	}
	// Предыдущий выпуск — по опубликованным релизам, а не по локальным тегам.
	var published []Release
	for _, r := range releases {
		if !r.Draft && !r.Prerelease && strings.HasPrefix(r.TagName, "v") && stableVersion.MatchString(r.TagName[1:]) {
			published = append(published, r)
		}
	}
	slices.SortFunc(published, func(a, b Release) int { return compareVersions(b.TagName, a.TagName) })
	if len(published) > 0 {
		s.newest = &published[0]
	}
	switch {
	case version != "":
		s.version = version
	case s.newest != nil:
		s.version = nextPatch(s.newest.TagName)
	default:
		if s.version, err = t.AppVersion(); err != nil {
			return nil, err
		}
	}
	s.tag = "v" + s.version
	for i := range published {
		if published[i].TagName != s.tag && compareVersions(published[i].TagName, s.tag) < 0 {
			s.previous = &published[i]
			break
		}
	}
	if s.previous != nil {
		s.previousRevision = tagRevision(s.previous.TagName)
		if err := require(s.previousRevision != "", "У предыдущего опубликованного релиза %s нет тега на GitHub.", s.previous.TagName); err != nil {
			return nil, err
		}
		if _, err := t.git("merge-base", "--is-ancestor", s.previousRevision, s.revision); err != nil {
			return nil, fail("Предыдущий релиз не является предком HEAD или его коммита нет локально. Получите историю и теги: git fetch --tags origin.")
		}
	}
	for i := range releases {
		if releases[i].TagName == s.tag {
			s.existing = &releases[i]
		}
	}
	s.remoteTag = tagRevision(s.tag)
	return s, nil
}

func (t *Tool) releaseDir(tag string) string { return filepath.Join(t.Dir, "bin", "release", tag) }

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type commit struct {
	Revision string `json:"revision"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
}

// Plan только читает Git и GitHub: версия, предыдущий выпуск, коммиты и состояние.
func (t *Tool) Plan(version string) (map[string]any, error) {
	s, err := t.state(version)
	if err != nil {
		return nil, err
	}
	rng := s.revision
	if s.previousRevision != "" {
		rng = s.previousRevision + ".." + s.revision
	}
	log, err := t.git("log", "--reverse", "--format=%H%x00%s%x00%b%x00", rng)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(log, "\x00")
	commits := []commit{}
	for i := 0; i+2 < len(fields); i += 3 {
		commits = append(commits, commit{strings.TrimSpace(fields[i]), fields[i+1], strings.TrimSpace(fields[i+2])})
	}
	base := s.previousRevision
	if base == "" {
		base = emptyTree
	}
	changes, err := t.git("diff", "--stat", base, s.revision)
	if err != nil {
		return nil, err
	}
	runs, err := t.GH.Runs(s.revision)
	if err != nil {
		return nil, err
	}
	run, ci := ciRun(runs)
	app, err := t.AppVersion()
	if err != nil {
		return nil, err
	}
	releaseState := "absent"
	if s.existing != nil {
		releaseState = "published"
		if s.existing.Draft {
			releaseState = "draft"
		}
	}
	var previousTag any
	if s.previous != nil {
		previousTag = s.previous.TagName
	}
	return map[string]any{
		"version":           s.version,
		"tag":               s.tag,
		"revision":          s.revision,
		"appVersion":        app,
		"versionCommitted":  app == s.version,
		"previousTag":       previousTag,
		"previousRevision":  s.previousRevision,
		"firstRelease":      s.previous == nil,
		"workingTreeClean":  s.clean,
		"remoteMainMatches": s.mainMatches(),
		"ci":                ci,
		"ciRun":             run.ID,
		"releaseState":      releaseState,
		"remoteTagExists":   s.remoteTag != "",
		"prepared":          exists(filepath.Join(t.releaseDir(s.tag), "release.json")),
		"commits":           commits,
		"changes":           changes,
	}, nil
}

type assetInfo struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type metadata struct {
	Schema      int         `json:"schema"`
	Tag         string      `json:"tag"`
	Version     string      `json:"version"`
	Revision    string      `json:"revision"`
	PreviousTag string      `json:"previousTag,omitempty"`
	CIRun       int64       `json:"ciRun"`
	NotesSHA256 string      `json:"notesSha256"`
	Assets      []assetInfo `json:"assets"`
}

// sumsFile — SHA256SUMS в формате shasum -a 256.
func sumsFile(names []string, sums map[string]string) string {
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "%s  %s\n", sums[n], n)
	}
	return b.String()
}

// parseSums — строки «хеш  имя» (у shasum в бинарном режиме — «хеш *имя»).
func parseSums(text string) map[string]string {
	out := map[string]string{}
	for line := range strings.Lines(text) {
		hash, name, ok := strings.Cut(strings.TrimRight(line, "\r\n"), " ")
		if ok {
			out[strings.TrimPrefix(strings.TrimPrefix(name, " "), "*")] = hash
		}
	}
	return out
}

// checkedAssets — размеры и хеши файлов комплекта; SHA256SUMS обязан совпадать с поставками.
func checkedAssets(version, dir string) ([]assetInfo, error) {
	var assets []assetInfo
	sums := map[string]string{}
	for _, name := range assetNames(version) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || len(b) == 0 {
			return nil, fail("Нет файла выпуска или он пуст: %s.", name)
		}
		sums[name] = digest(b)
		assets = append(assets, assetInfo{name, int64(len(b)), sums[name]})
	}
	listed, _ := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err := require(string(listed) == sumsFile(packages(version), sums), "SHA256SUMS не совпадает с файлами комплекта."); err != nil {
		return nil, err
	}
	return assets, nil
}

func (t *Tool) assertLocalTag(tag, revision string) error {
	local, err := t.git("rev-parse", "refs/tags/"+tag+"^{commit}")
	return require(err == nil && local == revision, "Локальный тег %s не указывает на коммит подготовленного выпуска.", tag)
}

type prepared struct {
	dir    string
	meta   metadata
	notes  string
	assets []assetInfo
}

func (t *Tool) readPrepared(tag, version string) (*prepared, error) {
	dir := t.releaseDir(tag)
	b, err := os.ReadFile(filepath.Join(dir, "release.json"))
	if err != nil {
		return nil, fail("Выпуск %s не подготовлен: сначала выполните release prepare.", tag)
	}
	var m metadata
	if err := json.Unmarshal(b, &m); err != nil || m.Schema != 1 || m.Tag != tag || m.Version != version {
		return nil, fail("Подготовленный комплект относится к другой версии.")
	}
	notes, err := os.ReadFile(filepath.Join(dir, "notes.md"))
	if err != nil || digest(notes) != m.NotesSHA256 {
		return nil, fail("Описание изменилось после подготовки релиза.")
	}
	assets, err := checkedAssets(version, dir)
	if err != nil {
		return nil, err
	}
	if err := require(slices.Equal(assets, m.Assets), "Файлы комплекта изменились после подготовки релиза."); err != nil {
		return nil, err
	}
	return &prepared{dir, m, string(notes), assets}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Prepare ставит локальный тег и собирает комплект из артефактов успешного CI
// этого коммита в bin/release/vX/. На GitHub ничего не отправляет.
func (t *Tool) Prepare(version, notesFile string) (map[string]any, error) {
	if err := require(version != "" && notesFile != "", "prepare требует --version и --notes."); err != nil {
		return nil, err
	}
	s, err := t.state(version)
	if err != nil {
		return nil, err
	}
	for _, check := range []error{
		require(s.clean, "Подготовка требует чистого дерева. Зафиксируйте изменения отдельным действием."),
		require(s.existing == nil || s.existing.Draft, "Эта версия уже опубликована; для изменений выберите новую."),
		require(s.newest == nil || compareVersions(s.tag, s.newest.TagName) > 0, "Новая версия должна быть выше последнего опубликованного релиза %s.", tagOf(s.newest)),
		require(s.remoteTag == "" || s.remoteTag == s.revision, "Тег %s на GitHub указывает на другой коммит.", s.tag),
	} {
		if check != nil {
			return nil, check
		}
	}
	app, err := t.AppVersion()
	if err != nil {
		return nil, err
	}
	if err := require(app == version, "В build/ версия %s, а не %s: сначала release bump --version %s, push и зелёный CI.", app, version, version); err != nil {
		return nil, err
	}
	notes, err := os.ReadFile(notesFile)
	if err != nil {
		return nil, fail("Не прочитал описание %s.", notesFile)
	}
	if err := require(strings.TrimSpace(string(notes)) != "", "Описание релиза не должно быть пустым."); err != nil {
		return nil, err
	}
	hasLocalTag := false
	if out, _ := t.git("tag", "--list", s.tag); out != "" {
		hasLocalTag = true
		if err := t.assertLocalTag(s.tag, s.revision); err != nil {
			return nil, err
		}
	}
	dir := t.releaseDir(s.tag)
	rel, _ := filepath.Rel(t.Dir, dir)
	if exists(filepath.Join(dir, "release.json")) {
		p, err := t.readPrepared(s.tag, version)
		if err != nil {
			return nil, err
		}
		if err := require(p.meta.Revision == s.revision, "Комплект этой версии подготовлен из другого коммита."); err != nil {
			return nil, err
		}
		if err := require(p.notes == string(notes), "Релиз уже подготовлен с другим описанием."); err != nil {
			return nil, err
		}
		return map[string]any{"tag": s.tag, "revision": s.revision, "directory": rel, "prepared": true, "reused": true}, nil
	}
	// Готовые байты одной версии не заменяются другими молча: незавершённый каталог разбирают вручную.
	if err := require(!exists(dir), "Каталог %s остался от незавершённой подготовки. Удалите его перед повтором prepare.", rel); err != nil {
		return nil, err
	}
	// Файлы собирает CI на push в main: локальная сборка их не заменяет.
	runs, err := t.GH.Runs(s.revision)
	if err != nil {
		return nil, err
	}
	run, ci := ciRun(runs)
	hint := map[string]string{
		"absent":  " Отправьте коммит в main: git push origin main. Если в коммите только документация, CI не запускался: gh workflow run CI --ref main.",
		"pending": " Дождитесь окончания: gh run watch " + strconv.FormatInt(run.ID, 10) + ".",
		"failure": " Отменённый или упавший запуск перезапускают: gh run rerun " + strconv.FormatInt(run.ID, 10) + ".",
	}[ci]
	if err := require(ci == "success", "CI коммита %s: %s. Файлы выпуска берутся из его успешного запуска на push или вручную.%s", short(s.revision), ci, hint); err != nil {
		return nil, err
	}
	download := filepath.Join(t.Dir, "bin", "release", "."+s.tag+".download")
	if err := os.RemoveAll(download); err != nil {
		return nil, err
	}
	defer os.RemoveAll(download)
	if err := t.GH.Download(run.ID, download); err != nil {
		return nil, fail("Не удалось скачать артефакты запуска %d: %v.", run.ID, err)
	}
	// У каждого артефакта свой SHA256SUMS от scripts/release.sh: файлы сверяются с ним.
	// Файлы лежат в каталоге версии: upload-artifact сохраняет его из bin/release/<версия>/.
	for name, files := range artifacts(version) {
		listed, err := os.ReadFile(filepath.Join(download, name, version, "SHA256SUMS"))
		if err != nil {
			return nil, fail("В артефакте %s запуска %d нет SHA256SUMS.", name, run.ID)
		}
		sums := parseSums(string(listed))
		for _, f := range files {
			b, err := os.ReadFile(filepath.Join(download, name, version, f))
			if err != nil {
				return nil, fail("В артефакте %s запуска %d нет %s.", name, run.ID, f)
			}
			if err := require(sums[f] == digest(b), "%s не совпадает с SHA256SUMS артефакта %s.", f, name); err != nil {
				return nil, err
			}
		}
	}
	if !hasLocalTag {
		if _, err := t.git("tag", "-a", s.tag, "-m", "Forktopus "+version, s.revision); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	sums := map[string]string{}
	for name, files := range artifacts(version) {
		for _, f := range files {
			if err := copyFile(filepath.Join(download, name, version, f), filepath.Join(dir, f)); err != nil {
				return nil, err
			}
			b, _ := os.ReadFile(filepath.Join(dir, f))
			sums[f] = digest(b)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sumsFile(packages(version), sums)), 0o644); err != nil {
		return nil, err
	}
	assets, err := checkedAssets(version, dir)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), notes, 0o644); err != nil {
		return nil, err
	}
	m := metadata{Schema: 1, Tag: s.tag, Version: version, Revision: s.revision, PreviousTag: tagOf(s.previous), CIRun: run.ID, NotesSHA256: digest(notes), Assets: assets}
	if err := writeJSON(filepath.Join(dir, "release.json"), m); err != nil {
		return nil, err
	}
	return map[string]any{"tag": s.tag, "revision": s.revision, "ciRun": run.ID, "directory": rel, "prepared": true, "reused": false}, nil
}

func tagOf(r *Release) string {
	if r == nil {
		return ""
	}
	return r.TagName
}

func short(rev string) string { return rev[:min(12, len(rev))] }

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

type verifiedAsset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// verifyRemote — GitHub сам считает digest загруженного файла: сверка без скачивания.
// У черновика адрес временный (untagged-…), версионный проверяется после публикации.
func verifyRemote(remote *Asset, a assetInfo, tag string) (verifiedAsset, error) {
	if remote == nil || remote.State != "uploaded" || remote.Size != a.Size {
		return verifiedAsset{}, fail("Размер или состояние вложения %s на GitHub не совпадает.", a.Name)
	}
	if remote.Digest != "sha256:"+a.SHA256 {
		return verifiedAsset{}, fail("SHA-256 вложения %s на GitHub отсутствует или не совпадает с локальным файлом.", a.Name)
	}
	u := releaseBase + "download/" + tag + "/" + url.PathEscape(a.Name)
	if tag != "" && remote.BrowserDownloadURL != u {
		return verifiedAsset{}, fail("Адрес вложения %s не совпадает с версионным.", a.Name)
	}
	return verifiedAsset{a.Name, u, remote.Size, a.SHA256}, nil
}

func findAsset(list []Asset, name string) *Asset {
	for i := range list {
		if list[i].Name == name {
			return &list[i]
		}
	}
	return nil
}

// Publish отправляет тег, создаёт черновик, загружает недостающие файлы, сверяет
// их через API и публикует релиз как latest. Повтор после сбоя догружает недостающее.
func (t *Tool) Publish(version string) (map[string]any, error) {
	if err := require(version != "", "publish требует --version."); err != nil {
		return nil, err
	}
	s, err := t.state(version)
	if err != nil {
		return nil, err
	}
	for _, check := range []error{
		require(s.clean, "Публикация требует чистого рабочего дерева."),
		require(s.mainMatches(), "Перед публикацией отправьте текущий коммит в main на GitHub."),
		require(s.newest == nil || compareVersions(s.tag, s.newest.TagName) >= 0, "На GitHub уже есть более новый релиз %s; latest не будет понижен.", tagOf(s.newest)),
	} {
		if check != nil {
			return nil, check
		}
	}
	p, err := t.readPrepared(s.tag, version)
	if err != nil {
		return nil, err
	}
	revision := p.meta.Revision
	if err := t.assertLocalTag(s.tag, revision); err != nil {
		return nil, err
	}
	// HEAD может быть новее исходного коммита, но тот обязан быть в main.
	if _, err := t.git("merge-base", "--is-ancestor", revision, s.revision); err != nil {
		return nil, fail("Исходный коммит комплекта отсутствует в main на GitHub.")
	}
	if err := require(s.remoteTag == "" || s.remoteTag == revision, "Тег на GitHub не соответствует подготовленному комплекту."); err != nil {
		return nil, err
	}
	if s.remoteTag == "" {
		if _, err := t.git("push", "origin", "refs/tags/"+s.tag); err != nil {
			return nil, err
		}
	}
	title := "Forktopus " + version
	if s.existing == nil {
		if err := t.GH.Create(s.tag, title, filepath.Join(p.dir, "notes.md")); err != nil {
			return nil, err
		}
		if s, err = t.state(version); err != nil {
			return nil, err
		}
	}
	release := s.existing
	if err := require(release != nil && !release.Prerelease && release.Name == title && strings.TrimRight(release.Body, "\n") == strings.TrimRight(p.notes, "\n"),
		"Описание, имя или канал существующего релиза отличаются от подготовленного комплекта."); err != nil {
		return nil, err
	}
	names := assetNames(version)
	remote, err := t.GH.Assets(release.ID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, a := range remote {
		if err := require(!seen[a.Name] && slices.Contains(names, a.Name), "В релизе есть лишние или повторяющиеся вложения."); err != nil {
			return nil, err
		}
		seen[a.Name] = true
	}
	for _, a := range p.assets {
		r := findAsset(remote, a.Name)
		// После обрыва загрузки GitHub оставляет пустой starter: в черновике удаляется только этот след.
		if release.Draft && r != nil && r.State == "starter" && r.Size == 0 {
			if err := t.GH.DeleteAsset(r.ID); err != nil {
				return nil, err
			}
			r = nil
		}
		if r == nil {
			if err := require(release.Draft, "В опубликованном релизе не хватает вложений; опубликованные файлы не изменяются."); err != nil {
				return nil, err
			}
			if err := t.GH.Upload(s.tag, filepath.Join(p.dir, a.Name)); err != nil {
				return nil, err
			}
			fresh, err := t.GH.Assets(release.ID)
			if err != nil {
				return nil, err
			}
			r = findAsset(fresh, a.Name)
		}
		if _, err := verifyRemote(r, a, ""); err != nil {
			return nil, err
		}
	}
	fresh, err := t.state(version)
	if err != nil {
		return nil, err
	}
	if err := require(fresh.clean && fresh.revision == s.revision && fresh.remoteTag == revision && fresh.mainMatches(),
		"Git-состояние изменилось во время загрузки; публикация остановлена."); err != nil {
		return nil, err
	}
	if err := require(fresh.newest == nil || compareVersions(s.tag, fresh.newest.TagName) >= 0, "Во время загрузки опубликован более новый релиз."); err != nil {
		return nil, err
	}
	if release.Draft {
		if err := t.GH.Publish(s.tag); err != nil {
			return nil, err
		}
	}
	latest, err := t.GH.Latest()
	if err != nil {
		return nil, err
	}
	if err := require(latest.TagName == s.tag && !latest.Draft && !latest.Prerelease, "Релиз не стал latest; проверьте состояние GitHub."); err != nil {
		return nil, err
	}
	// После публикации список читается заново: проверка черновика не заменяет проверку опубликованного.
	published, err := t.GH.Assets(latest.ID)
	if err != nil {
		return nil, err
	}
	if err := require(len(published) == len(p.assets), "Опубликованный релиз должен содержать ровно %d ожидаемых вложений.", len(p.assets)); err != nil {
		return nil, err
	}
	var verified []verifiedAsset
	for _, a := range p.assets {
		v, err := verifyRemote(findAsset(published, a.Name), a, s.tag)
		if err != nil {
			return nil, err
		}
		verified = append(verified, v)
	}
	receipt := map[string]any{"schema": 1, "tag": s.tag, "revision": revision, "verification": "github-api-sha256", "assets": verified}
	if err := writeJSON(filepath.Join(p.dir, "public-release.json"), receipt); err != nil {
		return nil, err
	}
	return map[string]any{"tag": s.tag, "revision": revision, "url": releaseBase + "tag/" + s.tag, "published": true, "verification": "github-api-sha256", "assets": len(verified)}, nil
}

// isReleaseError — отказ выпуска, а не сбой команды.
func isReleaseError(err error) bool {
	var e *Error
	return errors.As(err, &e)
}
