// Package buildinfo описывает версию и тип поставки запущенного приложения.
package buildinfo

import (
	buildconfig "github.com/forktopus/forktopus/build"
	"runtime"
)

// PackageFormat задаётся компоновщиком при упаковке: deb, appimage, nsis,
// exe или macos. Пустая метка не разрешает угадывать формат по окружению/пути.
var PackageFormat string

// Version берётся из встроенного build/config.yml, общего с упаковкой.
func Version() string { return buildconfig.Version() }

// Distribution использует встроенную метку. Перемещение файла и установка
// рядом другой поставки не меняют тип пакета для следующего обновления.
func Distribution() (format, reason string) {
	return distribution(runtime.GOOS, runtime.GOARCH, Production, PackageFormat)
}

func distribution(goos, arch string, production bool, format string) (string, string) {
	if !production {
		return "", "Сборка разработки: обновления проверяются в установленном приложении."
	}
	if arch != "amd64" && !(goos == "darwin" && arch == "arm64") {
		return "", "Для этой архитектуры пока нет пакетов Forktopus."
	}
	supported := false
	switch goos {
	case "linux":
		supported = format == "deb" || format == "appimage"
	case "windows":
		supported = format == "nsis" || format == "exe"
	case "darwin":
		supported = format == "macos"
	default:
		return "", "Для этой ОС пока нет пакетов Forktopus."
	}
	if !supported {
		return "", "Тип поставки не указан или не поддерживается этой сборкой. Проверка обновлений недоступна."
	}
	return format, ""
}
