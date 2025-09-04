package common_test

import (
	"strings"
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

func FuzzObfuscate(f *testing.F) {
	f.Add("Hello, World!")
	f.Add("")
	f.Add("123")
	f.Add("12a345lgsdlfasd67890")

	f.Fuzz(func(t *testing.T, input string) {
		got := common.Obfuscate(input)

		t.Log("Input: ", input)
		t.Log("Output: ", got)

		switch len(input) {
		case 0:
			require.Empty(t, got, "Obfuscate should return an empty string when the input is empty")
			return
		case 1:
			require.Equal(t, "*", got, "Obfuscate should reveal no characters when the input is a single character")
			return
		case 2:
			require.Equal(t, "**", got, "Obfuscate should reveal no characters when the input is two characters")
			return
		case 3:
			require.Equal(t, "***", got, "Obfuscate should reveal no characters when the input is three characters")
			return
		case 4:
			require.Equal(t, "****", got, "Obfuscate should reveal no characters when the input is four characters")
			return
		}

		endPrefix := 2
		beginSuffix := len(input) - 2

		require.Equal(t, input[:endPrefix], got[:endPrefix], "Obfuscate should reveal the first two characters")
		require.Equal(t, input[beginSuffix:], got[beginSuffix:], "Obfuscate should reveal the last two characters")

		nAsterisks := strings.Count(got[endPrefix:beginSuffix], "*")
		require.Equal(t, beginSuffix-endPrefix, nAsterisks, "Obfuscate should not reveal any characters in the middle")
	})
}
