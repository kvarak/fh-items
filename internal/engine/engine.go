// Package engine implements the crafting affordability check: given a
// catalog, runtime state, and a desired selection of items to craft, it
// determines whether the selection is affordable from the active
// character's current owned items and resources.
//
// This is a strict check, not a planner: if a prerequisite item (per
// Frosthaven's item-upgrade recipes) isn't already owned in sufficient
// quantity, that's reported as a shortfall. The engine never automatically
// crafts a prerequisite from the supply and never substitutes its cost -
// those are deliberately out of scope for v1.
package engine

import (
	"fmt"
	"sort"

	"fh-items/internal/catalog"
	"fh-items/internal/state"
)

// Selection is the desired set of items to craft, keyed by item ID with
// the quantity to craft (selecting the same item twice sets quantity 2).
type Selection map[int]int

// ShortfallKind categorizes why a check failed, so callers (the API, the
// UI) can distinguish the reasons without parsing message text.
type ShortfallKind string

const (
	// ShortfallLocked means the item itself isn't unlocked, or there
	// aren't enough physical copies available - not selectable at all.
	ShortfallLocked ShortfallKind = "locked"
	// ShortfallResource means the item's own materials/herbs/gold cost
	// couldn't be paid from available resources.
	ShortfallResource ShortfallKind = "resource"
	// ShortfallPrerequisite means a required prerequisite item isn't owned
	// in sufficient quantity.
	ShortfallPrerequisite ShortfallKind = "prerequisite"
	// ShortfallOther covers anything else (e.g. an unknown item ID).
	ShortfallOther ShortfallKind = "other"
)

// Shortfall is one reason a selection couldn't be fully checked/afforded.
type Shortfall struct {
	Kind    ShortfallKind
	ItemID  int
	Message string
}

// ConsumedPrerequisite records a prerequisite item consumed from the
// active character's owned items while checking a selection.
type ConsumedPrerequisite struct {
	ItemID   int
	Quantity int
}

// Totals aggregates resources spent across the whole selection.
type Totals struct {
	Materials map[catalog.ResourceType]int
	Herbs     map[catalog.ResourceType]int
	Gold      int
}

// Result is the outcome of a feasibility check.
type Result struct {
	Feasible              bool
	Shortfalls            []Shortfall
	ConsumedPrerequisites []ConsumedPrerequisite
	Totals                Totals
	// FinalAvailableCopies/FinalOwnedCount are the resulting supply/owned
	// counts, per item ID, had the selection been crafted. Only items
	// touched by the check are present.
	FinalAvailableCopies map[int]int
	FinalOwnedCount      map[int]int
}

// CheckFeasibility determines whether sel can be crafted from st, without
// mutating st. Every prerequisite item required by a selected item must
// already be owned by the active character in sufficient quantity; a
// missing prerequisite is reported as a shortfall rather than being
// automatically crafted or substituted.
func CheckFeasibility(cat catalog.Catalog, st state.State, sel Selection) Result {
	c := &checker{
		itemsByID:       indexByID(cat.Items),
		itemStates:      st.Items,
		ownedCount:      countOwned(st.Character.OwnedItems),
		availableCopies: map[int]int{},
		materials:       cloneCounts(st.Character.Materials),
		personalHerbs:   cloneCounts(st.Character.Herbs),
		sharedHerbs:     cloneCounts(st.SharedSupply.Herbs),
		gold:            st.Character.Gold,
		herbSourcing:    st.HerbSourcing,
	}
	for id, is := range st.Items {
		c.availableCopies[id] = is.AvailableCopies
	}
	startMaterials := cloneCounts(c.materials)
	startPersonalHerbs := cloneCounts(c.personalHerbs)
	startSharedHerbs := cloneCounts(c.sharedHerbs)
	startGold := c.gold

	for _, id := range sortedKeys(sel) {
		qty := sel[id]
		if qty <= 0 {
			continue
		}
		c.checkItem(id, qty)
	}

	return Result{
		Feasible:              len(c.shortfalls) == 0,
		Shortfalls:            c.shortfalls,
		ConsumedPrerequisites: c.consumed,
		FinalAvailableCopies:  c.availableCopies,
		FinalOwnedCount:       c.ownedCount,
		Totals: Totals{
			Materials: diffCounts(startMaterials, c.materials),
			Herbs:     diffCounts(addCounts(startPersonalHerbs, startSharedHerbs), addCounts(c.personalHerbs, c.sharedHerbs)),
			Gold:      startGold - c.gold,
		},
	}
}

// checker holds the mutable scratch state for a single feasibility check.
// It never touches the real state.State - CheckFeasibility works on copies.
type checker struct {
	itemsByID       map[int]catalog.Item
	itemStates      map[int]state.ItemState
	ownedCount      map[int]int
	availableCopies map[int]int
	materials       map[catalog.ResourceType]int
	personalHerbs   map[catalog.ResourceType]int
	sharedHerbs     map[catalog.ResourceType]int
	gold            int
	herbSourcing    state.HerbSourcingRule

	consumed   []ConsumedPrerequisite
	shortfalls []Shortfall
}

// checkItem evaluates crafting qty copies of item id: it must be unlocked
// and have enough available copies, its own resource cost must be
// payable, and every prerequisite item it lists must already be owned in
// sufficient quantity. Any failure is recorded as a shortfall; checking
// continues so the result can report every problem at once.
func (c *checker) checkItem(id int, qty int) {
	item, ok := c.itemsByID[id]
	if !ok {
		c.shortfalls = append(c.shortfalls, Shortfall{Kind: ShortfallOther, ItemID: id, Message: fmt.Sprintf("item %d: not found in catalog", id)})
		return
	}

	is := c.itemStates[id]
	if !is.Unlocked {
		c.shortfalls = append(c.shortfalls, Shortfall{Kind: ShortfallLocked, ItemID: id, Message: fmt.Sprintf("item %d (%s): not unlocked", id, item.Name)})
		return
	}
	if c.availableCopies[id] < qty {
		c.shortfalls = append(c.shortfalls, Shortfall{Kind: ShortfallLocked, ItemID: id, Message: fmt.Sprintf(
			"item %d (%s): need %d available cop(ies), have %d", id, item.Name, qty, c.availableCopies[id])})
		return
	}
	c.availableCopies[id] -= qty

	if err := c.payCost(item.Cost, qty); err != nil {
		c.shortfalls = append(c.shortfalls, Shortfall{Kind: ShortfallResource, ItemID: id, Message: fmt.Sprintf("item %d (%s): %s", id, item.Name, err)})
	}

	for _, req := range item.Cost.RequiredItems {
		neededQty := req.Quantity * qty
		have := c.ownedCount[req.ItemID]
		if have < neededQty {
			reqName := fmt.Sprintf("item %d", req.ItemID)
			if reqItem, ok := c.itemsByID[req.ItemID]; ok {
				reqName = fmt.Sprintf("item %d (%s)", req.ItemID, reqItem.Name)
			}
			c.shortfalls = append(c.shortfalls, Shortfall{
				Kind:   ShortfallPrerequisite,
				ItemID: req.ItemID,
				Message: fmt.Sprintf(
					"item %d (%s): requires %d x %s owned, but only %d owned", id, item.Name, neededQty, reqName, have),
			})
			continue
		}
		c.ownedCount[req.ItemID] -= neededQty
		c.availableCopies[req.ItemID] += neededQty
		c.consumed = append(c.consumed, ConsumedPrerequisite{ItemID: req.ItemID, Quantity: neededQty})
	}
}

// payCost deducts the resource cost of crafting qty copies of an item's
// base cost (not including its prerequisite items, handled separately).
func (c *checker) payCost(cost catalog.Cost, qty int) error {
	for rt, amount := range cost.Materials {
		need := amount * qty
		if c.materials[rt] < need {
			return fmt.Errorf("need %d %s, have %d in personal supply", need, rt, c.materials[rt])
		}
		c.materials[rt] -= need
	}

	for rt, amount := range cost.Herbs {
		if err := c.spendHerb(rt, amount*qty); err != nil {
			return err
		}
	}

	for _, amount := range cost.AnyHerb {
		if err := c.spendAnyHerb(amount * qty); err != nil {
			return err
		}
	}

	need := cost.Gold * qty
	if c.gold < need {
		return fmt.Errorf("need %d gold, have %d", need, c.gold)
	}
	c.gold -= need

	return nil
}

// spendHerb deducts a specific herb type, drawing from the shared and
// personal supplies in the order given by the state's herb sourcing rule.
func (c *checker) spendHerb(rt catalog.ResourceType, need int) error {
	for _, supply := range c.herbSupplyOrder() {
		if need == 0 {
			break
		}
		take := min(need, supply[rt])
		supply[rt] -= take
		need -= take
	}
	if need > 0 {
		return fmt.Errorf("need %d more %s than personal + shared supply has", need, rt)
	}
	return nil
}

// spendAnyHerb deducts `need` units of any single freely-chosen herb type.
func (c *checker) spendAnyHerb(need int) error {
	for _, rt := range catalog.HerbTypes {
		total := 0
		for _, supply := range c.herbSupplyOrder() {
			total += supply[rt]
		}
		if total >= need {
			return c.spendHerb(rt, need)
		}
	}
	return fmt.Errorf("no single herb type has %d units available (personal + shared)", need)
}

func (c *checker) herbSupplyOrder() []map[catalog.ResourceType]int {
	switch c.herbSourcing {
	case state.HerbSourcePersonalFirst:
		return []map[catalog.ResourceType]int{c.personalHerbs, c.sharedHerbs}
	case state.HerbSourceSharedOnly:
		return []map[catalog.ResourceType]int{c.sharedHerbs}
	case state.HerbSourcePersonalOnly:
		return []map[catalog.ResourceType]int{c.personalHerbs}
	case state.HerbSourceSharedFirst:
		fallthrough
	default:
		return []map[catalog.ResourceType]int{c.sharedHerbs, c.personalHerbs}
	}
}

func indexByID(items []catalog.Item) map[int]catalog.Item {
	m := make(map[int]catalog.Item, len(items))
	for _, item := range items {
		m[item.ID] = item
	}
	return m
}

func countOwned(ownedItems []int) map[int]int {
	m := make(map[int]int, len(ownedItems))
	for _, id := range ownedItems {
		m[id]++
	}
	return m
}

func cloneCounts(m map[catalog.ResourceType]int) map[catalog.ResourceType]int {
	out := make(map[catalog.ResourceType]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func addCounts(a, b map[catalog.ResourceType]int) map[catalog.ResourceType]int {
	out := cloneCounts(a)
	for k, v := range b {
		out[k] += v
	}
	return out
}

func diffCounts(before, after map[catalog.ResourceType]int) map[catalog.ResourceType]int {
	out := map[catalog.ResourceType]int{}
	for k, v := range before {
		if d := v - after[k]; d != 0 {
			out[k] = d
		}
	}
	return out
}

func sortedKeys(sel Selection) []int {
	keys := make([]int, 0, len(sel))
	for id := range sel {
		keys = append(keys, id)
	}
	sort.Ints(keys)
	return keys
}
