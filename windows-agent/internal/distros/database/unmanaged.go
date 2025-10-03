package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/google/uuid"
	"github.com/ubuntu/gowsl"
	"gopkg.in/ini.v1"
)

// BasicDistroInfo contains the minimal information about a distro instance for display purposes.
type BasicDistroInfo struct {
	Name      string      // The distro instance name
	GUID      string      // The WSL internal GUID
	DistroID  string      // Same as /etc/os-relase ID
	VersionID string      // Same as /etc/os-relase VERSION_ID
	Hostname  string      // Instance hostname
	State     gowsl.State // Stopped, Running, Uninstalling etc
}

// GetUnmanagedDistros returns a list of Ubuntu instances that are currently registered in WSL but not managed by this agent.
// That's an expensive operation that involves a light wake up of all instances not previously discovered.
// This information is a snapshot at the time of the call, we don't store it.
func (db *DistroDB) GetUnmanagedDistros() (distros []BasicDistroInfo) {
	managed := db.GetAll()

	registered, err := gowsl.RegisteredDistros(db.ctx)
	if err != nil {
		log.Errorf(db.ctx, "failed to get registered distros: %v", err)
		return distros
	}

	uncRoot := selectUNCRoot(db.ctx)
	for _, d := range registered {
		// Acquiring the distro GUID from GoWSL is quite expensive the way it's implemented now, so we do it once and then pass it along.
		guid, err := d.GUID()
		if err != nil {
			log.Warningf(db.ctx, "failed to get GUID for distro %q: %v", d.Name(), err)
			continue
		}
		if slices.ContainsFunc(managed, func(m *distro.Distro) bool {
			// m.GUID() is cheap because it reads from a value already cached.
			return guid.String() == m.GUID()
		}) {
			// This is a managed distro, skip it.
			continue
		}

		info, err := basicDistroInfo(db.ctx, d, guid, uncRoot)
		if err != nil {
			log.Warningf(db.ctx, "failed to get basic info for distro %q: %v", d.Name(), err)
			continue
		}
		distros = append(distros, info)
	}

	return distros
}

// basicDistroInfo collects the minimal information about a distro instance for display purposes.
func basicDistroInfo(ctx context.Context, d gowsl.Distro, guid uuid.UUID, uncRoot string) (info BasicDistroInfo, err error) {
	// State has to be read earlier, because reading files and running commands alters it (into 'Running' ofc).
	state, err := d.State()
	if err != nil {
		return BasicDistroInfo{}, fmt.Errorf("failed to get state of distro %q: %v", d.Name(), err)
	}

	osInfo, err := readOsRelease(filepath.Join(uncRoot, d.Name()))
	if err != nil {
		return BasicDistroInfo{}, fmt.Errorf("failed to get distro basic info for %q: %v", d.Name(), err)
	}

	if osInfo.Id != "ubuntu" {
		// Our business only concerns with Ubuntu instances.
		return BasicDistroInfo{}, fmt.Errorf("skipping non-Ubuntu distro instance %q", d.Name())
	}

	hostname, err := d.Command(ctx, "hostname").CombinedOutput()
	if err != nil {
		return BasicDistroInfo{}, fmt.Errorf("failed to get hostname for %s: %v", d.Name(), err)
	}

	return BasicDistroInfo{
		Name:      d.Name(),
		GUID:      guid.String(),
		DistroID:  osInfo.Id,
		VersionID: osInfo.VersionId,
		Hostname:  strings.TrimSpace(string(hostname)),
		State:     state,
	}, nil
}

type uncRootKeyT struct{}

var uncRootKey = uncRootKeyT{}

// WithUNCRootPath returns a child context that will use the supplied Windows Universal Naming Convention
// root path to access WSL files. This is only meant for testing.
func WithUNCRootPath(ctx context.Context, uncRoot string) context.Context {
	return context.WithValue(ctx, uncRootKey, uncRoot)
}

// selectUNCRoot returns the Windows Universal Naming Convention root path to use to access WSL
// files based on the supplied context (to allow injection for testing).
func selectUNCRoot(ctx context.Context) string {
	if u, ok := ctx.Value(uncRootKey).(string); ok {
		return u
	}
	return `\\wsl.localhost\`
}

type osReleaseInfo struct {
	//nolint:revive //ini mapper is strict with naming, so we cannot rename Id -> ID as the linter suggests
	Id, VersionId, PrettyName string
}

// readOsRelease marshalls the os-release file found at either /usr/lib/os-release or /etc/os-release relative to the supplied root directory.
func readOsRelease(root string) (osReleaseInfo, error) {
	var marshaller osReleaseInfo
	// systemd docs advise reading first /etc/os-release, then /usr/lib/os-release as a fallback, although vendors are
	// supposed to ship /usr/lib/os-release and /etc/os-release is likely a symlink to it.
	// See https://www.freedesktop.org/software/systemd/man/249/os-release.html
	// Reading /etc/os-release via the Windows UNC path when it's a symlink to /usr/lib/os-release fails.
	// Since we only care about Ubuntu, which does like systemd suggests above, we read /usr/lib/os-release first.
	out, err := os.ReadFile(filepath.Join(root, "usr", "lib", "os-release"))
	if err != nil {
		errUsrLib := err
		if out, err = os.ReadFile(filepath.Join(root, "etc", "os-release")); err != nil {
			return osReleaseInfo{}, fmt.Errorf("could not read the os-release file from %s: %v", root, errors.Join(errUsrLib, err))
		}
	}

	if err := ini.MapToWithMapper(&marshaller, ini.SnackCase, out); err != nil {
		return osReleaseInfo{}, fmt.Errorf("could not parse %s: %v", root, err)
	}

	return marshaller, nil
}
