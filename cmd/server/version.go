package main

import "runtime/debug"

// commit is set at build time with -ldflags "-X main.commit=<sha>".
var commit string

// buildCommit returns the git SHA the binary was built from, or "" if unknown.
func buildCommit() string {
	if commit != "" {
		return commit
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return ""
}
