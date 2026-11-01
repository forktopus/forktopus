package desktop

import "slices"

// HiddenFlag — запуск без окна, сразу в трей (так стартует автозапуск).
const HiddenFlag = "--hidden"

// StartHidden — запуск с HiddenFlag и трей есть: без трея окно показывается,
// иначе до приложения было бы не добраться.
func StartHidden(args []string) bool {
	return slices.Contains(args, HiddenFlag) && trayAvailable()
}
