package cmd

import (
	"testing"

	"github.com/steveyegge/gastown/internal/agentaddr"
	"github.com/steveyegge/gastown/internal/session"
)

// TestPatrolAssigneeMatchesSlingAssignee pins the invariant the address split
// broke: the string `gt patrol new` stores must be the string `gt sling` stores
// for the same agent. When they diverged, `gt patrol report` could not see a
// slung patrol and stranded its wisp.
func TestPatrolAssigneeMatchesSlingAssignee(t *testing.T) {
	cases := []struct {
		name     string
		role     Role
		rig      string
		identity *session.AgentIdentity
		want     string
	}{
		{
			name:     "deacon is town level and keeps its trailing slash",
			role:     RoleDeacon,
			identity: &session.AgentIdentity{Role: session.RoleDeacon},
			want:     "deacon/",
		},
		{
			name:     "witness is qualified by its rig",
			role:     RoleWitness,
			rig:      "gastown",
			identity: &session.AgentIdentity{Role: session.RoleWitness, Rig: "gastown"},
			want:     "gastown/witness",
		},
		{
			name:     "refinery is qualified by its rig",
			role:     RoleRefinery,
			rig:      "gastown",
			identity: &session.AgentIdentity{Role: session.RoleRefinery, Rig: "gastown"},
			want:     "gastown/refinery",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := buildPatrolConfig(tc.role, RoleInfo{Role: tc.role, Rig: tc.rig})
			if err != nil {
				t.Fatalf("buildPatrolConfig: %v", err)
			}
			if cfg.Assignee != tc.want {
				t.Errorf("patrol assignee = %q, want %q", cfg.Assignee, tc.want)
			}
			if got := canonicalAssigneeAddress(tc.identity); got != tc.want {
				t.Errorf("sling assignee = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPatrolAssigneeVariantsFindLegacyRows covers the read side: rows written
// before canonicalization carry the old spelling, so a lookup that matched only
// the canonical form would skip them.
func TestPatrolAssigneeVariantsFindLegacyRows(t *testing.T) {
	cfg, err := buildPatrolConfig(RoleDeacon, RoleInfo{Role: RoleDeacon})
	if err != nil {
		t.Fatalf("buildPatrolConfig: %v", err)
	}

	variants := cfg.assigneeVariants()
	if len(variants) == 0 || variants[0] != "deacon/" {
		t.Fatalf("variants = %v, want canonical %q first", variants, "deacon/")
	}

	var sawLegacy bool
	for _, v := range variants {
		if v == "deacon" {
			sawLegacy = true
		}
	}
	if !sawLegacy {
		t.Errorf("variants %v do not include the legacy bare %q spelling", variants, "deacon")
	}
}

// TestPatrolRigNameTownLevel guards the naive-cut bug: the canonical town-level
// address ends in a slash, which cutting at the first slash reports as a rig
// named "deacon".
func TestPatrolRigNameTownLevel(t *testing.T) {
	if got := patrolRigName(PatrolConfig{Assignee: "deacon/"}); got != "" {
		t.Errorf("patrolRigName(%q) = %q, want empty (town level)", "deacon/", got)
	}
	if got := patrolRigName(PatrolConfig{Assignee: "gastown/witness"}); got != "gastown" {
		t.Errorf("patrolRigName(%q) = %q, want %q", "gastown/witness", got, "gastown")
	}
}

// TestAddressForRoleRejectsIncompleteRoles ensures a rig-scoped role detected
// without its rig is refused rather than stored as a bare pool name.
func TestAddressForRoleRejectsIncompleteRoles(t *testing.T) {
	if addr, ok := addressForRole(RoleWitness, "", ""); ok {
		t.Errorf("addressForRole(witness, no rig) = %q, want rejected", addr.String())
	}
	if addr, ok := addressForRole(RolePolecat, "gastown", ""); ok {
		t.Errorf("addressForRole(polecat, no name) = %q, want rejected", addr.String())
	}
	addr, ok := addressForRole(RoleDog, "", "alpha")
	if !ok {
		t.Fatalf("addressForRole(dog, alpha) rejected, want accepted")
	}
	if got := addr.String(); got != "deacon/dogs/alpha" {
		t.Errorf("dog address = %q, want %q", got, "deacon/dogs/alpha")
	}
}

// TestNormalizeAddressPreservesMailBehaviour pins that routing mail through the
// shared package did not change how mail resolves an address.
func TestNormalizeAddressPreservesMailBehaviour(t *testing.T) {
	cases := map[string]string{
		"Mayor/":          "mayor",
		"mayor":           "mayor",
		"  deacon/  ":     "deacon",
		"gastown/Witness": "gastown/witness",
	}
	for in, want := range cases {
		if got := normalizeAddress(in); got != want {
			t.Errorf("normalizeAddress(%q) = %q, want %q", in, got, want)
		}
		if got := agentaddr.MatchKey(in); got != want {
			t.Errorf("agentaddr.MatchKey(%q) = %q, want %q", in, got, want)
		}
	}
}
