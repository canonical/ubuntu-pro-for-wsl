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
		// A closed custodian surfaces at the first write rather than at the purge:
		// an unpurgeable node is reported and survived, so it is the inability to
		// publish the agent's own file that makes New fail.
		"Error when the custodian is already closed": {closedCustodian: true, wantErr: true, wantErrContains: "could not create agent's cloud-init file"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			custodian := newCloudInitCustodian(t)

			if tc.closedCustodian {
				// Construction both purges and writes the agent's file, so a closed
				// custodian fails New at whichever of the two cannot proceed.
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

// TestStartupPurge drives cloudinit.New over a seeded .cloud-init sub-tree and
// checks, one row per scenario, what survives the startup disposition: stamped
// nodes are left untouched, whatever their name or content, and unstamped nodes
// are purged (a per-distro-looking one at error level, as it smells like
// tampering). The agent's own file is always regenerated afterwards.
func TestStartupPurge(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// seedStamped are written through the custodian, so they carry the watermark.
		seedStamped map[string]string
		// seedRaw are written behind the custodian's back, so they carry none: what a
		// crash, a pre-custodian agent, or a stamp-less filesystem leaves behind.
		seedRaw map[string]string
		// seedDirs are directories planted in the sub-tree, foreign by shape rather than
		// by stamp. seedDangling plants a symlink whose target cannot be stated, and
		// seedUnremovable a tree the purge cannot delete (non-root POSIX only).
		seedDirs        []string
		seedDangling    string
		seedUnremovable string
		// degraded marks the filesystem as unable to carry the watermark, after seeding.
		degraded bool

		// wantFiles must still hold exactly this content; wantGone must be absent
		// entirely, whatever their shape; wantDirs must still be directories.
		wantFiles map[string]string
		wantGone  []string
		wantDirs  []string

		// wantLogContains must appear in an entry at wantLogLevel, and nothing at
		// warning or above may name quietAbout (quietAboutOnUnix only off Windows,
		// where an attribute-less file has no clean "not owned" answer).
		wantLogLevel     logrus.Level
		wantLogContains  string
		quietAbout       []string
		quietAboutOnUnix []string
	}{
		"Preserves per-distro data on restart": {
			seedStamped: map[string]string{"CoolDistro.user-data": "distro-user-data", "CoolDistro.meta-data": "instance-id: inst-123\n"},
			wantFiles:   map[string]string{"CoolDistro.user-data": "distro-user-data", "CoolDistro.meta-data": "instance-id: inst-123\n"},
		},
		// Without extended attributes every node reads as unstamped. Purging on that
		// basis would destroy the user's provisioning data on every startup.
		"Keeps per-distro data when the watermark cannot be read": {
			seedRaw:         map[string]string{"CoolDistro.user-data": "distro-user-data", "CoolDistro.meta-data": "instance-id: inst-123\n"},
			degraded:        true,
			wantFiles:       map[string]string{"CoolDistro.user-data": "distro-user-data", "CoolDistro.meta-data": "instance-id: inst-123\n"},
			wantLogLevel:    logrus.ErrorLevel,
			wantLogContains: "cannot carry the ownership watermark",
		},

		"Removes an unstamped unrecognised file": {
			seedRaw:         map[string]string{"stale.txt": "planted"},
			wantGone:        []string{"stale.txt"},
			wantLogLevel:    logrus.WarnLevel,
			wantLogContains: "stale.txt",
		},
		"Removes a per-distro node that is not owned": {
			seedRaw:         map[string]string{"ForeignDistro.user-data": "planted"},
			wantGone:        []string{"ForeignDistro.user-data"},
			wantLogLevel:    logrus.ErrorLevel,
			wantLogContains: "ForeignDistro.user-data",
		},
		"Preserves a lone meta-data file": {
			seedStamped: map[string]string{"LoneDistro.meta-data": "instance-id: lone-inst-123\n"},
			wantFiles:   map[string]string{"LoneDistro.meta-data": "instance-id: lone-inst-123\n"},
			wantGone:    []string{"LoneDistro.user-data"},
		},

		// Adoption without the watermark is unconditional for files, because nothing can
		// distinguish ours from foreign. A directory is foreign by shape, so losing the
		// watermark must not turn this sub-tree into somewhere directories survive.
		"Purges a directory even when the watermark cannot be read": {
			seedDirs: []string{"DirDistro.user-data"},
			degraded: true,
			wantGone: []string{"DirDistro.user-data"},
		},
		"Purges a directory named like a distro file": {
			seedDirs: []string{"DirDistro.meta-data"},
			wantGone: []string{"DirDistro.meta-data"},
		},

		// Refusing to start would leave the node exactly where it is, still there for
		// cloud-init to consume at first boot, and would take the agent down too.
		"Reports but survives a node it cannot remove": {
			seedUnremovable: "Foreign.user-data",
			wantDirs:        []string{"Foreign.user-data"},
			wantLogLevel:    logrus.ErrorLevel,
			wantLogContains: "could not remove every unrecognised node",
		},
		"Warns and purges a node whose ownership cannot be determined": {
			seedDangling:    "dangling.user-data",
			wantGone:        []string{"dangling.user-data"},
			wantLogLevel:    logrus.WarnLevel,
			wantLogContains: "could not check ownership",
		},
		"Removes leftover temporaries and a stale agent file quietly": {
			seedRaw:          map[string]string{".tmp-agent.yaml-abcd1234": "partial", "agent.yaml": "stale"},
			wantGone:         []string{".tmp-agent.yaml-abcd1234"},
			quietAbout:       []string{".tmp-"},
			quietAboutOnUnix: []string{"agent.yaml"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.seedUnremovable != "" && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("read-only directory semantics require a non-root Unix user")
			}

			cloudInitDir := filepath.Join(t.TempDir(), ".cloud-init")
			custodian, err := securefiles.Open(cloudInitDir)
			require.NoError(t, err, "Setup: could not open cloud-init custodian")
			defer custodian.Close()

			hook := test.NewGlobal()
			defer hook.Reset()

			for name, content := range tc.seedStamped {
				require.NoError(t, custodian.WriteFile(name, []byte(content)), "Setup: could not write %s", name)
			}
			for name, content := range tc.seedRaw {
				require.NoError(t, os.WriteFile(filepath.Join(cloudInitDir, name), []byte(content), 0600),
					"Setup: could not plant %s", name)
			}
			for _, name := range tc.seedDirs {
				require.NoError(t, os.Mkdir(filepath.Join(cloudInitDir, name), 0700), "Setup: could not plant %s", name)
			}
			if tc.seedDangling != "" {
				target := filepath.Join(cloudInitDir, "no-such-target")
				if err := os.Symlink(target, filepath.Join(cloudInitDir, tc.seedDangling)); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if tc.seedUnremovable != "" {
				keep := filepath.Join(cloudInitDir, tc.seedUnremovable, "keep")
				require.NoError(t, os.MkdirAll(keep, 0700), "Setup: could not create the obstructing tree")
				require.NoError(t, os.WriteFile(filepath.Join(keep, "child"), []byte("x"), 0600), "Setup: could not fill it")
				//nolint:gosec // G302 - test setup removes directory write permission.
				require.NoError(t, os.Chmod(keep, 0500), "Setup: could not make it read-only")
				//nolint:gosec // G302 - test teardown restores directory permissions.
				t.Cleanup(func() { _ = os.Chmod(keep, 0700) })
			}

			// Degrade after seeding: the data must predate the loss of the watermark,
			// exactly as it does when a healthy profile is later moved to a filesystem
			// without extended attributes. Only the report is overridden; every other
			// call still reaches the real sub-tree on disk.
			var dir cloudinit.Custodian = custodian
			if tc.degraded {
				dir = degradedCustodian{Custodian: custodian}
			}

			_, err = cloudinit.New(context.Background(), &mockConfig{proToken: "token"}, dir)
			require.NoError(t, err, "Setup: cloudinit.New should succeed")

			// The agent's own file is published whatever else happened, which is also
			// what proves a stale one was replaced rather than kept.
			gotAgent, err := os.ReadFile(filepath.Join(cloudInitDir, "agent.yaml"))
			require.NoError(t, err, "agent.yaml should have been written")
			require.Contains(t, string(gotAgent), "token", "agent.yaml should carry the token")

			for name, want := range tc.wantFiles {
				got, err := os.ReadFile(filepath.Join(cloudInitDir, name))
				require.NoError(t, err, "%s should have survived startup", name)
				require.Equal(t, want, string(got), "%s should have survived unchanged", name)
			}
			for _, name := range tc.wantGone {
				_, err := os.Lstat(filepath.Join(cloudInitDir, name))
				require.True(t, os.IsNotExist(err), "%s should not have survived startup", name)
			}
			for _, name := range tc.wantDirs {
				require.DirExists(t, filepath.Join(cloudInitDir, name), "%s should still be there", name)
			}

			if tc.wantLogContains != "" {
				require.True(t, loggedAt(hook, tc.wantLogLevel, tc.wantLogContains),
					"expected a %s entry mentioning %q", tc.wantLogLevel, tc.wantLogContains)
			}
			quiet := tc.quietAbout
			if runtime.GOOS != "windows" {
				quiet = append(quiet, tc.quietAboutOnUnix...)
			}
			for _, entry := range hook.AllEntries() {
				if entry.Level > logrus.WarnLevel {
					continue
				}
				for _, substr := range quiet {
					require.NotContains(t, entry.Message, substr, "expected churn to be disposed of quietly")
				}
			}
		})
	}
}

// degradedCustodian is a real custodian that reports a filesystem unable to carry the
// watermark. Everything else is delegated, so the sub-tree under test is genuine.
type degradedCustodian struct {
	cloudinit.Custodian
}

// loggedAt reports whether the hook captured an entry at the given level containing substr.
func loggedAt(hook *test.Hook, level logrus.Level, substr string) bool {
	for _, entry := range hook.AllEntries() {
		if entry.Level == level && strings.Contains(entry.Message, substr) {
			return true
		}
	}
	return false
}

func (degradedCustodian) IsDegraded() bool { return true }
