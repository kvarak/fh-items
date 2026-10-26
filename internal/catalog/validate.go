package catalog

import (
	"errors"
	"fmt"
	"slices"
)

// Validate checks structural and referential integrity of the catalog. It
// returns a single error aggregating all problems found, or nil.
func (c Catalog) Validate() error {
	var errs []error

	if c.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("catalog schema version %d does not match expected %d", c.SchemaVersion, SchemaVersion))
	}
	if c.SourceChecksum == "" {
		errs = append(errs, errors.New("catalog is missing a source checksum"))
	}

	seenIDs := make(map[int]bool, len(c.Items))
	for _, item := range c.Items {
		for _, p := range item.validate() {
			errs = append(errs, fmt.Errorf("item %d (%s): %w", item.ID, item.Name, p))
		}
		if seenIDs[item.ID] {
			errs = append(errs, fmt.Errorf("duplicate item id %d", item.ID))
		}
		seenIDs[item.ID] = true
	}

	// Cross-reference required-item ingredients against the catalog.
	for _, item := range c.Items {
		for _, req := range item.Cost.RequiredItems {
			if !seenIDs[req.ItemID] {
				errs = append(errs, fmt.Errorf("item %d (%s): requires unknown item id %d", item.ID, item.Name, req.ItemID))
			}
		}
	}

	if cycle := findRequirementCycle(c.Items); cycle != nil {
		errs = append(errs, fmt.Errorf("prerequisite cycle detected: %v", cycle))
	}

	return errors.Join(errs...)
}

// findRequirementCycle walks the RequiredItems graph looking for a cycle,
// returning the item IDs forming it (or nil if the graph is acyclic).
// Frosthaven upgrade chains never loop, and prerequisite resolution relies on that.
func findRequirementCycle(items []Item) []int {
	requirementsOf := make(map[int][]int, len(items))
	for _, item := range items {
		for _, req := range item.Cost.RequiredItems {
			requirementsOf[item.ID] = append(requirementsOf[item.ID], req.ItemID)
		}
	}

	const (
		unvisited = 0
		visiting  = 1
		done      = 2
	)
	state := make(map[int]int, len(items))
	var path []int

	var visit func(id int) []int
	visit = func(id int) []int {
		switch state[id] {
		case done:
			return nil
		case visiting:
			// Found a back-edge to an ancestor: return the cycle slice.
			for i, p := range path {
				if p == id {
					return append(append([]int{}, path[i:]...), id)
				}
			}
			return []int{id}
		}
		state[id] = visiting
		path = append(path, id)
		for _, dep := range requirementsOf[id] {
			if cycle := visit(dep); cycle != nil {
				return cycle
			}
		}
		path = path[:len(path)-1]
		state[id] = done
		return nil
	}

	for _, item := range items {
		if state[item.ID] == unvisited {
			if cycle := visit(item.ID); cycle != nil {
				return cycle
			}
		}
	}
	return nil
}

func (item Item) validate() []error {
	var errs []error

	if item.ID <= 0 {
		errs = append(errs, errors.New("id must be positive"))
	}
	if item.Name == "" {
		errs = append(errs, errors.New("name must not be empty"))
	}

	for rt, qty := range item.Cost.Materials {
		if !isMaterialType(rt) {
			errs = append(errs, fmt.Errorf("materials cost uses non-material resource type %q", rt))
		}
		if qty <= 0 {
			errs = append(errs, fmt.Errorf("materials cost for %q must be positive, got %d", rt, qty))
		}
	}
	for rt, qty := range item.Cost.Herbs {
		if !isHerbType(rt) {
			errs = append(errs, fmt.Errorf("herbs cost uses non-herb resource type %q", rt))
		}
		if qty <= 0 {
			errs = append(errs, fmt.Errorf("herbs cost for %q must be positive, got %d", rt, qty))
		}
	}
	for _, qty := range item.Cost.AnyHerb {
		if qty <= 0 {
			errs = append(errs, fmt.Errorf("anyHerb cost entries must be positive, got %d", qty))
		}
	}
	seenReq := make(map[int]bool, len(item.Cost.RequiredItems))
	for _, req := range item.Cost.RequiredItems {
		if req.ItemID == item.ID {
			errs = append(errs, errors.New("requires itself as a prerequisite"))
		}
		if req.Quantity <= 0 {
			errs = append(errs, fmt.Errorf("required item %d quantity must be positive, got %d", req.ItemID, req.Quantity))
		}
		if seenReq[req.ItemID] {
			errs = append(errs, fmt.Errorf("required item %d listed more than once", req.ItemID))
		}
		seenReq[req.ItemID] = true
	}
	if item.Cost.Gold < 0 {
		errs = append(errs, errors.New("gold cost must not be negative"))
	}
	if item.Cost.Gold > 0 && (len(item.Cost.Materials) > 0 || len(item.Cost.Herbs) > 0 || len(item.Cost.AnyHerb) > 0) {
		errs = append(errs, errors.New("item has both a gold cost and a materials/herb cost"))
	}

	switch item.RequiredBuilding {
	case "", BuildingCraftsman, BuildingJeweler, BuildingTradingPost:
	default:
		errs = append(errs, fmt.Errorf("unknown required building %q", item.RequiredBuilding))
	}
	if item.RequiredBuilding == "" && item.RequiredBuildingLevel != 0 {
		errs = append(errs, errors.New("requiredBuildingLevel set without a requiredBuilding"))
	}

	return errs
}

func isMaterialType(rt ResourceType) bool { return slices.Contains(MaterialTypes, rt) }

func isHerbType(rt ResourceType) bool { return slices.Contains(HerbTypes, rt) }
