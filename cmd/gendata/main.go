// Command gendata regenerates the embedded app catalog from the
// authoritative, immutable source data at .github/docs/items.json.
//
// Usage:
//
//	go run ./cmd/gendata [-source path] [-out path]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"fh-items/internal/catalog"
)

func main() {
	sourcePath := flag.String("source", "../.github/docs/items.json", "path to the immutable source items.json")
	outPath := flag.String("out", "internal/catalog/data/catalog.gen.json", "path to write the generated catalog")
	flag.Parse()

	if err := run(*sourcePath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, "gendata:", err)
		os.Exit(1)
	}
}

func run(sourcePath, outPath string) error {
	sourceJSON, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("reading source file: %w", err)
	}

	cat, err := catalog.GenerateFromSource(sourceJSON)
	if err != nil {
		return fmt.Errorf("generating catalog: %w", err)
	}
	if err := cat.Validate(); err != nil {
		return fmt.Errorf("generated catalog failed validation: %w", err)
	}

	out, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding catalog: %w", err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}

	fmt.Printf("wrote %d items (schema v%d, source checksum %s) to %s\n",
		len(cat.Items), cat.SchemaVersion, cat.SourceChecksum, outPath)
	return nil
}
