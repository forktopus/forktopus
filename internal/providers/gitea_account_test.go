package providers

import (
	"context"
	"testing"

	"github.com/forktopus/forktopus/internal/model"
)

// Один HTTP 403 имеет разные причины: проверяем готовность CLI и данные CI,
// чтобы блокировка аккаунта не стала отсутствием поддержки или достоверным нулём.
func TestGiteaAccountDiagnostics(t *testing.T) {
	const blocked = "This account is prohibited from signing in, please contact your site administrator."
	const inactive = "This account is not activated."
	cases := []struct {
		name         string
		target       Target
		api          reply
		code         model.ErrorCode
		availability model.Availability
	}{
		{"gt: запрет входа текстом", gtTarget, stderr(1, "Error: permission denied by the Gitea instance: "+blocked), model.CodeAccountBlocked, model.AvailabilityUnavailable},
		{"gt: запрет входа с кодом 4", gtTarget, stderr(4, "Error: "+blocked), model.CodeAccountBlocked, model.AvailabilityUnavailable},
		{"gt: неактивный аккаунт текстом", gtTarget, stderr(1, "Error: permission denied by the Gitea instance: "+inactive), model.CodeAccountBlocked, model.AvailabilityUnavailable},
		{"gt: запрет входа JSON", gtTarget, gtHTTP(403, "GET", "user", `{"message":"`+blocked+`"}`), model.CodeAccountBlocked, model.AvailabilityUnavailable},
		{"gt: неактивный аккаунт JSON", gtTarget, gtHTTP(403, "GET", "user", `{"message":"`+inactive+`"}`), model.CodeAccountBlocked, model.AvailabilityUnavailable},
		{"tea: запрет входа JSON при коде 0", teaTarget, apiReply("HTTP/2.0 403 Forbidden", `{"message":"`+blocked+`"}`), model.CodeAccountBlocked, model.AvailabilityUnavailable},
		{"tea: неактивный аккаунт JSON", teaTarget, apiReply("HTTP/2.0 403 Forbidden", `{"message":"`+inactive+`"}`), model.CodeAccountBlocked, model.AvailabilityUnavailable},
		{"tea: запрет входа текстом", teaTarget, stderr(1, "Error: "+blocked), model.CodeAccountBlocked, model.AvailabilityUnavailable},
		{"gt: обычный отказ прав текстом", gtTarget, stderr(1, "Error: permission denied by the Gitea instance: user should have specific read permission"), model.CodeAccessDenied, model.AvailabilityNotSupported},
		{"gt: обычный отказ прав JSON", gtTarget, gtHTTP(403, "GET", "user", `{"message":"forbidden"}`), model.CodeAccessDenied, model.AvailabilityNotSupported},
		{"tea: обычный отказ прав", teaTarget, apiReply("HTTP/2.0 403 Forbidden", `{"message":"forbidden"}`), model.CodeAccessDenied, model.AvailabilityNotSupported},
		{"gt: нет маршрута API", gtTarget, gtHTTP(404, "GET", "user", "404 page not found"), model.CodeUnsupported, model.AvailabilityNotSupported},
		{"tea: нет маршрута API", teaTarget, apiReply("HTTP/2.0 404 Not Found", "404 page not found"), model.CodeUnsupported, model.AvailabilityNotSupported},
		{"gt: истёкший токен", gtTarget, gtHTTP(401, "GET", "user", `{"message":"invalid username, password or token"}`), model.CodeAuthRequired, model.AvailabilityUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, _ := gtWith(map[string]reply{"--version": gtOK("version 0.2.0"), "api": c.api})
			d := g.Diagnose(context.Background(), "/repo", c.target)
			if d.Availability != c.availability || d.Error == nil || d.Error.Code != c.code {
				t.Fatalf("диагностика: %+v, ошибка: %+v", d, d.Error)
			}
			q := Query{Dir: "/repo", Target: c.target, Branch: "main"}
			ci := g.CIRuns(context.Background(), q)
			if ci.Availability != c.availability || ci.Error == nil || ci.Error.Code != c.code || ci.Active != nil {
				t.Errorf("CI: %+v, ошибка: %+v", ci, ci.Error)
			}
			if c.target.Tool == "gt" {
				pr := g.PullRequests(context.Background(), q)
				if pr.Availability != c.availability || pr.Error == nil || pr.Error.Code != c.code || pr.Open != nil {
					t.Errorf("PR: %+v, ошибка: %+v", pr, pr.Error)
				}
			}
		})
	}
}
