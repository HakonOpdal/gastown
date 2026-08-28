// Package agentaddr provides the canonical representation of a Gas Town agent
// address, together with a single parse and a single format.
//
// Gas Town writes an agent's address as a bare string in bead assignees, hook
// records, mail identities and lock names. Historically each command built that
// string its own way, so the same agent was stored under several spellings —
// "deacon" from `gt patrol`, "deacon/" from `gt sling` — and exact-match
// lookups missed rows that plainly existed. That mismatch stranded patrol wisps
// and produced the wisp backlog described in gt-cw1.
//
// There is exactly one correct string form for an address, and it is produced
// by Address.String. Callers that write an assignee should write Canonical(s);
// callers that read one should query Variants(s) so that rows written by older
// builds are still found.
package agentaddr

import (
	"fmt"
	"strings"
)

// Role identifies the kind of agent an address points at.
type Role string

const (
	RoleMayor    Role = "mayor"
	RoleDeacon   Role = "deacon"
	RoleOverseer Role = "overseer"
	RoleWitness  Role = "witness"
	RoleRefinery Role = "refinery"
	RoleCrew     Role = "crew"
	RolePolecat  Role = "polecats"
	RoleDog      Role = "dogs"
)

// bootName is the deacon variant that runs the boot watchdog.
const bootName = "boot"

// Address is the canonical, parsed form of a Gas Town agent address.
//
// Any of the fields may be empty:
//   - Rig is empty for town-level agents (mayor, deacon, boot, dogs, overseer).
//   - Name is empty for singleton roles (mayor, deacon, witness, refinery) and
//     for the bare dog-pool address "deacon/dogs".
type Address struct {
	Rig  string // rig name; empty means town level
	Role Role   // agent role
	Name string // worker name within the role; empty for singletons
}

// IsTownLevel reports whether the address names a town-level agent, i.e. one
// that is not scoped to a rig.
func (a Address) IsTownLevel() bool {
	return a.Rig == ""
}

// IsZero reports whether the address is empty.
func (a Address) IsZero() bool {
	return a.Rig == "" && a.Role == "" && a.Name == ""
}

// String returns the canonical string form of the address.
//
//	mayor            → "mayor/"
//	deacon           → "deacon/"
//	boot             → "deacon/boot"
//	dog pool         → "deacon/dogs"
//	dog alpha        → "deacon/dogs/alpha"
//	overseer         → "overseer"
//	witness          → "gastown/witness"
//	refinery         → "gastown/refinery"
//	crew max         → "gastown/crew/max"
//	polecat quartz   → "gastown/polecats/quartz"
//
// Town-level singletons keep the trailing slash: that is the form `gt sling`
// has always written and the form `gt mail` resolves against.
func (a Address) String() string {
	switch a.Role {
	case RoleOverseer:
		return "overseer"
	case RoleMayor:
		return "mayor/"
	case RoleDeacon:
		if a.Name != "" {
			return "deacon/" + a.Name
		}
		return "deacon/"
	case RoleDog:
		if a.Name != "" {
			return "deacon/dogs/" + a.Name
		}
		return "deacon/dogs"
	case RoleWitness, RoleRefinery:
		if a.Rig == "" {
			return ""
		}
		return a.Rig + "/" + string(a.Role)
	case RoleCrew, RolePolecat:
		if a.Rig == "" || a.Name == "" {
			return ""
		}
		return a.Rig + "/" + string(a.Role) + "/" + a.Name
	default:
		return ""
	}
}

// Parse converts any accepted spelling of an agent address into its canonical
// form. It is liberal in what it accepts (Postel's law): surrounding space,
// trailing slashes, mixed case in the role segment, the legacy singular
// "polecat" path segment, and the rig-scoped spelling of a town-level singleton
// ("gastown/deacon") all resolve to the same Address.
func Parse(addr string) (Address, error) {
	s := strings.TrimSpace(addr)
	s = strings.TrimRight(s, "/")
	if s == "" {
		return Address{}, fmt.Errorf("empty agent address %q", addr)
	}

	parts := strings.Split(s, "/")
	for _, part := range parts {
		if part == "" {
			return Address{}, fmt.Errorf("invalid agent address %q: empty path segment", addr)
		}
	}

	switch len(parts) {
	case 1:
		switch strings.ToLower(parts[0]) {
		case string(RoleOverseer):
			return Address{Role: RoleOverseer}, nil
		case string(RoleMayor):
			return Address{Role: RoleMayor}, nil
		case string(RoleDeacon):
			return Address{Role: RoleDeacon}, nil
		}
		return Address{}, fmt.Errorf("invalid agent address %q: bare role is not addressable", addr)

	case 2:
		head, tail := parts[0], parts[1]
		if strings.EqualFold(head, string(RoleDeacon)) {
			switch strings.ToLower(tail) {
			case bootName:
				return Address{Role: RoleDeacon, Name: bootName}, nil
			case string(RoleDog):
				return Address{Role: RoleDog}, nil
			}
			return Address{}, fmt.Errorf("invalid agent address %q: unknown deacon sub-agent %q", addr, tail)
		}
		// A rig-scoped spelling of a town-level singleton resolves to the
		// singleton: mayor and deacon are one per town, not one per rig.
		switch strings.ToLower(tail) {
		case string(RoleMayor):
			return Address{Role: RoleMayor}, nil
		case string(RoleDeacon):
			return Address{Role: RoleDeacon}, nil
		case string(RoleWitness):
			return Address{Rig: head, Role: RoleWitness}, nil
		case string(RoleRefinery):
			return Address{Rig: head, Role: RoleRefinery}, nil
		case string(RoleCrew), string(RolePolecat), "polecat":
			return Address{}, fmt.Errorf("invalid agent address %q: missing worker name", addr)
		}
		// "rig/name" is the shorthand for a polecat.
		return Address{Rig: head, Role: RolePolecat, Name: tail}, nil

	case 3:
		rig, kind, name := parts[0], parts[1], parts[2]
		if strings.EqualFold(rig, string(RoleDeacon)) && strings.EqualFold(kind, string(RoleDog)) {
			return Address{Role: RoleDog, Name: name}, nil
		}
		switch strings.ToLower(kind) {
		case string(RoleCrew):
			return Address{Rig: rig, Role: RoleCrew, Name: name}, nil
		case string(RolePolecat), "polecat":
			return Address{Rig: rig, Role: RolePolecat, Name: name}, nil
		}
		return Address{}, fmt.Errorf("invalid agent address %q: unknown agent kind %q", addr, kind)

	default:
		return Address{}, fmt.Errorf("invalid agent address %q: too many path segments", addr)
	}
}

// Canonical returns the canonical form of addr. Inputs that cannot be parsed
// are returned trimmed and otherwise unchanged, so that callers writing an
// address never silently lose an unrecognised value.
func Canonical(addr string) string {
	parsed, err := Parse(addr)
	if err != nil {
		return strings.TrimSpace(addr)
	}
	canonical := parsed.String()
	if canonical == "" {
		return strings.TrimSpace(addr)
	}
	return canonical
}

// Variants returns every spelling of addr that may appear in storage written by
// an older build, canonical form first. Readers should query all of them and
// merge the results; writers should use Canonical.
//
// Only town-level singletons have a legacy variant: `gt patrol` wrote a bare
// "deacon" where `gt sling` wrote "deacon/", which is the split that stranded
// patrol wisps. Rig-scoped addresses have only ever had one spelling.
func Variants(addr string) []string {
	canonical := Canonical(addr)
	variants := []string{canonical}

	switch canonical {
	case "mayor/":
		variants = append(variants, "mayor")
	case "deacon/":
		variants = append(variants, "deacon")
	}

	// Preserve a caller-supplied spelling that canonicalization changed, so a
	// lookup never loses rows written under the exact string it was given.
	trimmed := strings.TrimSpace(addr)
	if trimmed != "" && !contains(variants, trimmed) {
		variants = append(variants, trimmed)
	}
	return variants
}

// Equal reports whether two addresses name the same agent, regardless of
// spelling.
func Equal(a, b string) bool {
	canonicalA, errA := Parse(a)
	canonicalB, errB := Parse(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
	}
	return strings.EqualFold(canonicalA.String(), canonicalB.String())
}

// NormalizeForCompare lowercases an address and trims a trailing slash so that
// "Mayor/" and "mayor" compare equal. This is the loose, comparison-only
// normalization the mail reply matcher has always used; it is deliberately
// separate from Canonical, which produces the storage form.
func NormalizeForCompare(addr string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(addr)), "/")
}

// Rig returns the rig an address belongs to, or "" for town-level agents and
// for addresses that cannot be parsed.
func Rig(addr string) string {
	parsed, err := Parse(addr)
	if err != nil {
		return ""
	}
	return parsed.Rig
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
