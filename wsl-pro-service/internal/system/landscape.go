package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ubuntu/decorate"
)

const (
	landscapeConfigPath = "/etc/landscape/client.conf"
)

// LandscapeEnable registers the current distro to Landscape with the specified config.
func (s *System) LandscapeEnable(ctx context.Context, landscapeConfig string) (err error) {
	// Decorating here to avoid stuttering the URL (url package prints it as well)
	defer decorate.OnError(&err, "could not register to landscape")

	if err := s.writeConfig(landscapeConfig); err != nil {
		return err
	}

	exe, args := s.backend.LandscapeConfigExecutable("--config", landscapeConfigPath, "--silent")
	//nolint:gosec // In production code, these variables are hard-coded.
	if out, err := exec.CommandContext(ctx, exe, args...).Output(); err != nil {
		return fmt.Errorf("%s returned an error: %v\nOutput:%s", exe, err, string(out))
	}

	return nil
}

// LandscapeDisable unregisters the current distro from Landscape.
func (s *System) LandscapeDisable(ctx context.Context) (err error) {
	exe, args := s.backend.LandscapeConfigExecutable("--disable")

	//nolint:gosec // In production code, these variables are hard-coded (except for the URLs).
	if out, err := exec.CommandContext(ctx, exe, args...).Output(); err != nil {
		return fmt.Errorf("could not disable Landscape: %s returned an error: %v\nOutput:%s", exe, err, string(out))
	}

	return nil
}

func (s *System) writeConfig(landscapeConfig string) (err error) {
	defer decorate.OnError(&err, "could not write Landscape configuration")

	tmp := s.backend.Path(landscapeConfigPath + ".new")
	final := s.backend.Path(landscapeConfigPath)

	if err := os.MkdirAll(filepath.Dir(tmp), 0750); err != nil {
		return fmt.Errorf("could not create config directory: %v", err)
	}

	//nolint:gosec // Needs 0604 for the Landscape client to be able to read it
	if err = os.WriteFile(tmp, []byte(landscapeConfig), 0604); err != nil {
		return fmt.Errorf("could not write to file: %v", err)
	}

	if err := os.Rename(tmp, final); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}

	return nil
}
