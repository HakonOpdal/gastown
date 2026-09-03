package cmd

import (
	"fmt"

	"github.com/steveyegge/gastown/internal/agentaddr"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/style"
)

// maxStepPropagationDepth bounds the walk over a molecule's step tree so a
// malformed parent chain cannot spin forever.
const maxStepPropagationDepth = 8

// stepNeedsAssignee reports whether a step wisp should be re-pointed at the
// molecule's resolved address.
//
// Closed steps are left alone: their assignee is history, and rewriting it
// costs a Dolt commit for no lookup benefit.
func stepNeedsAssignee(step *beads.Issue, targetAgent string) bool {
	if step == nil || step.ID == "" {
		return false
	}
	if step.Status == "closed" {
		return false
	}
	return !agentaddr.Equal(step.Assignee, targetAgent)
}

// stepsNeedingAssignee returns the IDs of the steps that should be re-pointed.
func stepsNeedingAssignee(steps []*beads.Issue, targetAgent string) []string {
	var ids []string
	for _, step := range steps {
		if stepNeedsAssignee(step, targetAgent) {
			ids = append(ids, step.ID)
		}
	}
	return ids
}

// propagateAssigneeToSteps points every open step wisp beneath rootID at the
// same address as the root.
//
// `bd mol wisp` writes the child steps itself, and it has no way to know which
// agent the molecule was dispatched to: the root gets the resolved address
// while its steps inherit the bare pool role, so a molecule dispatched to
// "deacon/dogs/alpha" leaves five steps assigned to "dog" (gt-cw1, symptom 2).
// Those steps are then invisible to every per-agent lookup. Propagating after
// the root is hooked closes that gap at the one place dispatch knows the answer.
//
// Returns the number of steps re-pointed. Errors are returned but callers treat
// them as non-fatal: the molecule is already dispatched by this point.
func propagateAssigneeToSteps(b *beads.Beads, rootID, targetAgent string) (int, error) {
	if b == nil || rootID == "" || targetAgent == "" {
		return 0, nil
	}
	canonical := agentaddr.Canonical(targetAgent)

	updated := 0
	var firstErr error
	frontier := []string{rootID}
	seen := map[string]bool{rootID: true}

	for depth := 0; depth < maxStepPropagationDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, parentID := range frontier {
			children, err := listChildrenAcrossTables(b, parentID)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("listing steps of %s: %w", parentID, err)
				}
				continue
			}
			for _, child := range children {
				if child == nil || child.ID == "" || seen[child.ID] {
					continue
				}
				seen[child.ID] = true
				next = append(next, child.ID)

				if !stepNeedsAssignee(child, canonical) {
					continue
				}
				if err := b.Update(child.ID, beads.UpdateOptions{Assignee: &canonical}); err != nil {
					if firstErr == nil {
						firstErr = fmt.Errorf("assigning step %s to %s: %w", child.ID, canonical, err)
					}
					continue
				}
				updated++
			}
		}
		frontier = next
	}

	return updated, firstErr
}

// propagateMoleculeAssignee points a dispatched molecule and its step wisps at
// the agent the work was dispatched to.
//
// The root is only filled in when it has no assignee yet — in the
// formula-on-bead path the hook lands on the base bead, leaving the bonded wisp
// root blank — so an address resolved by dispatch is never overwritten here.
//
// Failures are reported as warnings: the work is already on the agent's hook by
// the time this runs, and a missed step assignee degrades lookups rather than
// losing the dispatch.
func propagateMoleculeAssignee(townRoot, rootID, targetAgent, workDir string) {
	if rootID == "" || targetAgent == "" {
		return
	}
	canonical := agentaddr.Canonical(targetAgent)
	b := beads.New(beads.ResolveHookDir(townRoot, rootID, workDir))

	if root, err := b.Show(rootID); err == nil && root != nil && root.Assignee == "" {
		if err := b.Update(rootID, beads.UpdateOptions{Assignee: &canonical}); err != nil {
			style.PrintWarning("could not assign molecule root %s to %s: %v", rootID, canonical, err)
		}
	}

	updated, err := propagateAssigneeToSteps(b, rootID, canonical)
	if err != nil {
		style.PrintWarning("could not propagate assignee to steps of %s: %v", rootID, err)
	}
	if updated > 0 {
		fmt.Printf("%s Assigned %d molecule step(s) to %s\n", style.Bold.Render("\u2713"), updated, canonical)
	}
}
