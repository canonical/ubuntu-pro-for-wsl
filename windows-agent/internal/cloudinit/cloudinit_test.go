package cloudinit_test

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"sync"
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			publicDir := t.TempDir()

			conf := &mockConfig{
				subcriptionErr: tc.breakWriteAgentData,
			}

			_, err := cloudinit.New(ctx, conf, publicDir)
			if tc.wantErr {
				require.Error(t, err, "Cloud-init creation should have returned an error")
				return
			}
			require.NoError(t, err, "Cloud-init creation should have returned no error")

			// We don't assert on specifics, as they are tested in WriteAgentData tests.
			path := filepath.Join(publicDir, ".cloud-init", "agent.yaml")
			require.FileExists(t, path, "agent data file was not created when updating the config")
		})
	}
}

func TestWriteAgentData(t *testing.T) {
	t.Parallel()

	// All error cases share a golden file so we need to protect it during updates
	var sharedGolden goldenMutex

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

		wantErr bool
	}{
		"Success":                                    {},
		"Success without pro token":                  {skipProToken: true},
		"Success without Landscape":                  {skipLandscapeConf: true},
		"Success without Landscape [client] section": {landscapeNoClientSection: true},
		"Success with empty contents":                {skipProToken: true, skipLandscapeConf: true},

		"Error obtaining pro token":             {breakSubscription: true, wantErr: true},
		"Error obtaining Landscape config":      {breakLandscape: true, wantErr: true},
		"Error with erroneous Landscape config": {badLandscape: true, wantErr: true},

		"Error when the datadir cannot be created":   {breakDir: true, wantErr: true},
		"Error when the temp file cannot be written": {breakTempFile: true, wantErr: true},
		"Error when the temp file cannot be renamed": {breakFile: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
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

			err = ci.WriteAgentData()
			var opts []testutils.Option
			if tc.wantErr {
				require.Error(t, err, "WriteAgentData should have returned an error")
				errorGolden := filepath.Join(testutils.TestFamilyPath(t), "golden", "error-cases")
				opts = append(opts, testutils.WithGoldenPath(errorGolden))
			} else {
				require.NoError(t, err, "WriteAgentData should return no errors")
			}

			// Assert that the file was updated (success case) or that the old one remains (error case)
			if tc.breakFile || tc.breakDir {
				// Cannot really assert on anything: we removed the old file
				return
			}

			got, err := os.ReadFile(path)
			require.NoError(t, err, "There should be no error reading the cloud-init agent file")

			sharedGolden.Lock()
			defer sharedGolden.Unlock()

			want := testutils.LoadWithUpdateFromGolden(t, string(got), opts...)
			require.Equal(t, want, string(got), "Agent cloud-init file does not match the golden file")
		})
	}
}

func TestDefaultDistroData(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		userErr bool
		nilUser bool

		wantErr bool
	}{
		"Success": {},
		"Success when the user cannot be obtained": {userErr: true},

		"Error when the template cannot be executed": {nilUser: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			ci, err := cloudinit.New(ctx, &mockConfig{}, t.TempDir())
			require.NoError(t, err, "Setup: cloud-init New should return no errors")

			ci.InjectUser(func() (*user.User, error) {
				if tc.userErr {
					return nil, errors.New("could not get user: mock error")
				}

				if tc.nilUser {
					return nil, nil
				}

				return &user.User{
					Username: "testuser",
					Name:     "Test User",
				}, nil
			})

			got, err := ci.DefaultDistroData(ctx)
			if tc.wantErr {
				require.Error(t, err, "DefaultDistroData should have returned an error")
				return
			}
			require.NoError(t, err, "DefaultDistroData should return no errors")

			want := golden.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "DefaultDistroData does not match the golden file")
		})
	}
}

// goldenMutex is a mutex that only works when golden update is enabled.
type goldenMutex struct {
	sync.Mutex
}

func (mu *goldenMutex) Lock() {
	if !testutils.UpdateEnabled() {
		return
	}
	mu.Mutex.Lock()
}

func (mu *goldenMutex) Unlock() {
	if !testutils.UpdateEnabled() {
		return
	}
	mu.Mutex.Unlock()
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
		"Success":                  {},
		"Success with no old data": {noOldData: true},
		"Success with empty data":  {emptyData: true},

		"Error when the datadir cannot be created":   {breakDir: true, wantErr: true},
		"Error when the temp file cannot be written": {breakTempFile: true, wantErr: true},
		"Error when the temp file cannot be renamed": {breakFile: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
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

		wantErr bool
	}{
		"Success":                                  {},
		"Success when the file did not exist":      {fileDoesNotExist: true},
		"Success when the directory did not exist": {dirDoesNotExist: true},
	}

	for name, tc := range testCases {
		tc := tc
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
				require.NoError(t, os.MkdirAll(dir, 0700), "Setup: could not set up directory")
				if !tc.fileDoesNotExist {
					require.NoError(t, os.WriteFile(path, []byte("hello, world!"), 0600), "Setup: could not set up directory")
				}
			}

			err = ci.RemoveDistroData(distroName)
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
