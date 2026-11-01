package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/forktopus/forktopus/internal/gittest"
)

// fakeGH — GitHub в памяти: релизы, вложения с digest, запуски CI и их артефакты.
type fakeGH struct {
	releases  []Release
	assets    map[int64][]Asset
	runs      []Run
	artifacts map[string]map[string][]byte // артефакт → путь «версия/файл» → байты, как их выкладывает CI
	failOnce  string                       // загрузка этого файла обрывается, оставляя starter
	corrupt   string                       // у этого файла GitHub «насчитает» другой digest
	nextID    int64
	uploads   int
}

func newFakeGH() *fakeGH { return &fakeGH{assets: map[int64][]Asset{}, nextID: 100} }

func (g *fakeGH) id() int64 { g.nextID++; return g.nextID }

func (g *fakeGH) byTag(tag string) *Release {
	for i := range g.releases {
		if g.releases[i].TagName == tag {
			return &g.releases[i]
		}
	}
	return nil
}

func (g *fakeGH) Releases() ([]Release, error)     { return slices.Clone(g.releases), nil }
func (g *fakeGH) Assets(id int64) ([]Asset, error) { return slices.Clone(g.assets[id]), nil }
func (g *fakeGH) Runs(string) ([]Run, error)       { return slices.Clone(g.runs), nil }
func (g *fakeGH) DeleteAsset(id int64) error {
	for rid, list := range g.assets {
		g.assets[rid] = slices.DeleteFunc(list, func(a Asset) bool { return a.ID == id })
	}
	return nil
}

func (g *fakeGH) Download(runID int64, dir string) error {
	for name, files := range g.artifacts {
		sums := map[string]*strings.Builder{} // каталог → его SHA256SUMS
		for f, b := range files {
			path := filepath.Join(dir, name, f)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, b, 0o644); err != nil {
				return err
			}
			if sums[filepath.Dir(path)] == nil {
				sums[filepath.Dir(path)] = &strings.Builder{}
			}
			fmt.Fprintf(sums[filepath.Dir(path)], "%s  %s\n", digest(b), filepath.Base(f))
		}
		for d, s := range sums {
			if err := os.WriteFile(filepath.Join(d, "SHA256SUMS"), []byte(s.String()), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *fakeGH) Create(tag, title, notesFile string) error {
	body, err := os.ReadFile(notesFile)
	if err != nil {
		return err
	}
	g.releases = append(g.releases, Release{ID: g.id(), TagName: tag, Name: title, Body: string(body), Draft: true})
	return nil
}

func (g *fakeGH) url(r *Release, name string) string {
	if r.Draft {
		return releaseBase + "download/untagged-123/" + name
	}
	return releaseBase + "download/" + r.TagName + "/" + name
}

func (g *fakeGH) Upload(tag, file string) error {
	r := g.byTag(tag)
	name := filepath.Base(file)
	if g.failOnce == name {
		g.failOnce = ""
		g.assets[r.ID] = append(g.assets[r.ID], Asset{ID: g.id(), Name: name, State: "starter"})
		return errors.New("обрыв загрузки")
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	sum := digest(b)
	if g.corrupt == name {
		sum = digest(append(b, '!'))
	}
	g.uploads++
	g.assets[r.ID] = append(g.assets[r.ID], Asset{ID: g.id(), Name: name, Size: int64(len(b)), State: "uploaded", Digest: "sha256:" + sum, BrowserDownloadURL: g.url(r, name)})
	return nil
}

func (g *fakeGH) Publish(tag string) error {
	r := g.byTag(tag)
	r.Draft = false
	for i, a := range g.assets[r.ID] {
		g.assets[r.ID][i].BrowserDownloadURL = g.url(r, a.Name)
	}
	return nil
}

func (g *fakeGH) Latest() (Release, error) {
	var best *Release
	for i := range g.releases {
		if r := &g.releases[i]; !r.Draft && (best == nil || compareVersions(r.TagName, best.TagName) > 0) {
			best = r
		}
	}
	if best == nil {
		return Release{}, errors.New("нет релизов")
	}
	return *best, nil
}

// setArtifacts — файлы, которые «собрал» CI для версии.
func (g *fakeGH) setArtifacts(version string) {
	g.artifacts = map[string]map[string][]byte{}
	for name, files := range artifacts(version) {
		g.artifacts[name] = map[string][]byte{}
		for _, f := range files {
			g.artifacts[name][version+"/"+f] = []byte("содержимое " + f)
		}
	}
}

const githubURL = "git@github.com:" + repository + ".git"

// newRepo — рабочая копия с origin на GitHub (через insteadOf — локальный bare) и версией 0.1.0 в build/.
func newRepo(t *testing.T) (*Tool, *fakeGH, string) {
	t.Helper()
	for _, kv := range []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Forktopus Test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=Forktopus Test", "GIT_COMMITTER_EMAIL=test@example.invalid"} {
		k, v, _ := strings.Cut(kv, "=")
		t.Setenv(k, v)
	}
	base := t.TempDir()
	bare := filepath.Join(base, "origin.git")
	work := filepath.Join(base, "work")
	gittest.Git(t, base, "init", "-q", "--bare", "-b", "main", bare)
	gittest.Git(t, base, "init", "-q", "-b", "main", work)
	gittest.Git(t, work, "remote", "add", "origin", githubURL)
	gittest.Git(t, work, "config", "url."+bare+".insteadOf", githubURL)
	for _, f := range versionFiles {
		content := "<version>0.1.0</version>\n"
		if f == "build/config.yml" {
			content = "info:\n  version: \"0.1.0\" # версия\n"
		}
		gittest.WriteFile(t, work, f, content)
	}
	gittest.WriteFile(t, work, ".gitignore", "bin\n")
	gittest.Git(t, work, "add", ".")
	gittest.Git(t, work, "commit", "-q", "-m", "начало")
	gittest.Git(t, work, "push", "-q", "origin", "main")
	gh := newFakeGH()
	tool := &Tool{Dir: work, GH: gh, Git: func(args ...string) (string, error) {
		out, err := gittest.TryGit(work, args...)
		return strings.TrimSpace(out), err
	}}
	return tool, gh, work
}

func notes(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func wantRefusal(t *testing.T, err error, part string) {
	t.Helper()
	if !isReleaseError(err) || !strings.Contains(err.Error(), part) {
		t.Fatalf("ждали отказ «%s», получили %v", part, err)
	}
}

// Полный путь: план, смена версии, подготовка из артефактов CI, публикация с обрывом загрузки, повторная проверка.
func TestReleaseFlow(t *testing.T) {
	tool, gh, work := newRepo(t)

	plan, err := tool.Plan("")
	if err != nil {
		t.Fatal(err)
	}
	if plan["version"] != "0.1.0" || plan["firstRelease"] != true || plan["ci"] != "absent" || plan["versionCommitted"] != true {
		t.Fatalf("план первого выпуска: %v", plan)
	}

	if _, err := tool.Bump("0.2.0"); err != nil {
		t.Fatal(err)
	}
	if v, _ := tool.AppVersion(); v != "0.2.0" {
		t.Fatalf("версия после bump: %s", v)
	}
	for _, f := range versionFiles {
		if b, _ := os.ReadFile(filepath.Join(work, f)); !strings.Contains(string(b), "0.2.0") || strings.Contains(string(b), "0.1.0") {
			t.Fatalf("%s: %s", f, b)
		}
	}
	if msg := gittest.Git(t, work, "log", "-1", "--format=%s"); msg != "build: версия 0.2.0" {
		t.Fatalf("коммит версии: %q", msg)
	}
	gittest.Git(t, work, "push", "-q", "origin", "main")

	n := notes(t, "Первый выпуск.\n")
	_, err = tool.Prepare("0.2.0", n)
	wantRefusal(t, err, "CI коммита")
	gh.runs = []Run{{ID: 7, Name: "CI", Event: "push", Status: "in_progress"}}
	_, err = tool.Prepare("0.2.0", n)
	wantRefusal(t, err, "pending")
	gh.runs = []Run{{ID: 7, Name: "CI", Event: "push", Status: "completed", Conclusion: "success"}}
	gh.setArtifacts("0.2.0")

	res, err := tool.Prepare("0.2.0", n)
	if err != nil || res["reused"] != false {
		t.Fatalf("prepare: %v %v", res, err)
	}
	dir := filepath.Join(work, "bin/release/v0.2.0")
	sums, _ := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if lines := strings.Count(string(sums), "\n"); lines != 4 {
		t.Fatalf("SHA256SUMS: %s", sums)
	}
	if res, err := tool.Prepare("0.2.0", n); err != nil || res["reused"] != true {
		t.Fatalf("повтор prepare: %v %v", res, err)
	}
	_, err = tool.Prepare("0.2.0", notes(t, "Другое описание.\n"))
	wantRefusal(t, err, "другим описанием")

	// Обрыв загрузки .exe: GitHub оставляет пустой starter, повтор его убирает и догружает.
	gh.failOnce = "forktopus-0.2.0-windows-amd64.exe"
	if _, err := tool.Publish("0.2.0"); err == nil {
		t.Fatal("ждали ошибку загрузки")
	}
	before := gh.uploads
	res, err = tool.Publish("0.2.0")
	if err != nil || res["published"] != true || res["assets"] != 5 {
		t.Fatalf("publish: %v %v", res, err)
	}
	// .deb и AppImage уже на месте: догружаются .exe, .zip и SHA256SUMS.
	if gh.uploads-before != 3 {
		t.Fatalf("повтор должен догрузить три файла, загружено %d", gh.uploads-before)
	}
	if r := gh.byTag("v0.2.0"); r.Draft || r.Name != "Forktopus 0.2.0" || r.Body != "Первый выпуск.\n" {
		t.Fatalf("релиз: %+v", r)
	}
	if !strings.Contains(gittest.Git(t, work, "ls-remote", "--tags", "origin"), "refs/tags/v0.2.0") {
		t.Fatal("тег не отправлен")
	}
	if _, err := os.Stat(filepath.Join(dir, "public-release.json")); err != nil {
		t.Fatal(err)
	}

	// Опубликованный релиз только проверяется заново.
	before = gh.uploads
	if _, err := tool.Publish("0.2.0"); err != nil || gh.uploads != before {
		t.Fatalf("повтор publish: %v, загрузок %d", err, gh.uploads-before)
	}

	plan, err = tool.Plan("")
	if err != nil {
		t.Fatal(err)
	}
	if plan["version"] != "0.2.1" || plan["previousTag"] != "v0.2.0" || plan["versionCommitted"] != false {
		t.Fatalf("план после выпуска: %v", plan)
	}
	_, err = tool.Prepare("0.2.0", n)
	wantRefusal(t, err, "уже опубликована")
}

func TestPrepareChecksArtifacts(t *testing.T) {
	tool, gh, _ := newRepo(t)
	gh.runs = []Run{{ID: 1, Name: "CI", Event: "push", Status: "completed", Conclusion: "success"}}
	gh.setArtifacts("0.1.0")
	delete(gh.artifacts["forktopus-linux"], "0.1.0/Forktopus-0.1.0-x86_64.AppImage")
	_, err := tool.Prepare("0.1.0", notes(t, "x\n"))
	wantRefusal(t, err, "нет Forktopus-0.1.0-x86_64.AppImage")

	_, err = tool.Prepare("0.2.0", notes(t, "x\n"))
	wantRefusal(t, err, "release bump --version 0.2.0")
}

func TestPublishChecksDigest(t *testing.T) {
	tool, gh, _ := newRepo(t)
	gh.runs = []Run{{ID: 1, Name: "CI", Event: "push", Status: "completed", Conclusion: "success"}}
	gh.setArtifacts("0.1.0")
	if _, err := tool.Prepare("0.1.0", notes(t, "x\n")); err != nil {
		t.Fatal(err)
	}
	gh.corrupt = "SHA256SUMS"
	_, err := tool.Publish("0.1.0")
	wantRefusal(t, err, "SHA-256 вложения SHA256SUMS")
	if r := gh.byTag("v0.1.0"); r == nil || !r.Draft {
		t.Fatalf("релиз должен остаться черновиком: %+v", r)
	}
}

func TestBumpChecksFiles(t *testing.T) {
	tool, _, work := newRepo(t)
	gittest.WriteFile(t, work, "build/darwin/Info.plist", "без версии\n")
	gittest.Git(t, work, "commit", "-qam", "сломать")
	_, err := tool.Bump("0.2.0")
	wantRefusal(t, err, "build/darwin/Info.plist")
	if v, _ := tool.AppVersion(); v != "0.1.0" {
		t.Fatalf("при отказе файлы не меняются: %s", v)
	}
	gittest.WriteFile(t, work, "README", "грязно\n")
	_, err = tool.Bump("0.2.0")
	wantRefusal(t, err, "чистого дерева")
}

func TestVersions(t *testing.T) {
	if compareVersions("0.10.0", "v0.9.0") != 1 || compareVersions("v1.0.0", "1.0.0") != 0 || nextPatch("v0.2.9") != "0.2.10" {
		t.Fatal("сравнение версий")
	}
	if err := checkVersion("v0.2.0"); err == nil {
		t.Fatal("версия с v")
	}
	sums := parseSums("aa  a.deb\r\nbb *b.exe\n")
	if sums["a.deb"] != "aa" || sums["b.exe"] != "bb" {
		t.Fatalf("parseSums: %v", sums)
	}
	run, state := ciRun([]Run{{ID: 1, Name: "CI", Event: "pull_request", Status: "completed", Conclusion: "success"}, {ID: 2, Name: "CI", Event: "push", Status: "completed", Conclusion: "cancelled"}})
	if run.ID != 2 || state != "failure" {
		t.Fatalf("ciRun: %v %s", run, state)
	}
	// После коммита только с документацией сборки запускают вручную.
	run, state = ciRun([]Run{{ID: 2, Name: "CI", Event: "push", Status: "completed", Conclusion: "cancelled"}, {ID: 3, Name: "CI", Event: "workflow_dispatch", Status: "completed", Conclusion: "success"}})
	if run.ID != 3 || state != "success" {
		t.Fatalf("ciRun вручную: %v %s", run, state)
	}
}
