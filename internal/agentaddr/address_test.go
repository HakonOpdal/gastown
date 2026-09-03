package agentaddr

import (
	"reflect"
	"testing"
)

func TestParseCanonicalForms(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Address
		str  string
	}{
		// Town-level singletons — the trailing-slash split behind gt-cw1.
		{"bare deacon", "deacon", Address{Role: RoleDeacon}, "deacon/"},
		{"deacon trailing slash", "deacon/", Address{Role: RoleDeacon}, "deacon/"},
		{"deacon padded", "  deacon  ", Address{Role: RoleDeacon}, "deacon/"},
		{"deacon mixed case", "Deacon/", Address{Role: RoleDeacon}, "deacon/"},
		{"deacon repeated slashes", "deacon///", Address{Role: RoleDeacon}, "deacon/"},
		{"bare mayor", "mayor", Address{Role: RoleMayor}, "mayor/"},
		{"mayor trailing slash", "mayor/", Address{Role: RoleMayor}, "mayor/"},
		{"rig scoped mayor", "gastown/mayor", Address{Role: RoleMayor}, "mayor/"},
		{"rig scoped deacon", "gastown/deacon", Address{Role: RoleDeacon}, "deacon/"},
		{"overseer", "overseer", Address{Role: RoleOverseer}, "overseer"},
		{"boot", "deacon/boot", Address{Role: RoleDeacon, Name: "boot"}, "deacon/boot"},

		// Dogs live under the deacon, not under a rig.
		{"dog pool", "deacon/dogs", Address{Role: RoleDog}, "deacon/dogs"},
		{"dog named", "deacon/dogs/alpha", Address{Role: RoleDog, Name: "alpha"}, "deacon/dogs/alpha"},

		// Rig-scoped singletons.
		{"witness", "gastown/witness", Address{Rig: "gastown", Role: RoleWitness}, "gastown/witness"},
		{"witness trailing slash", "gastown/witness/", Address{Rig: "gastown", Role: RoleWitness}, "gastown/witness"},
		{"refinery", "sandbox/refinery", Address{Rig: "sandbox", Role: RoleRefinery}, "sandbox/refinery"},

		// Named workers.
		{"polecat", "gastown/polecats/quartz", Address{Rig: "gastown", Role: RolePolecat, Name: "quartz"}, "gastown/polecats/quartz"},
		{"polecat legacy singular", "gastown/polecat/quartz", Address{Rig: "gastown", Role: RolePolecat, Name: "quartz"}, "gastown/polecats/quartz"},
		{"polecat shorthand", "gastown/quartz", Address{Rig: "gastown", Role: RolePolecat, Name: "quartz"}, "gastown/polecats/quartz"},
		{"crew", "gastown/crew/max", Address{Rig: "gastown", Role: RoleCrew, Name: "max"}, "gastown/crew/max"},
		{"worker name case preserved", "gastown/polecats/Toast", Address{Rig: "gastown", Role: RolePolecat, Name: "Toast"}, "gastown/polecats/Toast"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse(c.in)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", c.in, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
			}
			if str := got.String(); str != c.str {
				t.Errorf("Parse(%q).String() = %q, want %q", c.in, str, c.str)
			}
			if canonical := Canonical(c.in); canonical != c.str {
				t.Errorf("Canonical(%q) = %q, want %q", c.in, canonical, c.str)
			}
		})
	}
}

func TestParseRejectsUnaddressableInput(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"only slashes", "///"},
		{"bare pool role", "dog"},
		{"bare witness", "witness"},
		{"bare refinery", "refinery"},
		{"missing rig", "/witness"},
		{"polecats without name", "gastown/polecats"},
		{"crew without name", "gastown/crew"},
		{"unknown deacon sub-agent", "deacon/kennel"},
		{"unknown kind", "gastown/mechanics/joe"},
		{"too deep", "gastown/polecats/quartz/extra"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, err := Parse(c.in); err == nil {
				t.Errorf("Parse(%q) = %+v, want error", c.in, got)
			}
		})
	}
}

// Canonical must never discard a value it does not understand: an unparseable
// assignee is passed through so a write cannot silently blank a bead.
func TestCanonicalPassesThroughUnparseableInput(t *testing.T) {
	for _, in := range []string{"dog", "queue:releases", "channel:town-square", "  announce:all  "} {
		want := trimmed(in)
		if got := Canonical(in); got != want {
			t.Errorf("Canonical(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsTownLevel(t *testing.T) {
	cases := map[string]bool{
		"deacon":                 true,
		"deacon/":                true,
		"mayor/":                 true,
		"deacon/dogs/alpha":      true,
		"deacon/boot":            true,
		"overseer":               true,
		"gastown/witness":        false,
		"gastown/polecats/toast": false,
		"gastown/crew/max":       false,
	}
	for in, want := range cases {
		parsed, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if got := parsed.IsTownLevel(); got != want {
			t.Errorf("Parse(%q).IsTownLevel() = %v, want %v", in, got, want)
		}
	}
}

// Variants is what lets a reader still find rows written by an older build.
// The town-level pair is the exact split that stranded patrol wisps: `gt sling`
// wrote "deacon/" while `gt patrol` wrote and matched a bare "deacon".
func TestVariants(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"deacon", []string{"deacon/", "deacon"}},
		{"deacon/", []string{"deacon/", "deacon"}},
		{"mayor", []string{"mayor/", "mayor"}},
		{"gastown/witness", []string{"gastown/witness"}},
		{"gastown/polecats/quartz", []string{"gastown/polecats/quartz"}},
		{"gastown/polecat/quartz", []string{"gastown/polecats/quartz", "gastown/polecat/quartz"}},
		{"dog", []string{"dog"}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := Variants(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Variants(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"deacon", "deacon/", true},
		{"Deacon/", "deacon", true},
		{"gastown/deacon", "deacon/", true},
		{"gastown/polecats/quartz", "gastown/quartz", true},
		{"gastown/polecat/quartz", "gastown/polecats/quartz", true},
		{"deacon", "mayor", false},
		{"gastown/witness", "sandbox/witness", false},
		{"deacon/dogs/alpha", "deacon/dogs/beta", false},
		{"dog", "dog", true},
		{"dog", "deacon/dogs", false},
	}
	for _, c := range cases {
		if got := Equal(c.a, c.b); got != c.want {
			t.Errorf("Equal(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestRig(t *testing.T) {
	cases := map[string]string{
		"deacon/":                "",
		"deacon":                 "",
		"mayor/":                 "",
		"deacon/dogs/alpha":      "",
		"gastown/witness":        "gastown",
		"sandbox/refinery":       "sandbox",
		"gastown/polecats/toast": "gastown",
		"dog":                    "",
	}
	for in, want := range cases {
		if got := Rig(in); got != want {
			t.Errorf("Rig(%q) = %q, want %q", in, got, want)
		}
	}
}

// NormalizeForCompare must keep the exact semantics the mail reply matcher
// relied on before it was moved here.
func TestNormalizeForCompare(t *testing.T) {
	cases := map[string]string{
		"mayor/":                 "mayor",
		"Mayor/":                 "mayor",
		"mayor":                  "mayor",
		"  deacon/  ":            "deacon",
		"gastown/polecats/Toast": "gastown/polecats/toast",
		"":                       "",
	}
	for in, want := range cases {
		if got := NormalizeForCompare(in); got != want {
			t.Errorf("NormalizeForCompare(%q) = %q, want %q", in, got, want)
		}
	}
}

func trimmed(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
