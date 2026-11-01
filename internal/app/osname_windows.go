//go:build windows

package app

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/windows"
)

// osName — версия Windows и архитектура: «Windows 11 10.0.26100 (windows/amd64)».
// ProductName из реестра на Windows 11 остаётся «Windows 10», поэтому
// поколение определяется по номеру сборки.
func osName() string {
	v := windows.RtlGetVersion()
	name := "Windows"
	switch {
	case v.MajorVersion == 10 && v.BuildNumber >= 22000:
		name = "Windows 11"
	case v.MajorVersion == 10:
		name = "Windows 10"
	}
	return fmt.Sprintf("%s %d.%d.%d (%s/%s)", name, v.MajorVersion, v.MinorVersion, v.BuildNumber, runtime.GOOS, runtime.GOARCH)
}
