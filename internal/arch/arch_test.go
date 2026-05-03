package arch_test

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const module = "github.com/localpull/orders"

func TestDomainDoesNotImportAdapters(t *testing.T) {
	assertNoTransitiveImport(t, module+"/internal/order", "adapters")
}

func TestAdaptersDoNotCrossImport(t *testing.T) {
	assertNoTransitiveImport(t, module+"/internal/adapters/postgres", "adapters/valkey")
	assertNoTransitiveImport(t, module+"/internal/adapters/valkey", "adapters/postgres")
	assertNoTransitiveImport(t, module+"/internal/adapters/projection", "adapters/postgres")
	assertNoTransitiveImport(t, module+"/internal/adapters/projection", "adapters/valkey")
}

func assertNoTransitiveImport(t *testing.T, pkg, forbidden string) {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedImports}
	pkgs, err := packages.Load(cfg, pkg)
	if err != nil {
		t.Fatalf("load %s: %v", pkg, err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("no packages found for %s", pkg)
	}
	for imp := range pkgs[0].Imports {
		if strings.Contains(imp, forbidden) {
			t.Errorf("%s must not import %q (found %s)", pkg, forbidden, imp)
		}
	}
}
