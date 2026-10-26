package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// Gloomhaven item ids, names, slots, and gold costs are from the Gloomhaven
// item cards; the Trading Post levels are the Frosthaven import sets.
//
//go:embed data/gloomhaven.json
var embeddedGloomhavenJSON []byte

// GloomhavenItem is a Gloomhaven item that can be imported via the Trading Post.
type GloomhavenItem struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Slot             string `json:"slot"`
	Gold             int    `json:"gold"`
	TradingPostLevel int    `json:"tradingPostLevel"`
}

// LoadGloomhaven parses the embedded Gloomhaven import list.
func LoadGloomhaven() ([]GloomhavenItem, error) {
	var items []GloomhavenItem
	if err := json.Unmarshal(embeddedGloomhavenJSON, &items); err != nil {
		return nil, fmt.Errorf("parsing gloomhaven items: %w", err)
	}
	return items, nil
}
