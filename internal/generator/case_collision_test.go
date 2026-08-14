package generator

import "testing"

// A spec carrying two endpoint names that differ only in case must not emit two
// files that differ only in case. Go rejects such a package outright, and the
// seekingalpha press died exactly this way:
//
//	case-insensitive file name collision:
//	  "seekingalpha-symbols_list_PAYX.go" and "seekingalpha-symbols_list_payx.go"
func TestUniqueFileStemDisambiguatesCaseCollisions(t *testing.T) {
	g := &Generator{}

	first := g.uniqueFileStem("symbols_list_PAYX")
	second := g.uniqueFileStem("symbols_list_payx")

	if first == second {
		t.Fatalf("identical stems returned for case-variant inputs: %q", first)
	}
	if equalFold(first, second) {
		t.Fatalf("stems still collide case-insensitively: %q vs %q", first, second)
	}

	// A distinct name is untouched.
	if got := g.uniqueFileStem("orders_list"); got != "orders_list" {
		t.Fatalf("unrelated stem was rewritten: %q", got)
	}
	// An EXACT repeat must come back unchanged. Callers depend on that to
	// detect "this command file already exists" and skip re-wiring it;
	// renaming instead emits two files declaring the same Go identifier.
	a := g.uniqueFileStem("dupe_one")
	b := g.uniqueFileStem("dupe_one")
	if a != b {
		t.Fatalf("exact repeat was renamed (%q then %q) — breaks caller skip logic", a, b)
	}
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
