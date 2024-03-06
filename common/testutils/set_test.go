package testutils_test

import (
	"sync"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	set := testutils.NewSet[int]()
	const testSize = 10

	require.Zero(t, set.Len(), "Set should initialize empty")

	// Concurrently add items to it
	var wg sync.WaitGroup
	for i := 0; i < testSize; i++ {
		wg.Add(1)
		go func(i int) {
			set.Set(i)
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Check all items are eventually added
	for i := 0; i < testSize; i++ {
		require.True(t, set.Has(i), "Value %d should have been added to the set", i)
	}
	require.Equal(t, testSize, set.Len(), "Set should have all items in it")

	// Concurrently remove items
	wg = sync.WaitGroup{}
	for i := 0; i < testSize; i++ {
		wg.Add(1)
		go func(i int) {
			set.Unset(i)
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Check all items are eventually removed
	for i := 0; i < testSize; i++ {
		require.False(t, set.Has(i), "Value %d should have been removed from the set", i)
	}
}
