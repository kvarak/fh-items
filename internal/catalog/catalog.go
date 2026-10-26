// Package catalog defines the static item/resource data model for fh-items
// and loads the generated, versioned catalog embedded in the binary.
package catalog

// SchemaVersion is bumped whenever the Catalog JSON shape changes in a
// backward-incompatible way. Generated data embeds the version it was
// built with, and loaders refuse to load a mismatched version.
const SchemaVersion = 1

// ResourceType identifies a raw crafting material or herb type.
type ResourceType string

const (
	ResourceLumber      ResourceType = "lumber"
	ResourceMetal       ResourceType = "metal"
	ResourceHide        ResourceType = "hide"
	ResourceArrowvine   ResourceType = "arrowvine"
	ResourceAxenut      ResourceType = "axenut"
	ResourceCorpsecap   ResourceType = "corpsecap"
	ResourceFlamefruit  ResourceType = "flamefruit"
	ResourceRockroot    ResourceType = "rockroot"
	ResourceSnowthistle ResourceType = "snowthistle"
)

// MaterialTypes are the raw materials (as opposed to herbs).
var MaterialTypes = []ResourceType{ResourceLumber, ResourceMetal, ResourceHide}

// HerbTypes are the specific herb resource types (as opposed to the "any
// herb" wildcard cost used by some Alchemist recipes).
var HerbTypes = []ResourceType{
	ResourceArrowvine, ResourceAxenut, ResourceCorpsecap,
	ResourceFlamefruit, ResourceRockroot, ResourceSnowthistle,
}

// BuildingType identifies a building whose level can gate item availability.
type BuildingType string

const (
	BuildingCraftsman   BuildingType = "craftsman"
	BuildingJeweler     BuildingType = "jeweler"
	BuildingTradingPost BuildingType = "tradingPost"
)

// ItemRequirement is a prerequisite item (and quantity) consumed when
// crafting another item, per Frosthaven's item-upgrade recipes.
type ItemRequirement struct {
	ItemID   int `json:"itemId"`
	Quantity int `json:"quantity"`
}

// Cost describes what must be spent to craft or buy an item. A given item
// has either a materials/herbs cost (crafted) or a Gold cost (bought), per
// the source data, never both.
type Cost struct {
	// Materials costs lumber/metal/hide, keyed by ResourceType.
	Materials map[ResourceType]int `json:"materials,omitempty"`
	// Herbs costs specific herb types, keyed by ResourceType.
	Herbs map[ResourceType]int `json:"herbs,omitempty"`
	// AnyHerb lists wildcard herb requirements; each entry is a quantity
	// of one freely-chosen herb type (e.g. [2, 1] means "2 of one herb
	// type plus 1 of a different herb type").
	AnyHerb []int `json:"anyHerb,omitempty"`
	// RequiredItems lists prerequisite items consumed as crafting
	// ingredients (Frosthaven's item-upgrade recipes).
	RequiredItems []ItemRequirement `json:"requiredItems,omitempty"`
	// Gold is the gold price for shop-purchased items.
	Gold int `json:"gold,omitempty"`
}

// Item is a single craftable or purchasable item from the catalog.
type Item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	// Slot is the equipment slot (e.g. "head", "body", "1h"); empty for
	// items with no slot (e.g. party items).
	Slot string `json:"slot,omitempty"`
	Cost Cost   `json:"cost"`
	// RequiredBuilding/RequiredBuildingLevel gate items unlocked by
	// building level. Items obtained other ways (blueprints, loot, scenario
	// rewards) leave RequiredBuilding empty.
	RequiredBuilding      BuildingType `json:"requiredBuilding,omitempty"`
	RequiredBuildingLevel int          `json:"requiredBuildingLevel,omitempty"`
}

// Catalog is the full versioned, generated item catalog.
type Catalog struct {
	SchemaVersion int `json:"schemaVersion"`
	// SourceChecksum is a checksum of the source items.json this catalog
	// was generated from, used to detect staleness.
	SourceChecksum string `json:"sourceChecksum"`
	Items          []Item `json:"items"`
}
