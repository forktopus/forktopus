package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNilAndZeroDifferInJSON(t *testing.T) {
	zero := 0
	unknown, err := json.Marshal(BranchStatus{Name: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(unknown), "ahead") {
		t.Errorf("нет upstream, но ahead в JSON: %s", unknown)
	}

	known, err := json.Marshal(BranchStatus{Name: "main", Ahead: &zero, Behind: &zero})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(known), `"ahead":0`) || !strings.Contains(string(known), `"behind":0`) {
		t.Errorf("достоверный ноль потерян: %s", known)
	}

	pr, err := json.Marshal(PRSummary{Availability: AvailabilityUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(pr), "open") {
		t.Errorf("недоступный источник отдаёт число PR: %s", pr)
	}
}

func TestErrorJSONAndCode(t *testing.T) {
	err := fmt.Errorf("обёртка: %w", Errorf(CodeCLIMissing, "%s не найден", "gh"))
	if got := CodeOf(err); got != CodeCLIMissing {
		t.Fatalf("CodeOf = %q", got)
	}
	if got := CodeOf(errors.New("x")); got != CodeOther {
		t.Fatalf("CodeOf чужой ошибки = %q", got)
	}
	if CodeOf(nil) != "" {
		t.Fatal("CodeOf(nil) не пуст")
	}
	var none *Error
	if CodeOf(none) != "" {
		t.Fatal("CodeOf((*Error)(nil)) не пуст")
	}

	var e error = AsError(err)
	data, jsonErr := json.Marshal(&e) // так сериализует ошибки Wails
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	want := `{"code":"CLI_MISSING","message":"gh не найден"}`
	if string(data) != want {
		t.Errorf("JSON = %s, ожидалось %s", data, want)
	}
}

func TestPathID(t *testing.T) {
	if PathID("/a/b/") != PathID("/a/b") {
		t.Error("PathID зависит от завершающего слэша")
	}
	if PathID("/a/b") == PathID("/a/c") {
		t.Error("разные пути дали одинаковый ID")
	}
	if len(PathID("/a")) != 16 {
		t.Errorf("длина ID = %d", len(PathID("/a")))
	}
}
