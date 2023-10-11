package system

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"gopkg.in/ini.v1"
)

const (
	landscapeConfigPath = "/etc/landscape/client.conf"
)

// LandscapeEnable registers the current distro to Landscape with the specified config.
func (s *System) LandscapeEnable(ctx context.Context, landscapeConfig string) (err error) {
	// Decorating here to avoid stuttering the URL (url package prints it as well)
	defer decorate.OnError(&err, "could not register to landscape")

	if landscapeConfig, err = modifyConfig(ctx, s, landscapeConfig); err != nil {
		return err
	}

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

// modifyConfig overrides parameters in the configuration to adapt them to the current distro.
func modifyConfig(ctx context.Context, s *System, landscapeConfig string) (string, error) {
	if landscapeConfig == "" {
		return "", nil
	}

	r := strings.NewReader(landscapeConfig)
	data, err := ini.Load(r)
	if err != nil {
		return "", fmt.Errorf("could not parse config: %v", err)
	}

	if err := overrideComputerTitle(ctx, s, data); err != nil {
		return "", err
	}

	w := &bytes.Buffer{}
	if _, err := data.WriteTo(w); err != nil {
		return "", fmt.Errorf("could not write modified config: %v", err)
	}

	return w.String(), nil
}

// overrideComputerTitle overrides the computer_title field in the Landscape config.
func overrideComputerTitle(ctx context.Context, s *System, data *ini.File) error {
	const section = "client"
	const key = "computer_title"

	distroName, err := s.wslDistroName(ctx)
	if err != nil {
		return err
	}

	sec, err := data.GetSection(section)
	if err != nil {
		if sec, err = data.NewSection(section); err != nil {
			return fmt.Errorf("could not find nor create section %q: %v", section, err)
		}
	}

	if sec.HasKey(key) {
		log.Infof(ctx, "Landscape config contains key %q. Its value will be overridden with %s", key, distroName)
		sec.DeleteKey(key)
	}

	if _, err := sec.NewKey(key, distroName); err != nil {
		return fmt.Errorf("could not create %q key", key)
	}

	return nil
}
