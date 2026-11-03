// Подготовка AppArmor выполняется до загрузки GTK/WebKit: иначе отказ
// user namespaces завершает процесс внутри C и не оставляет понятной ошибки.
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

const setupFlag = "--setup-apparmor"
const restart = 20

// Профиль относится к самому AppImage, а не к случайной точке FUSE.
// После exec разрешение наследуют AppRun, приложение и песочница WebKit.
func profile(image string) (string, string) {
	sum := sha256.Sum256([]byte(image))
	name := fmt.Sprintf("forktopus-appimage-%x", sum[:])
	var path strings.Builder
	for _, c := range []byte(image) {
		if c == '/' || c == '.' || c == '-' || c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			path.WriteByte(c)
		} else {
			// Hex сохраняет точный путь: пробелы, AARE и @{переменные}
			// не должны превращаться в шаблон либо новые правила профиля.
			fmt.Fprintf(&path, `\x%02x`, c)
		}
	}
	body := fmt.Sprintf(`# Разрешение user namespaces для одного файла Forktopus AppImage.
# Общая политика AppArmor и песочница WebKit остаются включёнными.
abi <abi/4.0>,
profile %s "%s" flags=(unconfined) {
  userns,
}
`, name, path.String())
	return name, body
}

// Скрипт использует только системные программы: root не может читать helper
// из пользовательского FUSE mount. Аргументы передаются отдельно от кода sh.
const installScript = `set -eu
name=$1
case "$name" in forktopus-appimage-*) ;; *) exit 1 ;; esac
suffix=${name#forktopus-appimage-}
case "$suffix" in ''|*[!0-9a-f]*) exit 1 ;; esac
[ "${#suffix}" -eq 64 ]
[ "$(id -u)" -eq 0 ]
parser=/sbin/apparmor_parser
[ -x "$parser" ] || parser=/usr/sbin/apparmor_parser
umask 022
tmp=$(mktemp /etc/apparmor.d/.forktopus-appimage.XXXXXX)
trap 'rm -f "$tmp"' EXIT HUP INT TERM
printf '%s' "$2" > "$tmp"
chmod 644 "$tmp"
"$parser" -r "$tmp"
mv -f "$tmp" "/etc/apparmor.d/$name"
`

func imagePath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("не найден путь APPIMAGE; запустите сам файл AppImage")
	}
	path, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	header := make([]byte, 11)
	if _, err = f.ReadAt(header, 0); err != nil || string(header[:4]) != "\x7fELF" || string(header[8:11]) != "AI\x02" {
		return "", fmt.Errorf("файл не является AppImage типа 2: %s", path)
	}
	return path, nil
}

func probe() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--probe-userns")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:                 syscall.CLONE_NEWUSER,
		UidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
		GidMappingsEnableSetgroups: false,
	}
	return cmd.Run()
}

func restricted() bool {
	data, err := os.ReadFile("/proc/sys/kernel/apparmor_restrict_unprivileged_userns")
	return err == nil && strings.TrimSpace(string(data)) == "1"
}

func notify(text string) {
	fmt.Fprintln(os.Stderr, text)
	if bin, err := exec.LookPath("zenity"); err == nil {
		_ = exec.Command(bin, "--error", "--title=Forktopus", "--no-markup", "--text="+text).Run()
	}
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--probe-userns" || args[0] == "--version") {
		return 0
	}
	return prepare(args, restricted, probe)
}

func prepare(args []string, isRestricted func() bool, checkUserns func() error) int {
	explicit := len(args) == 1 && args[0] == setupFlag
	if !explicit && (!isRestricted() || checkUserns() == nil) {
		return 0
	}
	if !explicit && os.Getenv("FORKTOPUS_APPIMAGE_SETUP_RETRY") == "1" {
		notify("После настройки AppArmor user namespaces всё ещё недоступны. Запуск отменён; проверьте ограничения системы.")
		return 1
	}
	image, err := imagePath(os.Getenv("APPIMAGE"))
	if err != nil {
		notify("Не удалось настроить запуск Forktopus: " + err.Error())
		return 1
	}
	message := "Ubuntu запрещает песочнице WebKit создавать user namespaces. Для запуска Forktopus нужен профиль AppArmor только для этого файла AppImage. Система попросит пароль администратора. Настроить запуск?"
	if !explicit {
		// Автозапуск не должен открывать запрос пароля при входе в систему.
		if slices.Contains(args, "--hidden") {
			fmt.Fprintln(os.Stderr, "Forktopus: требуется настройка AppArmor; запустите AppImage вручную.")
			return 1
		}
		bin, err := exec.LookPath("zenity")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Forktopus: требуется профиль AppArmor. Выполните: '%s' %s\n", strings.ReplaceAll(image, "'", "'\\''"), setupFlag)
			return 1
		}
		if exec.Command(bin, "--question", "--title=Запуск Forktopus", "--no-markup", "--text="+message, "--ok-label=Настроить", "--cancel-label=Отмена").Run() != nil {
			return 1
		}
	}
	name, body := profile(image)
	cmd := exec.Command("pkexec", "/bin/sh", "-c", installScript, "forktopus-apparmor", name, body)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		notify("Настройка AppArmor не выполнена. Запуск отменён; повторите запуск AppImage для новой попытки.")
		return 1
	}
	if explicit {
		fmt.Println("Профиль AppArmor настроен. Запустите Forktopus обычным способом.")
		return 0
	}
	return restart
}

func main() { os.Exit(run(os.Args[1:])) }
