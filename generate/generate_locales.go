// Package main generates pot, po, and mo files to enable i18n.
// Use `go run generate_locales.go help` to see usage.
package main

import (
	"github.com/canonical/ubuntu-pro-for-wsl/generate/internal/locales"
)

func main() {
	locales.Main()
}
