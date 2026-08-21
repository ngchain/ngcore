package ngstate

import "testing"

// TestContractExports pins the export listing: zero-arg funcs are callable,
// the reserved init entry and functions with params are not
func TestContractExports(t *testing.T) {
	if ex, err := ContractExports(nil); err != nil || ex != nil {
		t.Fatalf("empty source = %v (%v), want nil", ex, err)
	}

	src := mustWat(`
(module
  (memory 1)
  (func (export "main") nop)
  (func (export "init") nop)
  (func (export "add") (param i32)))
`)
	exports, err := ContractExports(src)
	if err != nil {
		t.Fatal(err)
	}

	byName := map[string]ContractExport{}
	for _, e := range exports {
		byName[e.Name] = e
	}
	if len(byName) != 3 {
		t.Fatalf("exports = %+v, want 3", exports)
	}
	if !byName["main"].Callable {
		t.Error("main (0-arg) must be callable")
	}
	if byName["init"].Callable {
		t.Error("init is reserved, must not be callable")
	}
	if byName["add"].Callable || byName["add"].Params != 1 {
		t.Errorf("add = %+v, want 1 param, not callable", byName["add"])
	}
}
