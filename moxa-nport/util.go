package main

import (
	"path/filepath"
	"strings"
)

// outputSuffixFromConfig returns a string like "_config-name" (or empty) based on the config filename.
func outputSuffixFromConfig(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" || name == "config" {
		return ""
	}
	return "_" + name
}
