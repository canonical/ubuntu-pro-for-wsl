package cloudinit_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/cloudinit"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

const landscapeConfigOld = `[irrelevant]
info=this section should have been omitted

[client]
data=This is an old data field
info=This is the old configuration
`

const landscapeConfigNew = `[irrelevant]
info=this section should have been omitted

[client]
info = This is the new configuration
url = www.example.com/new/rickroll
hostagent_uid = landscapeUID1234
`

func newCloudInitCustodian(t *testing.T) *securefiles.Custodian {
	t.Helper()
	publicDir := t.TempDir()
	c, err := securefiles.Open(filepath.Join(publicDir, ".cloud-init"))
	require.NoError(t, err, "Setup: could not open cloud-init custodian")
	// The custodian holds an open handle on its root directory, which on Windows blocks
	// removing that directory: it must be closed before the TempDir cleanup runs.
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		breakWriteAgentData bool
		emptyConfig         bool
		closedCustodian     bool

		wantErr         bool
		wantErrContains string
		wantNoAgentYaml bool
	}{
		"Success": {},
		"No file if there is no config to write into":        {emptyConfig: true, wantNoAgentYaml: true},
		"Error when cloud-init agent file cannot be written": {breakWriteAgentData: true, wantErr: true},
		"Error when the custodian is already closed":         {closedCustodian: true, wantErr: true, wantErrContains: "could not purge"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			custodian := newCloudInitCustodian(t)

			if tc.closedCustodian {
				// The startup purge runs on construction, so a closed custodian fails New.
				require.NoError(t, custodian.Close(), "Setup: could not close the custodian")
			}

			proToken := "test token"
			if tc.emptyConfig {
				proToken = ""
			}

			conf := &mockConfig{
				proToken:       proToken,
				subcriptionErr: tc.breakWriteAgentData,
			}

			ci, err := cloudinit.New(ctx, conf, custodian)
			if tc.wantErr {
				require.Error(t, err, "Cloud-init creation should have returned an error")
				if tc.wantErrContains != "" {
					require.Contains(t, err.Error(), tc.wantErrContains)
				}
				return
			}
			require.NoError(t, err, "Cloud-init creation should have returned no error")
			require.NotEmpty(t, ci, "Cloud-init creation should have returned a CloudInit object")

			path := filepath.Join(custodian.BasePath(), "agent.yaml")
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
	}{
		"Success":                            {},
		"Without hostagent UID":              {skipHostAgentUID: true},
		"Without pro token":                  {skipProToken: true},
		"Without Landscape":                  {skipLandscapeConf: true},
		"Without Landscape [client] section": {landscapeNoClientSection: true},
		"With empty contents":                {skipProToken: true, skipLandscapeConf: true},

		"Error obtaining pro token":             {breakSubscription: true},
		"Error obtaining Landscape config":      {breakLandscape: true},
		"Error with erroneous Landscape config": {badLandscape: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			custodian := newCloudInitCustodian(t)
			path := filepath.Join(custodian.BasePath(), "agent.yaml")

			//#nosec G101 // False positive, not real credentials.
			conf := &mockConfig{
				proToken:      "OLD_PRO_TOKEN",
				landscapeConf: landscapeConfigOld,
			}

			// Test a clean filesystem (New calls WriteAgentData internally)
			ci, err := cloudinit.New(ctx, conf, custodian)
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

			ci.Update(ctx)

			// Assert that the file was updated (success case) or that the old one remains (error case)
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

type testMetadata struct {
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
		breakFile         bool
		breakMetadataFile bool

		want         string
		wantErr      bool
		wantMetadata *testMetadata
	}{
		"Success":             {},
		"With no old data":    {want: newCloudInit, noOldData: true},
		"With new valid data": {want: newCloudInit},
		"With metadata":       {instanceID: "1234", wantMetadata: &testMetadata{InstanceID: "1234"}},

		"Error when the temp file cannot be renamed":     {breakFile: true, want: oldCloudInit, wantErr: true},
		"Error when the metadata file cannot be renamed": {breakMetadataFile: true, instanceID: "uid123", want: oldCloudInit, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			distroName := "CoolDistro"

			custodian := newCloudInitCustodian(t)
			path := filepath.Join(custodian.BasePath(), distroName+".user-data")
			metadataPath := filepath.Join(custodian.BasePath(), distroName+".meta-data")

			conf := &mockConfig{}

			// Test a clean filesystem (New calls WriteAgentData internally)
			ci, err := cloudinit.New(ctx, conf, custodian)
			require.NoError(t, err, "Setup: cloud-init New should return no errors")

			if !tc.noOldData {
				require.NoError(t, os.WriteFile(path, []byte(oldCloudInit), 0600), "Setup: could not write old distro data")
			}

			if tc.breakFile {
				require.NoError(t, os.RemoveAll(path), "Setup: Distro cloud-init file should not fail to delete")
				require.NoError(t, os.MkdirAll(path, 0600), "Setup: could not create directory to mess with cloud-init")
			}

			if tc.breakMetadataFile {
				require.NoError(t, os.RemoveAll(metadataPath), "Setup: Distro cloud-init file should not fail to delete")
				require.NoError(t, os.MkdirAll(metadataPath, 0600), "Setup: could not create directory to mess with cloud-init")
			}

			err = ci.WriteDistroData(distroName, tc.want, tc.instanceID)
			if tc.wantErr {
				require.Error(t, err, "WriteDistroData should have returned an error")
			} else {
				require.NoError(t, err, "WriteDistroData should return no errors")
			}

			// Assert that the file was updated (success case) or that the old one remains (error case)
			if tc.breakFile {
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
			var data testMetadata
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

			custodian := newCloudInitCustodian(t)
			dir := custodian.BasePath()
			path := filepath.Join(dir, distroName+".user-data")

			ci, err := cloudinit.New(ctx, &mockConfig{}, custodian)
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

func TestUpdateDoesNotLeavePartialContent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	custodian := newCloudInitCustodian(t)
	path := filepath.Join(custodian.BasePath(), "agent.yaml")

	conf := &mockConfig{
		proToken:      "token",
		landscapeConf: landscapeConfigNew,
	}
	ci, err := cloudinit.New(ctx, conf, custodian)
	require.NoError(t, err)

	// Pre-compute every complete blob the writer will publish, so the concurrent reader can
	// reject torn reads even when the fragment is itself parseable YAML. Recording the set
	// before the race begins also removes a scheduling race where the reader could observe a
	// just-published blob before the writer goroutine has recorded it.
	knownBlobs := make(map[string]struct{})
	tokens := []string{"token"}
	for i := range 20 {
		tokens = append(tokens, fmt.Sprintf("changed_token_%d", i))
	}
	for _, token := range tokens {
		conf.proToken = token
		ci.Update(ctx)
		data, err := os.ReadFile(filepath.Join(custodian.BasePath(), "agent.yaml"))
		require.NoError(t, err, "Setup: could not read cloud-init blob for token %q", token)
		knownBlobs[string(data)] = struct{}{}
	}

	stop := make(chan struct{})
	var readerErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				data, err := os.ReadFile(path)
				if err != nil {
					t.Logf("read error (transient): %v", err)
					continue
				}
				if len(data) == 0 {
					continue
				}
				if _, ok := knownBlobs[string(data)]; !ok {
					readerErr = fmt.Errorf("observed partial/foreign content not matching any published blob: %q", data)
					return
				}
			}
		}
	}()

	for i := range 20 {
		conf.proToken = fmt.Sprintf("changed_token_%d", i)
		ci.Update(ctx)
		time.Sleep(1 * time.Millisecond)
	}

	close(stop)
	wg.Wait()
	require.NoError(t, readerErr)
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

func TestSubScopedCustodianCloudInitPurge(t *testing.T) {
	ctx := context.Background()

	hook := test.NewGlobal()
	defer hook.Reset()

	parentDir := t.TempDir()
	rootCust, err := securefiles.Open(parentDir)
	require.NoError(t, err)
	defer rootCust.Close()

	cloudInitCust, err := rootCust.Subdir(".cloud-init")
	require.NoError(t, err)
	defer cloudInitCust.Close()

	// Write distro user data in sub-scoped custodian
	err = cloudInitCust.WriteFile("Noble.user-data", []byte("user-data-content"))
	require.NoError(t, err)

	conf := &mockConfig{proToken: "token"}
	ci, err := cloudinit.New(ctx, conf, cloudInitCust)
	require.NoError(t, err)

	// Verify distro user data survived and was not purged
	data, err := os.ReadFile(filepath.Join(cloudInitCust.BasePath(), "Noble.user-data"))
	require.NoError(t, err)
	require.Equal(t, []byte("user-data-content"), data)

	// Verify no unrecognised-node warning log was emitted for legitimate files or agent.yaml
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.WarnLevel && (strings.Contains(entry.Message, "Noble.user-data") || strings.Contains(entry.Message, "agent.yaml")) {
			t.Fatalf("unexpected unrecognised-node warning log for legitimate file: %s", entry.Message)
		}
	}

	_ = ci
}

// startupPurgeState carries per-row state captured by a seed and compared by
// the corresponding check (file identities that must change on recreation).
type startupPurgeState struct {
	hook *test.Hook
}

// TestStartupPurge drives cloudinit.New over a seeded .cloud-init sub-tree and
// checks, one row per scenario, what survives the startup disposition: stamped
// nodes are left untouched, whatever their name or content, and unstamped nodes
// are purged (a per-distro-looking one at error level, as it smells like
// tampering). The agent's own file is always regenerated afterwards.
func TestStartupPurge(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// prepare seeds the raw filesystem before any custodian exists, modelling
		// a root left by a released agent.
		prepare func(t *testing.T, publicDir string)
		// seed runs on the open custodian before New.
		seed func(t *testing.T, c *securefiles.Custodian, state *startupPurgeState)
		// notOwned, when non-nil, forces the ownership predicate.
		notOwned *bool
		// check runs after New succeeds.
		check func(t *testing.T, c *securefiles.Custodian, cloudInitDir string, state *startupPurgeState)
	}{
		"Preserves per-distro data on restart": {
			seed: func(t *testing.T, c *securefiles.Custodian, state *startupPurgeState) {
				t.Helper()
				require.NoError(t, c.WriteFile("CoolDistro.user-data", []byte("distro-user-data")))
				require.NoError(t, c.WriteFile("CoolDistro.meta-data", []byte("instance-id: inst-123\n")))
			},
			check: func(t *testing.T, _ *securefiles.Custodian, cloudInitDir string, state *startupPurgeState) {
				t.Helper()

				// Per-distro content survived with its metadata.
				gotMeta, err := os.ReadFile(filepath.Join(cloudInitDir, "CoolDistro.meta-data"))
				require.NoError(t, err)
				var md testMetadata
				require.NoError(t, yaml.Unmarshal(gotMeta, &md))
				require.Equal(t, "inst-123", md.InstanceID)
				gotUserData, err := os.ReadFile(filepath.Join(cloudInitDir, "CoolDistro.user-data"))
				require.NoError(t, err)
				require.Equal(t, "distro-user-data", string(gotUserData))

				// Agent.yaml carries the new token.
				gotAgent, err := os.ReadFile(filepath.Join(cloudInitDir, "agent.yaml"))
				require.NoError(t, err)
				require.Contains(t, string(gotAgent), "token")
			},
		},
		"Removes an unstamped unrecognised file": {
			notOwned: new(bool),
			seed: func(t *testing.T, c *securefiles.Custodian, state *startupPurgeState) {
				t.Helper()
				require.NoError(t, c.WriteFile("stale.txt", []byte("stale")))
			},
			check: func(t *testing.T, c *securefiles.Custodian, _ string, state *startupPurgeState) {
				t.Helper()
				_, err := os.ReadFile(filepath.Join(c.BasePath(), "stale.txt"))
				require.Error(t, err, "unstamped unrecognised file should be purged")
				foundWarning := false
				for _, entry := range state.hook.AllEntries() {
					if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "stale.txt") {
						foundWarning = true
						break
					}
				}
				require.True(t, foundWarning, "expected warning log naming stale.txt")
			},
		},
		"Preserves a lone meta-data file": {
			seed: func(t *testing.T, c *securefiles.Custodian, state *startupPurgeState) {
				t.Helper()
				require.NoError(t, c.WriteFile("LoneDistro.meta-data", []byte("instance-id: lone-inst-123\n")))
			},
			check: func(t *testing.T, c *securefiles.Custodian, _ string, state *startupPurgeState) {
				t.Helper()
				readMeta, err := os.ReadFile(filepath.Join(c.BasePath(), "LoneDistro.meta-data"))
				require.NoError(t, err)
				require.Equal(t, []byte("instance-id: lone-inst-123\n"), readMeta)
				_, err = os.ReadFile(filepath.Join(c.BasePath(), "LoneDistro.user-data"))
				require.Error(t, err, "user-data should not be fabricated for a lone meta-data file")
			},
		},
		"Purges a directory named like a distro file": {
			seed: func(t *testing.T, c *securefiles.Custodian, state *startupPurgeState) {
				t.Helper()
				require.NoError(t, os.Mkdir(filepath.Join(c.BasePath(), "DirDistro.meta-data"), 0o700))
			},
			check: func(t *testing.T, c *securefiles.Custodian, _ string, state *startupPurgeState) {
				t.Helper()
				_, err := c.ReadDir("DirDistro.meta-data")
				require.Error(t, err, "directory named like distro meta-data should be purged")
			},
		},
		"Removes leftover temporaries and a stale agent file quietly": {
			seed: func(t *testing.T, c *securefiles.Custodian, state *startupPurgeState) {
				t.Helper()
				// Raw (unstamped) churn left by an earlier crash or a pre-custodian agent.
				require.NoError(t, os.WriteFile(filepath.Join(c.BasePath(), ".tmp-agent.yaml-abcd1234"), []byte("partial"), 0600))
				require.NoError(t, os.WriteFile(filepath.Join(c.BasePath(), "agent.yaml"), []byte("stale"), 0600))
			},
			check: func(t *testing.T, c *securefiles.Custodian, _ string, state *startupPurgeState) {
				t.Helper()
				_, err := os.ReadFile(filepath.Join(c.BasePath(), ".tmp-agent.yaml-abcd1234"))
				require.Error(t, err, "leftover temporary should be purged")

				// The stale agent file was replaced by a freshly generated one.
				got, err := os.ReadFile(filepath.Join(c.BasePath(), "agent.yaml"))
				require.NoError(t, err, "agent.yaml should have been regenerated")
				require.NotEqual(t, "stale", string(got))

				// Expected churn is disposed of quietly: no warning or error names them.
				// Exception on Windows: a completely EA-less file has no clean "not
				// owned" answer there (the EA query errors), so the stale agent file
				// earns a warning at the ownership check. Its removal is still quiet.
				for _, entry := range state.hook.AllEntries() {
					if entry.Level > logrus.WarnLevel {
						continue
					}
					require.NotContains(t, entry.Message, ".tmp-", "leftover temporaries should not be reported")
					if runtime.GOOS != "windows" {
						require.NotContains(t, entry.Message, "agent.yaml", "the agent's own file should not be reported")
					}
				}
			},
		},
		"Warns and purges a node whose ownership cannot be determined": {
			seed: func(t *testing.T, c *securefiles.Custodian, state *startupPurgeState) {
				t.Helper()
				// A dangling symlink fails the ownership check: its target cannot be
				// stated, so the node is treated as foreign and purged.
				dangling := filepath.Join(c.BasePath(), "dangling.user-data")
				if err := os.Symlink(filepath.Join(c.BasePath(), "no-such-target"), dangling); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			},
			check: func(t *testing.T, c *securefiles.Custodian, _ string, state *startupPurgeState) {
				t.Helper()
				_, err := os.Lstat(filepath.Join(c.BasePath(), "dangling.user-data"))
				require.True(t, os.IsNotExist(err), "node with undeterminable ownership should be purged")

				foundWarning := false
				for _, entry := range state.hook.AllEntries() {
					if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "could not check ownership") {
						foundWarning = true
						break
					}
				}
				require.True(t, foundWarning, "expected warning about the ownership check failure")
			},
		},
		"Removes a per-distro node that is not owned": {
			notOwned: new(bool),
			seed: func(t *testing.T, c *securefiles.Custodian, state *startupPurgeState) {
				t.Helper()
				require.NoError(t, c.WriteFile("ForeignDistro.user-data", []byte("foreign-user-data")))
			},
			check: func(t *testing.T, c *securefiles.Custodian, _ string, state *startupPurgeState) {
				t.Helper()
				// The foreign content was never read back and re-blessed.
				_, err := os.ReadFile(filepath.Join(c.BasePath(), "ForeignDistro.user-data"))
				require.Error(t, err, "unowned per-distro node should not survive startup")
				foundError := false
				for _, entry := range state.hook.AllEntries() {
					if entry.Level == logrus.ErrorLevel && strings.Contains(entry.Message, "ForeignDistro.user-data") {
						foundError = true
						break
					}
				}
				require.True(t, foundError, "expected error-level log naming the unowned per-distro node")
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			publicDir := t.TempDir()
			cloudInitDir := filepath.Join(publicDir, ".cloud-init")

			if tc.prepare != nil {
				tc.prepare(t, publicDir)
			}

			custodian, err := securefiles.Open(cloudInitDir)
			require.NoError(t, err, "Setup: could not open cloud-init custodian")
			defer custodian.Close()

			if tc.notOwned != nil {
				custodian.SetMockOwned(tc.notOwned)
			}
			state := &startupPurgeState{hook: test.NewGlobal()}
			defer state.hook.Reset()
			if tc.seed != nil {
				tc.seed(t, custodian, state)
			}

			ctx := context.Background()
			conf := &mockConfig{proToken: "token"}
			_, err = cloudinit.New(ctx, conf, custodian)
			require.NoError(t, err, "Setup: cloudinit.New should succeed")

			if tc.check != nil {
				tc.check(t, custodian, cloudInitDir, state)
			}
		})
	}
}
