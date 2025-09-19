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
	Name      string
	GUID      string
	VersionID string
	Hostname  string
	State     gowsl.State
}

// GetUnmanagedDistros returns a list of distros that are currently registered in WSL but not managed by this agent.
// That's an expensive operation that involves a light wake up of all instances discovered, information is a snapshot at the time of the call, we don't store it.
func (db *DistroDB) GetUnmanagedDistros() (distros []BasicDistroInfo, err error) {
	managed := db.GetAll()

	registryDistros, err := gowsl.RegisteredDistros(db.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get registered distros: %w", err)
	}

	for _, d := range registryDistros {
		// Acquiring the distro GUID from GoWSL is quite expensive the way it's implemented now, so we do it once and then pass it along.
		// TODO: Make this more elegant when d.GUID() becomes cheaoer.
		guid, err := d.GUID()
		if err != nil {
			log.Warningf(db.ctx, "failed to get GUID for distro %q: %v", d.Name(), err)
			continue
		}
		if slices.ContainsFunc(managed, func(m *distro.Distro) bool {
			// The m.GUID() here is not as expensive as d.GUID() above, because it's retrieved from a cache.
			return guid.String() == m.GUID()
		}) {
			// This is a managed distro, skip it.
			continue
		}

		info, err := basicDistroInfo(db.ctx, guid, d)
		if err != nil {
			log.Warningf(db.ctx, "failed to get GUID for distro %q: %v", d.Name(), err)
			continue
		}

		distros = append(distros, info)
	}

	return distros, nil

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
			return osReleaseInfo{}, fmt.Errorf("could not read %s: %v", root, errors.Join(errUsrLib, err))
		}
	}

	if err := ini.MapToWithMapper(&marshaller, ini.SnackCase, out); err != nil {
		return osReleaseInfo{}, fmt.Errorf("could not parse %s: %v", root, err)
	}

	return marshaller, nil
}
