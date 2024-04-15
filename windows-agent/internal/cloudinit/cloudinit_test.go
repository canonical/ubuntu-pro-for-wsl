package cloudinit_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/cloudinit"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		breakWriteAgentData bool
		wantErr             bool
	}{
		"Success": {},
		"Error when cloud-init agent file cannot be written": {breakWriteAgentData: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			publicDir := t.TempDir()

			conf := &mockConfig{
				subcriptionErr: tc.breakWriteAgentData,
			}

			ci, err := cloudinit.New(ctx, conf, publicDir)
			if tc.wantErr {
				require.Error(t, err, "Cloud-init creation should have returned an error")
				require.Nil(t, ci, "Cloud-init creation should not have returned a CloudInit object")
				return
			}
			require.NoError(t, err, "Cloud-init creation should have returned no error")
			require.NotNil(t, ci, "Cloud-init creation should have returned a CloudInit object")

			// We don't assert on specifics, as they are tested in WriteAgentData tests.
			path := filepath.Join(publicDir, ".cloud-init", "agent.yaml")
			require.FileExists(t, path, "agent data file was not created when updating the config")
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	const landscapeConfigOld string = `[irrelevant]
info=this section should have been omitted

[client]
data=This is an old data field
info=This is the old configuration
`

	const landscapeConfigNew string = `[irrelevant]
info=this section should have been omitted

[client]
info = This is the new configuration
url = www.example.com/new/rickroll
`

	testCases := map[string]struct {
		// Contents
		skipProToken      bool
		skipLandscapeConf bool

		// Break marshalling
		breakSubscription bool
		breakLandscape    bool

		// Landcape parsing
		landscapeNoClientSection bool
		badLandscape             bool

		// Break writing to file
		breakDir      bool
		breakTempFile bool
		breakFile     bool
	}{
		"Success":                            {},
		"Without pro token":                  {skipProToken: true},
		"Without Landscape":                  {skipLandscapeConf: true},
		"Without Landscape [client] section": {landscapeNoClientSection: true},
		"With empty contents":                {skipProToken: true, skipLandscapeConf: true},

		"Error obtaining pro token":             {breakSubscription: true},
		"Error obtaining Landscape config":      {breakLandscape: true},
		"Error with erroneous Landscape config": {badLandscape: true},

		"Error when the datadir cannot be created":   {breakDir: true},
		"Error when the temp file cannot be written": {breakTempFile: true},
		"Error when the temp file cannot be renamed": {breakFile: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			publicDir := t.TempDir()
			dir := filepath.Join(publicDir, ".cloud-init")
			path := filepath.Join(dir, "agent.yaml")

			conf := &mockConfig{
				proToken:      "OLD_PRO_TOKEN",
				landscapeConf: landscapeConfigOld,
			}

			// Test a clean filesystem (New calls WriteAgentData internally)
			ci, err := cloudinit.New(ctx, conf, publicDir)
			require.NoError(t, err, "cloudinit.New should return no error")
			require.FileExists(t, path, "New() should have created an agent cloud-init file")

			// Test overriding the file: New() created the agent.yaml file
			conf.subcriptionErr = tc.breakSubscription
			conf.landscapeErr = tc.breakLandscape

			conf.proToken = "NEW_PRO_TOKEN"
			if tc.skipProToken {
				conf.proToken = ""
			}

			conf.landscapeConf = landscapeConfigNew
			if tc.badLandscape {
				conf.landscapeConf = "This is not valid ini"
			}
			if tc.landscapeNoClientSection {
				conf.landscapeConf = "[irrelevant]\ninfo=This section should be ignored"
			}
			if tc.skipLandscapeConf {
				conf.landscapeConf = ""
			}

			if tc.breakTempFile {
				require.NoError(t, os.RemoveAll(path+".tmp"), "Setup: Agent cloud-init file should not fail to delete")
				require.NoError(t, os.MkdirAll(path+".tmp", 0600), "Setup: could not create directory to mess with cloud-init")
			}

			if tc.breakFile {
				require.NoError(t, os.RemoveAll(path), "Setup: Agent cloud-init file should not fail to delete")
				require.NoError(t, os.MkdirAll(path, 0600), "Setup: could not create directory to mess with cloud-init")
			}

			if tc.breakDir {
				require.NoError(t, os.RemoveAll(dir), "Setup: Agent cloud-init file should not fail to delete")
				require.NoError(t, os.WriteFile(dir, nil, 0600), "Setup: could not create file to mess with cloud-init directory")
			}

			ci.Update(ctx)

			// Assert that the file was updated (success case) or that the old one remains (error case)
			if tc.breakFile || tc.breakDir {
				// Cannot really assert on anything: we removed the old file
				return
			}

			got, err := os.ReadFile(path)
			require.NoError(t, err, "There should be no error reading the cloud-init agent file")

			want := testutils.LoadWithUpdateFromGolden(t, string(got))

			require.Equal(t, want, string(got), "Agent cloud-init file does not match the golden file")
		})
	}
}

func TestWriteDistroData(t *testing.T) {
	t.Parallel()

	const oldCloudInit = `# cloud-init
# I'm an old piece of user data
data:
	is_this_data: Yes, it is
	new: false
`

	const newCloudInit = `# cloud-init
# I'm a shiny new piece of user data
data:
	new: true
`

	testCases := map[string]struct {
		// Break marshalling
		emptyData bool
		noOldData bool

		// Break writing to file
		breakDir      bool
		breakTempFile bool
		breakFile     bool

		wantErr bool
	}{
		"Success":          {},
		"With no old data": {noOldData: true},
		"With empty data":  {emptyData: true},

		"Error when the datadir cannot be created":   {breakDir: true, wantErr: true},
		"Error when the temp file cannot be written": {breakTempFile: true, wantErr: true},
		"Error when the temp file cannot be renamed": {breakFile: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			distroName := "CoolDistro"

			publicDir := t.TempDir()
			dir := filepath.Join(publicDir, ".cloud-init")
			path := filepath.Join(dir, distroName+".user-data")

			conf := &mockConfig{}

			// Test a clean filesystem (New calls WriteAgentData internally)
			ci, err := cloudinit.New(ctx, conf, publicDir)
			require.NoError(t, err, "Setup: cloud-init New should return no errors")

			if !tc.noOldData {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0600), "Setup: could not write old distro data directory")
				require.NoError(t, os.WriteFile(path, []byte(oldCloudInit), 0600), "Setup: could not write old distro data")
			}

			if tc.breakTempFile {
				require.NoError(t, os.RemoveAll(path+".tmp"), "Setup: Distro cloud-init file should not fail to delete")
				require.NoError(t, os.MkdirAll(path+".tmp", 0600), "Setup: could not create directory to mess with cloud-init")
			}

			if tc.breakFile {
				require.NoError(t, os.RemoveAll(path), "Setup: Distro cloud-init file should not fail to delete")
				require.NoError(t, os.MkdirAll(path, 0600), "Setup: could not create directory to mess with cloud-init")
			}

			if tc.breakDir {
				require.NoError(t, os.RemoveAll(dir), "Setup: Distro cloud-init file should not fail to delete")
				require.NoError(t, os.WriteFile(dir, nil, 0600), "Setup: could not create file to mess with cloud-init directory")
			}

			var input string
			if !tc.emptyData {
				input = newCloudInit
			}

			err = ci.WriteDistroData(distroName, input)
			var want string
			if tc.wantErr {
				require.Error(t, err, "WriteAgentData should have returned an error")
				want = oldCloudInit
			} else {
				require.NoError(t, err, "WriteAgentData should return no errors")
				want = input
			}

			// Assert that the file was updated (success case) or that the old one remains (error case)
			if tc.breakFile || tc.breakDir {
				// Cannot really assert on anything: we removed the old file
				return
			}

			got, err := os.ReadFile(path)
			require.NoError(t, err, "There should be no error reading the distro's cloud-init file")
			require.Equal(t, want, string(got), "Agent cloud-init file does not match the golden file")
		})
	}
}

func TestRemoveDistroData(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileDoesNotExist bool
		dirDoesNotExist  bool
		fileIsDir        bool

		wantErr bool
	}{
		"Success":                                  {},
		"Success when the file did not exist":      {fileDoesNotExist: true},
		"Success when the directory did not exist": {dirDoesNotExist: true},

		"Error when file cannot be removed": {fileIsDir: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			distroName := "CoolDistro"

			publicDir := t.TempDir()
			dir := filepath.Join(publicDir, ".cloud-init")
			path := filepath.Join(dir, distroName+".user-data")

			ci, err := cloudinit.New(ctx, &mockConfig{}, publicDir)
			require.NoError(t, err, "Setup: cloud-init New should return no errors")

			if !tc.dirDoesNotExist {
				if tc.fileIsDir {
					// cloud-init will try to remove the file, but it is a directory
					dir = path
					// and the directory is not empty, thus remove should fail.
					path = filepath.Join(dir, distroName+".user-data")
				}
				require.NoError(t, os.MkdirAll(dir, 0700), "Setup: could not set up directory")
				if !tc.fileDoesNotExist {
					require.NoError(t, os.WriteFile(path, []byte("hello, world!"), 0600), "Setup: could not set up directory")
				}
			}

			err = ci.RemoveDistroData(distroName)
			if tc.wantErr {
				require.Error(t, err, "RemoveDistroData should return an error")
				require.FileExists(t, path, "RemoveDistroData should not have removed the distro cloud-init data file")
				return
			}
			require.NoError(t, err, "RemoveDistroData should return no errors")
			require.NoFileExists(t, path, "RemoveDistroData should remove the distro cloud-init data file")
		})
	}
}

type mockConfig struct {
	proToken       string
	subcriptionErr bool

	landscapeConf string
	landscapeErr  bool
}

func (c mockConfig) Subscription() (string, config.Source, error) {
	if c.subcriptionErr {
		return "", config.SourceNone, errors.New("culd not get subscription: mock error")
	}

	if c.proToken == "" {
		return "", config.SourceNone, nil
	}

	return c.proToken, config.SourceUser, nil
}

func (c mockConfig) LandscapeClientConfig() (string, config.Source, error) {
	if c.landscapeErr {
		return "", config.SourceNone, errors.New("could not get landscape configuration: mock error")
	}

	if c.landscapeConf == "" {
		return "", config.SourceNone, nil
	}

	return c.landscapeConf, config.SourceUser, nil
}
