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
	return s.fixAndEnableLandscapeFromConfig(ctx, landscapeConfig, true)
}

// LandscapeDisable unregisters the current distro from Landscape.
func (s *System) LandscapeDisable(ctx context.Context) (err error) {
	cmd := s.backend.LandscapeConfigExecutable(ctx, "--disable")
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("could not disable Landscape:%v", err)
	}

	return nil
}

func (s *System) fixAndEnableLandscapeFromConfig(ctx context.Context, landscapeConfig string, enableUnconditionally bool) (err error) {
	// Decorating here to avoid stuttering the URL (url package prints it as well)
	defer decorate.OnError(&err, "could not register distro to Landscape")

	r := strings.NewReader(landscapeConfig)
	iniFile, err := ini.Load(r)
	if err != nil {
		return fmt.Errorf("could not parse config: %v", err)
	}

	modifiedLandscapeConfig, err := normalizeLandscapeConfig(ctx, s, iniFile)
	if err != nil {
		return err
	}

	// No change to do, do not rewrite config
	if !enableUnconditionally && modifiedLandscapeConfig == landscapeConfig {
		log.Debug(ctx, "Landscape configuration is already valid")
		return nil
	}

	if err := s.writeConfig(modifiedLandscapeConfig); err != nil {
		return err
	}

	// TODO: check foreground/background
	cmd := s.backend.LandscapeConfigExecutable(ctx, "--config", landscapeConfigPath, "--silent")
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("could not enable Landscape: %v", err)
	}

	return nil
}

func (s *System) writeConfig(landscapeConfig string) (err error) {
	defer decorate.OnError(&err, "could not write Landscape configuration")

	userID, err := s.currentUser()
	if err != nil {
		return err
	}

	groupID, err := s.groupToGUID("landscape")
	if err != nil {
		return err
	}

	tmp := s.backend.Path(landscapeConfigPath + ".new")
	final := s.backend.Path(landscapeConfigPath)

	if err := os.MkdirAll(filepath.Dir(tmp), 0750); err != nil {
		return fmt.Errorf("could not create config directory: %v", err)
	}

	//nolint:gosec // Needs 0640 for the landscape client to be able to read it.
	if err := os.WriteFile(tmp, []byte(landscapeConfig), 0640); err != nil {
		return fmt.Errorf("could not write to file: %v", err)
	}

	if err := os.Chown(tmp, userID, groupID); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("could not change ownership to landscape group: %v", err)
	}

	if err := os.Rename(tmp, final); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}

	return nil
}

// normalizeLandscapeConfig ensures that the landscape config has the expected computer_title and SSL certificate path
// transformed in a Linux path.
func normalizeLandscapeConfig(ctx context.Context, s *System, iniFile *ini.File) (modifiedLandscapeConfig string, err error) {
	clientSection, err := iniFile.GetSection("client")
	if err != nil {
		return "", err
	}

	// Add or refresh computer title
	distroName, err := s.WslDistroName(ctx)
	if err != nil {
		return "", err
	}
	oldComputerTitle, err := clientSection.GetKey("computer_title")
	if err != nil {
		if _, err = clientSection.NewKey("computer_title", distroName); err != nil {
			return "", err
		}
	} else if oldComputerTitle.String() != distroName {
		oldComputerTitle.SetValue(distroName)
	}

	// Refresh SSL certificate path if any
	if err := overrideSSLCertificate(ctx, s, clientSection); err != nil {
		return "", fmt.Errorf("could not override SSL certificate path: %v", err)
	}

	// Return the modified config as a string.
	w := &bytes.Buffer{}
	if _, err := iniFile.WriteTo(w); err != nil {
		return "", fmt.Errorf("could not regenerate modified config: %v", err)
	}

	return w.String(), nil
}

// overrideComputerTitle converts the ssl_public_key field in the Landscape config
// from a Windows path to a Linux path.
func overrideSSLCertificate(ctx context.Context, s *System, section *ini.Section) error {
	const key = "ssl_public_key"

	k, err := section.GetKey(key)
	if err != nil {
		// No certificate: nothing to transform
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
