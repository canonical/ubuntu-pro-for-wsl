package system

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"gopkg.in/ini.v1"
)

const (
	landscapeConfigPath = "/etc/landscape/client.conf"
)

// LandscapeEnable registers the current distro to Landscape with the specified config.
func (s *System) LandscapeEnable(ctx context.Context, landscapeConfig string, hostagentUID string) (err error) {
	// Decorating here to avoid stuttering the URL (url package prints it as well)
	defer decorate.OnError(&err, "could not register distro to Landscape")

	if landscapeConfig, err = modifyConfig(ctx, s, landscapeConfig, hostagentUID); err != nil {
		return err
	}

	if err := s.writeConfig(landscapeConfig); err != nil {
		return err
	}

	cmd := s.backend.LandscapeConfigExecutable(ctx, "--config", landscapeConfigPath, "--silent")
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("could not enable Landscape: %v", err)
	}

	return nil
}

// LandscapeDisable unregisters the current distro from Landscape.
func (s *System) LandscapeDisable(ctx context.Context) (err error) {
	cmd := s.backend.LandscapeConfigExecutable(ctx, "--disable")
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("could not disable Landscape:%v", err)
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
func modifyConfig(ctx context.Context, s *System, landscapeConfig string, hostagentUID string) (string, error) {
	if landscapeConfig == "" {
		return "", nil
	}

	r := strings.NewReader(landscapeConfig)
	data, err := ini.Load(r)
	if err != nil {
		return "", fmt.Errorf("could not parse config: %v", err)
	}

	data.DeleteSection("host")

	distroName, err := s.WslDistroName(ctx)
	if err != nil {
		return "", err
	}
	if err := createKey(ctx, data, "client", "computer_title", distroName, true); err != nil {
		return "", err
	}

	if err := createKey(ctx, data, "client", "hostagent_uid", hostagentUID, true); err != nil {
		return "", err
	}

	if err := createKey(ctx, data, "client", "tags", "wsl", false); err != nil {
		return "", err
	}

	if err := overrideSSLCertificate(ctx, s, data); err != nil {
		return "", fmt.Errorf("could not override SSL certificate path: %v", err)
	}

	w := &bytes.Buffer{}
	if _, err := data.WriteTo(w); err != nil {
		return "", fmt.Errorf("could not write modified config: %v", err)
	}

	return w.String(), nil
}

// createKey tries to create a key with a particular value, optionally overriding an existing key.
func createKey(ctx context.Context, data *ini.File, section, key, value string, override bool) error {
	sec, err := data.GetSection(section)
	if err != nil {
		if sec, err = data.NewSection(section); err != nil {
			return fmt.Errorf("could not find nor create section %q: %v", section, err)
		}
	}

	if sec.HasKey(key) {
		if override {
			log.Infof(ctx, "Landscape config contains key %q. Its value will be overridden.", key)
			sec.DeleteKey(key)
		} else {
			log.Infof(ctx, "Landscape config contains key %q. Its value will not be overridden.", key)
			return nil
		}
	}

	if _, err := sec.NewKey(key, value); err != nil {
		return fmt.Errorf("could not create key %q: %v", key, err)
	}

	return nil
}

// overrideComputerTitle converts the ssl_public_key field in the Landscape config
// from a Windows path to a Linux path.
func overrideSSLCertificate(ctx context.Context, s *System, data *ini.File) error {
	const section = "client"
	const key = "ssl_public_key"

	sec, err := data.GetSection(section)
	if err != nil {
		// No certificate
		return nil
	}

	k, err := sec.GetKey(key)
	if err != nil {
		// No certificate
		return nil
	}

	pathWindows := k.String()

	cmd := s.backend.WslpathExecutable(ctx, "-ua", pathWindows)
	out, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("could not translate SSL certificate path %q to a WSL path: %v", pathWindows, err)
	}

	pathLinux := s.Path(strings.TrimSpace(string(out)))
	k.SetValue(pathLinux)

	return nil
}
