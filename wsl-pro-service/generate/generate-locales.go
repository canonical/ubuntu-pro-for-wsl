//go:build tools

// Package generate generates the i18n files
package generate

//go:generate go run ../../generate/generate_locales.go update-po generate.yaml
//go:generate go run ../../generate/generate_locales.go generate-mo generate.yaml
