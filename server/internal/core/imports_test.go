package core_test

import (
	"os/exec"
	"strings"
	"testing"
)

// Jádro nesmí importovat žádný produkt – jinak by produkty nešly později
// oddělit a mezi balíčky by snadno vznikl cyklus.
func TestCoreDoesNotImportProducts(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "mediater/internal/core/...").CombinedOutput()
	if err != nil {
		t.Skipf("go list nejde spustit: %v\n%s", err, out)
	}
	for _, pkg := range strings.Fields(string(out)) {
		if strings.HasPrefix(pkg, "mediater/internal/libriter") || strings.HasPrefix(pkg, "mediater/internal/sceniter") {
			t.Errorf("jádro závisí na produktu: %s", pkg)
		}
	}
}
