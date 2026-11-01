package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// repository — публичный репозиторий релизов; origin обязан указывать на него.
const repository = "forktopus/forktopus"

// Release, Asset и Run — поля ответов GitHub API, которые нужны выпуску.
type Release struct {
	ID         int64  `json:"id"`
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

type Asset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	State              string `json:"state"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Run struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	RunAttempt int    `json:"run_attempt"`
}

// GitHub — операции выпуска на GitHub. В тестах подменяется.
type GitHub interface {
	Releases() ([]Release, error)
	Assets(releaseID int64) ([]Asset, error)
	Runs(revision string) ([]Run, error)
	// Download скачивает все артефакты запуска в dir: по каталогу на артефакт.
	Download(runID int64, dir string) error
	Create(tag, title, notesFile string) error
	Upload(tag, file string) error
	Publish(tag string) error
	DeleteAsset(assetID int64) error
	Latest() (Release, error)
}

// ghClient ходит в GitHub через gh: вход и токен — забота gh, скрипт их не читает.
type ghClient struct {
	run func(name string, args ...string) (string, error)
}

func (g ghClient) api(endpoint string) ([]json.RawMessage, error) {
	out, err := g.run("gh", "api", endpoint, "--paginate", "--slurp")
	if err != nil {
		return nil, err
	}
	var pages []json.RawMessage
	if err := json.Unmarshal([]byte(out), &pages); err != nil {
		return nil, fmt.Errorf("ответ gh api %s: %w", endpoint, err)
	}
	return pages, nil
}

func pagesOf[T any](g ghClient, endpoint string) ([]T, error) {
	pages, err := g.api(endpoint)
	if err != nil {
		return nil, err
	}
	var all []T
	for _, p := range pages {
		var items []T
		if err := json.Unmarshal(p, &items); err != nil {
			return nil, fmt.Errorf("ответ gh api %s: %w", endpoint, err)
		}
		all = append(all, items...)
	}
	return all, nil
}

func (g ghClient) Releases() ([]Release, error) {
	return pagesOf[Release](g, "repos/"+repository+"/releases?per_page=100")
}

func (g ghClient) Assets(releaseID int64) ([]Asset, error) {
	return pagesOf[Asset](g, fmt.Sprintf("repos/%s/releases/%d/assets?per_page=100", repository, releaseID))
}

func (g ghClient) Runs(revision string) ([]Run, error) {
	pages, err := g.api("repos/" + repository + "/actions/runs?head_sha=" + revision + "&per_page=100")
	if err != nil {
		return nil, err
	}
	var all []Run
	for _, p := range pages {
		var page struct {
			WorkflowRuns []Run `json:"workflow_runs"`
		}
		if err := json.Unmarshal(p, &page); err != nil {
			return nil, fmt.Errorf("ответ gh api actions/runs: %w", err)
		}
		all = append(all, page.WorkflowRuns...)
	}
	return all, nil
}

func (g ghClient) Download(runID int64, dir string) error {
	_, err := g.run("gh", "run", "download", strconv.FormatInt(runID, 10), "--repo", repository, "--dir", dir)
	return err
}

func (g ghClient) Create(tag, title, notesFile string) error {
	_, err := g.run("gh", "release", "create", tag, "--repo", repository, "--verify-tag", "--draft", "--title", title, "--notes-file", notesFile)
	return err
}

func (g ghClient) Upload(tag, file string) error {
	_, err := g.run("gh", "release", "upload", tag, file, "--repo", repository)
	return err
}

func (g ghClient) Publish(tag string) error {
	_, err := g.run("gh", "release", "edit", tag, "--repo", repository, "--draft=false", "--latest")
	return err
}

func (g ghClient) DeleteAsset(assetID int64) error {
	_, err := g.run("gh", "api", "--method", "DELETE", fmt.Sprintf("repos/%s/releases/assets/%d", repository, assetID))
	return err
}

func (g ghClient) Latest() (Release, error) {
	out, err := g.run("gh", "api", "repos/"+repository+"/releases/latest")
	if err != nil {
		return Release{}, err
	}
	var r Release
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return Release{}, fmt.Errorf("ответ gh api releases/latest: %w", err)
	}
	return r, nil
}

// ciRun — последний запуск workflow CI на push этого коммита (только он
// собирает файлы выпуска) и его состояние: success, failure, pending или absent.
func ciRun(runs []Run) (Run, string) {
	var push []Run
	for _, r := range runs {
		if r.Name == "CI" && r.Event == "push" {
			push = append(push, r)
		}
	}
	if len(push) == 0 {
		return Run{}, "absent"
	}
	sort.Slice(push, func(i, j int) bool {
		if push[i].ID != push[j].ID {
			return push[i].ID > push[j].ID
		}
		return push[i].RunAttempt > push[j].RunAttempt
	})
	latest := push[0]
	switch {
	case latest.Status != "completed":
		return latest, "pending"
	case latest.Conclusion == "success":
		return latest, "success"
	}
	return latest, "failure"
}
