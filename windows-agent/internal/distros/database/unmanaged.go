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
		return
	}

	for _, d := range registered {
		// Acquiring the distro GUID from GoWSL is quite expensive the way it's implemented now, so we do it once and then pass it along.
		// TODO: Make this more elegant when d.GUID() becomes cheaper.
		g, err := d.GUID()
		if err != nil {
			log.Warningf(db.ctx, "failed to get GUID for distro %q: %v", d.Name(), err)
			continue
		}
		guid := g.String()
		if slices.ContainsFunc(managed, func(m *distro.Distro) bool {
			// The m.GUID() here is cheap because it reads from a value already cached.
			return guid == m.GUID()
		}) {
			// This is a managed distro, skip it.
			continue
		}

		info, err := basicDistroInfo(db.ctx, g, d)
		if err != nil {
			log.Warningf(db.ctx, "failed to get distro basic info for %q: %v", d.Name(), err)
			continue
		}

		if info.DistroID != "ubuntu" {
			// Our business only concerns with ubuntu
			continue
		}

		distros = append(distros, info)
	}

	return distros

}

// basicDistroInfo returns basic information about a given distro, guid is assumed to correctly match the distro.
func basicDistroInfo(ctx context.Context, guid uuid.UUID, d gowsl.Distro) (BasicDistroInfo, error) {
	// State has to be read earlier, because reading files and running commands alters it (into 'Running' ofc).
	state, err := d.State()
	if err != nil {
		return BasicDistroInfo{}, fmt.Errorf("failed to get state for %s: %v", d.Name(), err)
	}

	osInfo, err := readOsRelease(filepath.Join(`\\wsl.localhost\`, d.Name()))
	if err != nil {
		return BasicDistroInfo{}, fmt.Errorf("failed to read os-release for %s: %v", d.Name(), err)
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
		Hostname:  strings.TrimRight(string(hostname), "\n"),
		State:     state,
	}, nil
}

type osReleaseInfo struct {
	//nolint:revive
	// ini mapper is strict with naming, so we cannot rename Id -> ID as the linter suggests
	Id, VersionId, PrettyName string
}

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
			return osReleaseInfo{}, fmt.Errorf("could not read the os-release file from distro %s: %v", root, errors.Join(errUsrLib, err))
		}
	}

	if err := ini.MapToWithMapper(&marshaller, ini.SnackCase, out); err != nil {
		return osReleaseInfo{}, fmt.Errorf("could not parse %s: %v", root, err)
	}

	return marshaller, nil
}
