package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed data/catalog.gen.json
var embeddedCatalogJSON []byte

// Load parses and validates the catalog embedded in the binary. It refuses
// to load data whose schema version doesn't match the version this code
// expects, per the data-version compatibility rule: a mismatch is a build
// error, not something to silently tolerate.
func Load() (Catalog, error) {
	return LoadFrom(embeddedCatalogJSON)
}

// LoadFrom parses and validates a catalog from raw JSON bytes. It is
// exported separately from Load so tests and tools can validate arbitrary
// generated files without relying on the embedded copy.
func LoadFrom(data []byte) (Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return Catalog{}, fmt.Errorf("parsing catalog: %w", err)
	}

	if c.SchemaVersion != SchemaVersion {
		return Catalog{}, fmt.Errorf(
			"catalog schema version mismatch: data has version %d, code expects version %d; regenerate the app data with cmd/gendata",
			c.SchemaVersion, SchemaVersion,
		)
	}
	if err := c.Validate(); err != nil {
		return Catalog{}, fmt.Errorf("catalog failed validation: %w", err)
	}

	return c, nil
}
