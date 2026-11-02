package providers

import (
	"net/url"
	"strings"

	"github.com/forktopus/forktopus/internal/model"
)

// listURL строится из веб-адреса логина, а не SSH host/port. Счётчику нужен
// список даже при нуле; URL конкретного PR или запуска остаётся отдельным.
func listURL(q Query, ci bool) string {
	u, err := url.Parse(q.Target.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	u.User = nil
	parts := strings.Split(strings.Trim(q.Target.Project, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return ""
		}
		parts[i] = url.PathEscape(part)
	}
	base := strings.TrimRight(u.String(), "/") + "/" + strings.Join(parts, "/")
	switch q.Target.Provider {
	case model.ProviderGitHub:
		if ci {
			if q.Branch != "" {
				return base + "/actions?query=" + url.QueryEscape("branch:"+q.Branch)
			}
			return base + "/actions"
		}
		filter := "is:pr is:open"
		if q.Branch != "" {
			filter += " head:" + q.Branch
		} else if q.SHA != "" {
			filter += " " + q.SHA
		}
		return base + "/pulls?q=" + url.QueryEscape(filter)
	case model.ProviderGitLab:
		if ci {
			if q.Branch != "" {
				return base + "/-/pipelines?ref=" + url.QueryEscape(q.Branch)
			}
			if q.SHA != "" {
				return base + "/-/commit/" + url.PathEscape(q.SHA) + "/pipelines"
			}
			return base + "/-/pipelines"
		}
		link := base + "/-/merge_requests?state=opened"
		if q.Branch != "" {
			link += "&source_branch=" + url.QueryEscape(q.Branch)
		}
		return link
	case model.ProviderGitea:
		// В Gitea 1.26 страница Actions не фильтрует по ветке. Открываем сам список,
		// не добавляя неподдерживаемые параметры; PR также ведёт к открытым проекта.
		if ci {
			return base + "/actions"
		}
		return base + "/pulls?state=open"
	}
	return ""
}

func prListLink(q Query, s *model.PRSummary) {
	if s.Availability == model.AvailabilityOK {
		s.ListURL = listURL(q, false)
	}
}
func ciListLink(q Query, s *model.CISummary) {
	if s.Availability == model.AvailabilityOK {
		s.ListURL = listURL(q, true)
	}
}
