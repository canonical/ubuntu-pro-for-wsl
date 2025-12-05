package cloudinit_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/cloudinit"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		breakWriteAgentData bool
		emptyConfig         bool

		wantErr         bool
		wantNoAgentYaml bool
	}{
		"Success": {},
		"No file if there is no config to write into":        {emptyConfig: true, wantNoAgentYaml: true},
		"Error when cloud-init agent file cannot be written": {breakWriteAgentData: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			publicDir := t.TempDir()

			proToken := "test token"
			if tc.emptyConfig {
				proToken = ""
			}

			conf := &mockConfig{
				proToken:       proToken,
				subcriptionErr: tc.breakWriteAgentData,
			}

			ci, err := cloudinit.New(ctx, conf, publicDir)
			if tc.wantErr {
				require.Error(t, err, "Cloud-init creation should have returned an error")
				return
			}
			require.NoError(t, err, "Cloud-init creation should have returned no error")
			require.NotEmpty(t, ci, "Cloud-init creation should have returned a CloudInit object")

			// We don't assert on specifics, as they are tested in WriteAgentData tests.
			path := filepath.Join(publicDir, ".cloud-init", "agent.yaml")
			if tc.wantNoAgentYaml {
				require.NoFileExists(t, path, "there should be no agent data file if there is no config to write into")
				return
			}
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
hostagent_uid = landscapeUID1234
`

	testCases := map[string]struct {
		// Contents
		skipProToken      bool
		skipLandscapeConf bool
		skipHostAgentUID  bool

		// Break marshalling
		breakSubscription bool
		breakLandscape    bool

		// Landscape parsing
		landscapeNoClientSection bool
		badLandscape             bool

		// Break writing to file
		breakDir          bool
		breakTempFile     bool
		breakFile         bool
		breakRemovingFile bool

		wantAgentYamlAsDir bool
	}{
		"Success":                            {},
		"Without hostagent UID":              {skipHostAgentUID: true},
		"Without pro token":                  {skipProToken: true},
		"Without Landscape":                  {skipLandscapeConf: true},
		"Without Landscape [client] section": {landscapeNoClientSection: true},
		"With empty contents":                {skipProToken: true, skipLandscapeConf: true},

		"Error to remove existing agent.yaml":   {skipProToken: true, skipLandscapeConf: true, breakRemovingFile: true, wantAgentYamlAsDir: true},
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
			require.NoError(t, err, "Setup: cloudinit.New should return no error")
			require.FileExists(t, path, "Setup: New() should have created an agent cloud-init file")

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
			if tc.skipHostAgentUID {
				conf.landscapeConf = strings.Replace(conf.landscapeConf, "hostagent_uid = landscapeUID1234", "", 1)
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

			if tc.breakRemovingFile {
				_ = os.RemoveAll(path)
				require.NoError(t, os.MkdirAll(path, 0750), "Creating the directory that breaks removing agent.yaml should not fail")
				require.NoError(t, os.WriteFile(filepath.Join(path, "child.txt"), nil, 0600), "Setup: could not create file to mess with agent.yaml")
			}
			ci.Update(ctx)

			// Assert that the file was updated (success case) or that the old one remains (error case)
			if tc.breakFile || tc.breakDir {
				// Cannot really assert on anything: we removed the old file
				return
			}

			if tc.wantAgentYamlAsDir {
				require.DirExists(t, path, "There should be a directory instead of agent.yaml")
				return
			}

			golden := testutils.Path(t)
			if _, err = os.Stat(golden); err != nil && os.IsNotExist(err) {
				// golden file doesn't exist
				require.NoFileExists(t, path, "There should not be cloud-init agent file without useful contents")
				return
			}
			got, err := os.ReadFile(path)
			require.NoError(t, err, "There should be no error reading the cloud-init agent file")

			want := testutils.LoadWithUpdateFromGolden(t, string(got))

			require.Equal(t, want, string(got), "Agent cloud-init file does not match the golden file")
		})
	}
}

type metadata struct {
	InstanceID string `yaml:"instance-id"`
}

func TestWriteDistroData(t *testing.T) {
	t.Parallel()

	const oldCloudInit = `#cloud-config
# I'm an old piece of user data
data:
	is_this_data: Yes, it is
	new: false
`

	const newCloudInit = `#cloud-config
# I'm a shiny new piece of user data
data:
	new: true
`

	testCases := map[string]struct {
		instanceID string
		// Break marshalling
		noOldData bool

		// Break writing to file
		breakDir          bool
		breakTempFile     bool
		breakFile         bool
		breakMetadataFile bool

		want         string
		wantErr      bool
		wantMetadata *metadata
	}{
		"Success":             {},
		"With no old data":    {want: newCloudInit, noOldData: true},
		"With new valid data": {want: newCloudInit},
		"With metadata":       {instanceID: "1234", wantMetadata: &metadata{InstanceID: "1234"}},

		"Error when the datadir cannot be created":       {breakDir: true, want: oldCloudInit, wantErr: true},
		"Error when the temp file cannot be written":     {breakTempFile: true, want: oldCloudInit, wantErr: true},
		"Error when the temp file cannot be renamed":     {breakFile: true, want: oldCloudInit, wantErr: true},
		"Error when the metadata file cannot be renamed": {breakMetadataFile: true, instanceID: "uid123", want: oldCloudInit, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			distroName := "CoolDistro"

			publicDir := t.TempDir()
			dir := filepath.Join(publicDir, ".cloud-init")
			path := filepath.Join(dir, distroName+".user-data")
			metadataPath := filepath.Join(dir, distroName+".meta-data")

			conf := &mockConfig{}

			// Test a clean filesystem (New calls WriteAgentData internally)
			ci, err := cloudinit.New(ctx, conf, publicDir)
			require.NoError(t, err, "Setup: cloud-init New should return no errors")

			if !tc.noOldData {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700), "Setup: could not write old distro data directory")
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

			if tc.breakMetadataFile {
				require.NoError(t, os.RemoveAll(metadataPath), "Setup: Distro cloud-init file should not fail to delete")
				require.NoError(t, os.MkdirAll(metadataPath, 0600), "Setup: could not create directory to mess with cloud-init")
			}

			if tc.breakDir {
				require.NoError(t, os.RemoveAll(dir), "Setup: Distro cloud-init file should not fail to delete")
				require.NoError(t, os.WriteFile(dir, nil, 0600), "Setup: could not create file to mess with cloud-init directory")
			}

			err = ci.WriteDistroData(distroName, tc.want, tc.instanceID)
			if tc.wantErr {
				require.Error(t, err, "WriteDistroData should have returned an error")
			} else {
				require.NoError(t, err, "WriteDistroData should return no errors")
			}

			// Assert that the file was updated (success case) or that the old one remains (error case)
			if tc.breakFile || tc.breakDir {
				// Cannot really assert on anything: we removed the old file
				return
			}

			got, err := os.ReadFile(path)
			require.NoError(t, err, "There should be no error reading the distro's cloud-init file")
			require.Equal(t, tc.want, string(got), "Agent cloud-init file does not match the golden file")

			got, err = os.ReadFile(metadataPath)
			if tc.wantMetadata == nil {
				require.Error(t, err, "Metadata file should not exist when instanceID is not supplied")
				return
			}
			require.NoError(t, err, "There should be no error reading the distro's cloud-init metadata file")
			require.NotEmpty(t, string(got), "Bazinga")
			var data metadata
			require.NoError(t, yaml.Unmarshal(got, &data), "Could not unmarshall test metadata")
			require.Equal(t, *tc.wantMetadata, data, "cloud-init metadata does not match the golden file")
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
