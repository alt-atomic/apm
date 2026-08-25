package apt

import (
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
)

// TestNoOperationClassification проверяет полный путь: строка APT → MatchedError → тип apmerr.
func TestNoOperationClassification(t *testing.T) {
	matched := ErrorLinesAnalise([]string{"E: Packages are already installed: tmux"})
	if matched == nil {
		t.Fatal("already-installed error was not matched")
	}
	if !matched.IsNoOperation() {
		t.Fatalf("IsNoOperation() = false for code %d", matched.Entry.Code)
	}
	if matched.IsNotFound() {
		t.Error("already-installed must not be classified as not-found")
	}
	if got := apmerr.New(apmerr.ErrorTypeApt, matched).Type; got != apmerr.ErrorTypeNoOperation {
		t.Errorf("apmerr type = %s, want %s", got, apmerr.ErrorTypeNoOperation)
	}
}

// TestRegularAptErrorStaysApt страхует от чрезмерного повышения типа.
func TestRegularAptErrorStaysApt(t *testing.T) {
	matched := ErrorLinesAnalise([]string{"E: Broken packages"})
	if matched == nil {
		t.Fatal("broken-packages error was not matched")
	}
	if matched.IsNoOperation() {
		t.Error("broken packages must not be a no-op")
	}
	if got := apmerr.New(apmerr.ErrorTypeApt, matched).Type; got != apmerr.ErrorTypeApt {
		t.Errorf("apmerr type = %s, want %s", got, apmerr.ErrorTypeApt)
	}
}
