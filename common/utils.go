package common

import "strings"

// Obfuscate returns a partially hidden version of the contents, suitable for logging low-sensitive information.
// Hidden enough to prevent others from reading the value while still allowing the contents author to recognize it.
// Useful for reading logs with test data. For example: `Obfuscate("Blahkilull")=="Bl******ll`".
func Obfuscate(contents string) string {
	const endsToReveal = 2
	asterisksLength := len(contents) - 2*endsToReveal
	if asterisksLength < 1 {
		return strings.Repeat("*", len(contents))
	}

	return contents[0:endsToReveal] + strings.Repeat("*", asterisksLength) + contents[asterisksLength+endsToReveal:]
}
