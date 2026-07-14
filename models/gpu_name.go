package models

import "strings"

// NormalizeGPUName trims leading/trailing whitespace and collapses every
// internal run of whitespace (spaces, tabs, newlines) into a single space,
// so GPU names reported by different platforms map to one canonical form.
func NormalizeGPUName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}
