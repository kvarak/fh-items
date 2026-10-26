// Package web embeds the static frontend assets and HTML templates served
// by cmd/server.
package web

import "embed"

//go:embed static
var Static embed.FS

//go:embed templates
var Templates embed.FS
