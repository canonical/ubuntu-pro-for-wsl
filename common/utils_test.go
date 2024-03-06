package common_test

import (
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
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
