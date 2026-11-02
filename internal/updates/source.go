// Package updates проверяет публичные выпуски и скачивает проверенные пакеты.
// Сервис не устанавливает пакеты и не использует авторизацию Git-провайдеров.
package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const repository = "forktopus/forktopus"
const releaseBase = "https://github.com/" + repository + "/releases/"
const maxPackageSize int64 = 2 << 30

// ErrNoPackage отличает отсутствие поддерживаемой поставки от сетевого сбоя.
var ErrNoPackage = errors.New("в новом выпуске нет пакета выбранного формата")

var stable = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Release — неизменяемый снимок одного выпуска, включая версионный URL и SHA-256.
type Release struct {
	Version, Filename, URL, Page string
	Size                         int64
	SHA256                       [sha256.Size]byte
}

// Source отделяет сетевой контракт от состояния, файлов и Wails.
type Source interface {
	Check(context.Context, string, string) (*Release, error)
	Open(context.Context, Release) (io.ReadCloser, error)
}

// AssetName задаёт точное имя из scripts/release; setup.exe и exe различаются.
func AssetName(version, format string) string {
	switch format {
	case "deb":
		return "forktopus_" + version + "_amd64.deb"
	case "appimage":
		return "Forktopus-" + version + "-x86_64.AppImage"
	case "nsis":
		return "forktopus-" + version + "-windows-amd64-setup.exe"
	case "exe":
		return "forktopus-" + version + "-windows-amd64.exe"
	case "macos":
		return "Forktopus-" + version + "-macos-universal.zip"
	}
	return ""
}

// newer сравнивает числовые части без переполнения, включая 0.10.0 > 0.9.0.
func newer(a, b string) bool {
	aa, bb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range aa {
		if len(aa[i]) != len(bb[i]) {
			return len(aa[i]) > len(bb[i])
		}
		if aa[i] != bb[i] {
			return aa[i] > bb[i]
		}
	}
	return false
}

// GitHubSource использует существующие Releases и SHA256SUMS. Встроенный
// provider Wails не ограничивает JSON и трактует 404 как current, поэтому
// строгий контракт реализован здесь без повторного разбора его ответов.
type GitHubSource struct {
	client   *http.Client
	endpoint string
	allowed  func(*url.URL) bool
}

// NewGitHubSource не читает токены и сохраняет системную поддержку HTTP proxy.
func NewGitHubSource() *GitHubSource {
	allowed := func(u *url.URL) bool {
		if u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
			return false
		}
		switch u.Host {
		case "api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
			return true
		}
		return false
	}
	s := &GitHubSource{endpoint: "https://api.github.com/repos/" + repository + "/releases/latest", allowed: allowed}
	s.client = &http.Client{Timeout: 30 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !allowed(r.URL) {
			return errors.New("недопустимое перенаправление GitHub")
		}
		return nil
	}}
	return s
}

func (s *GitHubSource) get(ctx context.Context, address string) (io.ReadCloser, error) {
	u, err := url.Parse(address)
	if err != nil || !s.allowed(u) {
		return nil, errors.New("недопустимый адрес выпуска")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, errors.New("не удалось создать запрос выпуска")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Forktopus-updates")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := s.client.Do(req)
	// Не возвращаем url.Error: подписанный адрес CDN может содержать секреты.
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("GitHub недоступен: проверьте сеть и настройки прокси")
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		switch resp.StatusCode {
		case 403, 429:
			return nil, errors.New("GitHub ограничил запросы или запретил доступ. Повторите проверку позже")
		case 404:
			return nil, errors.New("выпуск или его файл не найден на GitHub")
		default:
			return nil, fmt.Errorf("GitHub ответил HTTP %d. Повторите попытку позже", resp.StatusCode)
		}
	}
	return resp.Body, nil
}

func (s *GitHubSource) read(ctx context.Context, address string, limit int64) ([]byte, error) {
	body, err := s.get(ctx, address)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	b, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, errors.New("не удалось прочитать метаданные выпуска")
	}
	if int64(len(b)) > limit {
		return nil, errors.New("метаданные выпуска превышают допустимый размер")
	}
	return b, nil
}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Check фиксирует метаданные, пакет и сумму из одного stable-релиза.
func (s *GitHubSource) Check(ctx context.Context, current, format string) (*Release, error) {
	if !stable.MatchString(current) {
		return nil, errors.New("не удалось определить стабильную версию приложения")
	}
	b, err := s.read(ctx, s.endpoint, 2<<20)
	if err != nil {
		return nil, err
	}
	var meta struct {
		Tag        string         `json:"tag_name"`
		Draft      bool           `json:"draft"`
		Prerelease bool           `json:"prerelease"`
		Assets     []releaseAsset `json:"assets"`
	}
	if json.Unmarshal(b, &meta) != nil {
		return nil, errors.New("GitHub вернул некорректные метаданные выпуска")
	}
	version := strings.TrimPrefix(meta.Tag, "v")
	if meta.Draft || meta.Prerelease || meta.Tag != "v"+version || !stable.MatchString(version) {
		return nil, errors.New("GitHub не вернул опубликованный стабильный выпуск")
	}
	if !newer(version, current) {
		return nil, nil
	}
	name := AssetName(version, format)
	if name == "" {
		return nil, errors.New("выберите поддерживаемый формат пакета")
	}
	var picked, sums *releaseAsset
	for i := range meta.Assets {
		a := &meta.Assets[i]
		if a.Name == name {
			if picked != nil {
				return nil, errors.New("в выпуске повторяется имя пакета")
			}
			picked = a
		}
		if a.Name == "SHA256SUMS" {
			if sums != nil {
				return nil, errors.New("в выпуске повторяется SHA256SUMS")
			}
			sums = a
		}
	}
	if picked == nil {
		return nil, ErrNoPackage
	}
	if picked.Size <= 0 || picked.Size > maxPackageSize {
		return nil, errors.New("в выпуске указан недопустимый размер пакета")
	}
	if sums == nil {
		return nil, errors.New("в выпуске нет обязательного файла SHA256SUMS")
	}
	base := releaseBase + "download/" + meta.Tag + "/"
	if picked.URL != base+name || sums.URL != base+"SHA256SUMS" {
		return nil, errors.New("адрес пакета или SHA256SUMS не относится к выбранному выпуску")
	}
	b, err = s.read(ctx, sums.URL, 64<<10)
	if err != nil {
		return nil, err
	}
	sum, err := ParseChecksum(string(b), name)
	if err != nil {
		return nil, err
	}
	return &Release{Version: version, Filename: name, URL: picked.URL, Page: releaseBase + "tag/" + meta.Tag, Size: picked.Size, SHA256: sum}, nil
}

// ParseChecksum требует единственную точную запись sha256sum, не принимает
// неполный digest и не подменяет полное имя совпадением по basename.
func ParseChecksum(body, name string) ([sha256.Size]byte, error) {
	var out [sha256.Size]byte
	found := false
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		sum, err := hex.DecodeString(fields[0])
		if found || err != nil || len(sum) != sha256.Size {
			return out, errors.New("SHA256SUMS содержит некорректную или повторную сумму пакета")
		}
		copy(out[:], sum)
		found = true
	}
	if !found {
		return out, errors.New("в SHA256SUMS нет суммы выбранного пакета")
	}
	return out, nil
}

// Open использует зафиксированный URL; latest при скачивании не перечитывается.
func (s *GitHubSource) Open(ctx context.Context, r Release) (io.ReadCloser, error) {
	return s.get(ctx, r.URL)
}
