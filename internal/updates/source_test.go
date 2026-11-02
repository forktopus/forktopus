package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Только транспорт fixture переписывает origin. Проверки настоящих URL
// GitHub и всех переходов выполняются тем же production Source.
func fixtureSource(t *testing.T, handler http.HandlerFunc) *GitHubSource {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	u, _ := url.Parse(server.URL)
	s := NewGitHubSource()
	transport := server.Client().Transport
	s.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		address := *r.URL
		address.Scheme = u.Scheme
		address.Host = u.Host
		clone.URL = &address
		return transport.RoundTrip(clone)
	})
	return s
}

func releaseJSON(version string, data []byte) map[string]any {
	name := AssetName(version, "deb")
	base := releaseBase + "download/v" + version + "/"
	return map[string]any{"tag_name": "v" + version, "draft": false, "prerelease": false, "assets": []releaseAsset{{Name: name, URL: base + name, Size: int64(len(data))}, {Name: "SHA256SUMS", URL: base + "SHA256SUMS", Size: 120}}}
}

func TestSourceContract(t *testing.T) {
	data := []byte("проверочный пакет")
	sum := sha256.Sum256(data)
	name := AssetName("0.10.0", "deb")
	for _, scenario := range []string{"success", "equal", "older", "404", "403", "429", "broken-json", "draft", "prerelease", "bad-tag", "missing-package", "missing-sums", "bad-size", "wrong-origin", "mixed-version", "bad-sum", "duplicate-sum", "short-sum", "missing-entry", "oversize-json", "oversize-sums", "redirect", "bad-redirect"} {
		t.Run(scenario, func(t *testing.T) {
			meta := releaseJSON("0.10.0", data)
			assets := meta["assets"].([]releaseAsset)
			switch scenario {
			case "draft":
				meta["draft"] = true
			case "prerelease":
				meta["prerelease"] = true
			case "bad-tag":
				meta["tag_name"] = "v0.10.0-rc.1"
			case "missing-package":
				meta["assets"] = assets[1:]
			case "missing-sums":
				meta["assets"] = assets[:1]
			case "bad-size":
				assets[0].Size = 0
			case "wrong-origin":
				assets[0].URL = "https://example.org/package"
			case "mixed-version":
				assets[0].URL = releaseBase + "download/v0.11.0/" + name
			}
			s := fixtureSource(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/latest") {
					switch scenario {
					case "404":
						w.WriteHeader(404)
						return
					case "403":
						w.WriteHeader(403)
						return
					case "429":
						w.WriteHeader(429)
						return
					case "broken-json":
						fmt.Fprint(w, "{")
						return
					case "oversize-json":
						fmt.Fprint(w, strings.Repeat(" ", (2<<20)+1))
						return
					}
					json.NewEncoder(w).Encode(meta)
					return
				}
				switch scenario {
				case "bad-redirect":
					http.Redirect(w, r, "https://example.org/private?secret=hidden", 302)
					return
				case "redirect":
					if r.URL.Path != "/cdn" {
						http.Redirect(w, r, "https://release-assets.githubusercontent.com/cdn", 302)
						return
					}
				case "oversize-sums":
					fmt.Fprint(w, strings.Repeat(" ", (64<<10)+1))
					return
				case "bad-sum":
					fmt.Fprintln(w, "xyz  "+name)
					return
				case "short-sum":
					fmt.Fprintln(w, "00  "+name)
					return
				case "missing-entry":
					fmt.Fprintln(w, hex.EncodeToString(sum[:])+"  wrong.deb")
					return
				case "duplicate-sum":
					fmt.Fprintf(w, "%x  %s\n", sum, name)
				}
				fmt.Fprintf(w, "%x  %s\n", sum, name)
			})
			current := "0.9.0"
			if scenario == "equal" {
				current = "0.10.0"
			}
			if scenario == "older" {
				current = "0.11.0"
			}
			rel, err := s.Check(context.Background(), current, "deb")
			switch scenario {
			case "success", "redirect":
				if err != nil || rel == nil || rel.Filename != name || rel.SHA256 != sum || rel.Page != releaseBase+"tag/v0.10.0" {
					t.Fatalf("%+v %v", rel, err)
				}
			case "equal", "older":
				if err != nil || rel != nil {
					t.Fatalf("%v %v", rel, err)
				}
			default:
				if err == nil {
					t.Fatalf("ожидался отказ: %+v", rel)
				}
				if strings.Contains(err.Error(), "secret=") {
					t.Fatal("ошибка раскрыла URL CDN")
				}
			}
		})
	}
}

func TestSourceCancelAndDownload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NewGitHubSource()
	if _, err := s.Check(ctx, "0.3.1", "deb"); err == nil {
		t.Fatal("отмена проигнорирована")
	}
	s = fixtureSource(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "пакет") })
	body, err := s.Open(context.Background(), Release{URL: releaseBase + "download/v0.4.0/file"})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil || string(b) != "пакет" {
		t.Fatalf("%s %v", b, err)
	}
	for _, address := range []string{"http://github.com/file", "https://user:pass@github.com/file", "https://github.com:444/file", "https://example.org/file"} {
		if _, err = s.Open(context.Background(), Release{URL: address}); err == nil {
			t.Fatalf("принят %s", address)
		}
	}
}

func TestVersionsAndNames(t *testing.T) {
	if !newer("0.10.0", "0.9.0") || newer("0.9.0", "0.10.0") || newer("0.10.0", "0.10.0") {
		t.Fatal("неверное сравнение версий")
	}
	if AssetName("0.4.0", "nsis") == AssetName("0.4.0", "exe") || AssetName("0.4.0", "unknown") != "" {
		t.Fatal("перепутаны форматы")
	}
}

// Полный путь Check → HTTP stream → SHA256 → файл использует настоящий
// клиент Source и временный TLS-сервер, не fake downloader.
func TestHTTPDownloadIntegration(t *testing.T) {
	data := []byte(strings.Repeat("пакет обновления", 8192))
	sum := sha256.Sum256(data)
	version := "0.10.0"
	name := AssetName(version, "deb")
	s := fixtureSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			json.NewEncoder(w).Encode(releaseJSON(version, data))
		case strings.HasSuffix(r.URL.Path, "/SHA256SUMS"):
			fmt.Fprintf(w, "%x  %s\n", sum, name)
		default:
			w.Write(data)
		}
	})
	service := New(context.Background(), Options{Version: "0.9.0", Format: "deb", Directory: t.TempDir(), Source: s})
	defer service.Close()
	service.Check()
	waitStatus(t, service, "available")
	service.Download()
	waitStatus(t, service, "ready")
	dir, err := service.ReadyFolder(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil || sha256.Sum256(b) != sum {
		t.Fatalf("неверный файл: %v", err)
	}
}

func TestHTTPInterruptedDownloadAndRetry(t *testing.T) {
	data := []byte(strings.Repeat("x", 128<<10))
	sum := sha256.Sum256(data)
	name := AssetName("0.4.0", "deb")
	var interrupted atomic.Bool
	interrupted.Store(true)
	s := fixtureSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			json.NewEncoder(w).Encode(releaseJSON("0.4.0", data))
		case strings.HasSuffix(r.URL.Path, "/SHA256SUMS"):
			fmt.Fprintf(w, "%x  %s\n", sum, name)
		default:
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			if interrupted.Load() {
				w.Write(data[:1024])
				return
			}
			w.Write(data)
		}
	})
	service := New(context.Background(), Options{Version: "0.3.1", Format: "deb", Directory: t.TempDir(), Source: s})
	defer service.Close()
	service.Check()
	waitStatus(t, service, "available")
	service.Download()
	st := waitStatus(t, service, "error")
	if !st.CanDownload {
		t.Fatal("после обрыва недоступен повтор")
	}
	noParts(t, service.dir)
	interrupted.Store(false)
	service.Download()
	waitStatus(t, service, "ready")
}
