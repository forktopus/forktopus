package runner

import "strings"

// mergePath — PATH из свежих записей и текущего значения с разделителем sep:
// порядок сохраняется, повторы (без учёта регистра и конечного слеша, как в
// Windows) и пустые записи убираются.
func mergePath(fresh []string, current, sep string) string {
	seen := map[string]bool{}
	var out []string
	for _, p := range append(fresh, strings.Split(current, sep)...) {
		key := strings.ToLower(strings.TrimRight(p, `\/`))
		if p == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return strings.Join(out, sep)
}
