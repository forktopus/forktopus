//go:build !unix && !windows

package runner

import "os/exec"

func detach(*exec.Cmd) {}
