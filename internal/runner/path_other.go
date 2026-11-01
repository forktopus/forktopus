//go:build !windows

package runner

// refreshPath: вне Windows PATH берётся из окружения процесса как есть.
func refreshPath() {}
