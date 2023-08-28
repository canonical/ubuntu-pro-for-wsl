package testutils

import "sync"

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
