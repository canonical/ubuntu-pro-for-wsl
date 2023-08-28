package common_test

import (
	"sync"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/stretchr/testify/require"
)

func TestWSLLauncher(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		want    string
		wantErr bool
	}{
		"Ubuntu":         {want: "ubuntu.exe"},
		"ubuntu":         {want: "ubuntu.exe"},
		"Ubuntu-Preview": {want: "ubuntupreview.exe"},
		"Ubuntu-18.04":   {want: "ubuntu1804.exe"},
		"Ubuntu-20.04":   {want: "ubuntu2004.exe"},
		"Ubuntu-22.04":   {want: "ubuntu2204.exe"},
		"Ubuntu-24.04":   {want: "ubuntu2404.exe"},
		"OtherDistro":    {wantErr: true},
	}

	for name, tc := range testCases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := common.WSLLauncher(name)
			if tc.wantErr {
				require.Error(t, err, "WSLLauncher should return an error")
				return
			}
			require.NoError(t, err, "WSLLauncher should return no errors")

			require.Equal(t, tc.want, got, "Unexpected return value for WSLLauncher")
		})
	}
}

func TestSet(t *testing.T) {
	set := common.NewSet[int]()
	const testSize = 10

	require.Zero(t, set.Len(), "Set should initialize empty")

	// Concurrently add items to it
	var wg sync.WaitGroup
	for i := 0; i < testSize; i++ {
		i := i
		wg.Add(1)
		go func() {
			set.Set(i)
			wg.Done()
		}()
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
		i := i
		wg.Add(1)
		go func() {
			set.Unset(i)
			wg.Done()
		}()
	}
	wg.Wait()

	// Check all items are eventually removed
	for i := 0; i < testSize; i++ {
		require.False(t, set.Has(i), "Value %d should have been removed from the set", i)
	}
}
