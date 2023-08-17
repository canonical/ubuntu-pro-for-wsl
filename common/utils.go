package common

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

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

// WSLLauncher translates the name of an Ubuntu WSL distro into the base path for its launcher.
func WSLLauncher(distroName string) (string, error) {
	r := strings.NewReplacer(
		"-", "",
		".", "",
	)

	executable := strings.ToLower(r.Replace(distroName))
	executable = fmt.Sprintf("%s.exe", executable)

	// Validate executable name to protect ourselves from code injection
	switch executable {
	case "ubuntu.exe":
		return executable, nil
	case "ubuntupreview.exe":
		return executable, nil
	default:
		if regexp.MustCompile(`^ubuntu\d\d\d\d\.exe$`).MatchString(executable) {
			return executable, nil
		}
	}

	return "", fmt.Errorf("WSL launcher executable %q does not match expected pattern", executable)
}

// Set is a simple thread-safe implementation of an unordered set.
// Useful for testing.
type Set[T comparable] struct {
	data map[T]struct{}
	mu   sync.RWMutex
}

// NewSet creates a new set.
func NewSet[T comparable]() *Set[T] {
	return &Set[T]{
		data: map[T]struct{}{},
		mu:   sync.RWMutex{},
	}
}

// Len returns the count of items in the set.
func (s *Set[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data)
}

// Set adds an entry to the set.
func (s *Set[T]) Set(v T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[v] = struct{}{}
}

// Unset removes an entry from the set.
func (s *Set[T]) Unset(v T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, v)
}

// Has returns true if the set contains the specified entry.
func (s *Set[T]) Has(v T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.data[v]
	return ok
}
