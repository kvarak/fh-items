package catalog

import (
	"fmt"
)

// Validate checks structural and referential integrity of the catalog. It
// returns a single error aggregating all problems found, or nil.
func (c Catalog) Validate() error {
	var errs []error

	if c.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("catalog schema version %d does not match expected %d", c.SchemaVersion, SchemaVersion))
	}
	if c.SourceChecksum == "" {
		errs = append(errs, fmt.Errorf("catalog is missing a source checksum"))
	}

	seenIDs := make(map[int]bool, len(c.Items))
	for _, item := range c.Items {
		if problems := item.validate(); len(problems) > 0 {
			for _, p := range problems {
				errs = append(errs, fmt.Errorf("item %d (%s): %w", item.ID, item.Name, p))
			}
		}
		if seenIDs[item.ID] {
			errs = append(errs, fmt.Errorf("duplicate item id %d", item.ID))
		}
		seenIDs[item.ID] = true
	}

	// Cross-reference required-item ingredients against the catalog.
	for _, item := range c.Items {
		for _, reqID := range item.Cost.RequiredItems {
			if !seenIDs[reqID] {
				errs = append(errs, fmt.Errorf("item %d (%s): requires unknown item id %d", item.ID, item.Name, reqID))
			}
		}
	}

	return joinErrors(errs)
}

func (item Item) validate() []error {
	var errs []error

	if item.ID <= 0 {
		errs = append(errs, fmt.Errorf("id must be positive"))
	}
	if item.Name == "" {
		errs = append(errs, fmt.Errorf("name must not be empty"))
	}
	if item.Count < 0 {
		errs = append(errs, fmt.Errorf("count must not be negative"))
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
	if item.Cost.Gold < 0 {
		errs = append(errs, fmt.Errorf("gold cost must not be negative"))
	}
	if item.Cost.Gold > 0 && !item.Cost.IsEmpty() && (len(item.Cost.Materials) > 0 || len(item.Cost.Herbs) > 0 || len(item.Cost.AnyHerb) > 0) {
		errs = append(errs, fmt.Errorf("item has both a gold cost and a materials/herb cost"))
	}

	switch item.RequiredBuilding {
	case "", BuildingCraftsman, BuildingJeweler, BuildingTradingPost:
	default:
		errs = append(errs, fmt.Errorf("unknown required building %q", item.RequiredBuilding))
	}
	if item.RequiredBuilding == "" && item.RequiredBuildingLevel != 0 {
		errs = append(errs, fmt.Errorf("requiredBuildingLevel set without a requiredBuilding"))
	}

	return errs
}

func isMaterialType(rt ResourceType) bool {
	for _, t := range MaterialTypes {
		if t == rt {
			return true
		}
	}
	return false
}

func isHerbType(rt ResourceType) bool {
	for _, t := range HerbTypes {
		if t == rt {
			return true
		}
	}
	return false
}

// joinErrors combines multiple errors into one, or returns nil if empty.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	msg := fmt.Sprintf("%d validation error(s):", len(errs))
	for _, e := range errs {
		msg += "\n  - " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}
