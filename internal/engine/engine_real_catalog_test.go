package engine

import (
	"testing"

	"fh-items/internal/catalog"
	"fh-items/internal/state"
)

// TestCheckFeasibility_RealCatalogChain sanity-checks the engine against
// the real embedded catalog's actual upgrade chain: Kicking Boots (43)
// requires owning Sturdy Boots (28) and a Muscle Potion (90). This guards
// against the synthetic unit tests passing while the real data shape
// breaks the engine. Per the simplified v1 rule, prerequisites must
// already be owned - nothing is auto-crafted - so the test pre-populates
// ownership of the direct prerequisites rather than the whole chain.
func TestCheckFeasibility_RealCatalogChain(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load() error = %v", err)
	}
	st := state.New(cat)
	st.Buildings.Craftsman = 9 // unlock every Craftsman-gated item for this check

	byID := indexByID(cat.Items)
	for _, id := range []int{5, 18, 28, 43, 90} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("expected item %d to exist in the real catalog", id)
		}
	}
	st.RefreshBuildingGatedUnlocks(cat)
	// Muscle Potion and similar ingredients are Alchemist/blueprint items
	// with no building gate - they start locked until "discovered" in a
	// real campaign. For this mechanics sanity check, unlock everything.
	for id, is := range st.Items {
		is.Unlocked = true
		st.Items[id] = is
	}

	// Give personal resources generous enough to cover Kicking Boots' own
	// cost, and own its direct prerequisites already (no auto-crafting).
	for _, rt := range catalog.MaterialTypes {
		st.Character.Materials[rt] = 20
	}
	for _, rt := range catalog.HerbTypes {
		st.Character.Herbs[rt] = 20
	}
	st.Character.OwnedItems = []int{28, 90} // Sturdy Boots, Muscle Potion

	result := CheckFeasibility(cat, st, Selection{43: 1})
	if !result.Feasible {
		t.Fatalf("expected Kicking Boots to be feasible with its prerequisites owned, got shortfalls: %v", result.Shortfalls)
	}
}
