package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShippedInventoryPasses(t *testing.T) {
	inv, err := Load(filepath.Join("..", "..", "inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if problems := Check(inv); len(problems) != 0 {
		t.Fatalf("shipped inventory violates the policy: %v", problems)
	}
}

func TestViolations(t *testing.T) {
	cases := []struct {
		name string
		inv  Inventory
		want string
	}{
		{
			name: "two watchers",
			inv: Inventory{
				Jobs:        map[string][]string{"watch": {"a", "b"}},
				Constraints: []Constraint{{Job: "watch", Exactly: 1}},
			},
			want: "want exactly 1",
		},
		{
			name: "missing survivor",
			inv: Inventory{
				Jobs:        map[string][]string{},
				Constraints: []Constraint{{Job: "store", Exactly: 1}},
			},
			want: "want exactly 1",
		},
		{
			name: "negative constraint",
			inv: Inventory{
				Jobs:        map[string][]string{"x": {}},
				Constraints: []Constraint{{Job: "x", Exactly: -1}},
			},
			want: "negative constraint",
		},
	}
	for _, tc := range cases {
		problems := Check(&tc.inv)
		if len(problems) == 0 || !strings.Contains(strings.Join(problems, "; "), tc.want) {
			t.Errorf("%s: problems = %v, want %q", tc.name, problems, tc.want)
		}
	}
	if got := Check(nil); len(got) != 1 {
		t.Errorf("nil inventory = %v", got)
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil {
		t.Fatal("expected an error for invalid json")
	}
}
