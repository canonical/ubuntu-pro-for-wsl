package system

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
func (s *System) LandscapeEnable(ctx context.Context, landscapeConfig string) (err error) {
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

// EnsureValidLandscapeConfig ensures that the Landscape configuration is valid and enabled.
func (s *System) EnsureValidLandscapeConfig(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not ensure valid Landscape configuration")

	s.syncWithCloudInit()
	landscapeConfig, err := os.ReadFile(s.Path(landscapeConfigPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Debug(ctx, "No Landscape configuration detected: nothing to do")
			return nil
		}
		return err
	}

	return s.fixAndEnableLandscapeFromConfig(ctx, string(landscapeConfig), false)
}

func (s *System) syncWithCloudInit() {
	log.Debug(context.Background(), "Checking cloud-init status")
	// Wait for cloud-init to finish if systemd and its service is enabled.
	// Since plucky the wsl-setup package ships a small script created for that purpose.
	// As we need to support back to focal, we keep a fallback if the script is not readable.
	script := `
if [ -r /usr/lib/wsl/wait-for-cloud-init ]; then
  source /usr/lib/wsl/wait-for-cloud-init
  exit 0
fi
if status=$(LANG=C systemctl is-system-running 2>/dev/null) || [ "${status}" != "offline" ] && systemctl is-enabled --quiet cloud-init.service 2>/dev/null; then
  cloud-init status --wait > /dev/null 2>&1 || true
fi`
	cmd := exec.Command("bash", "-ec", script)
	_ = cmd.Run()
}

func (s *System) fixAndEnableLandscapeFromConfig(ctx context.Context, landscapeConfig string, enableUnconditionally bool) (err error) {
	// Decorating here to avoid stuttering the URL (url package prints it as well)
	defer decorate.OnError(&err, "could not register distro to Landscape")

	r := strings.NewReader(landscapeConfig)
	iniFile, err := ini.Load(r)
	if err != nil {
		return fmt.Errorf("could not parse config: %v", err)
	}

	modifiedLandscapeConfig, didChange, err := normalizeLandscapeConfig(ctx, s, iniFile)
	if err != nil {
		return err
	}

	// No change to do, do not rewrite config
	if !enableUnconditionally && !didChange {
		log.Debug(ctx, "Landscape configuration is already valid")
		return nil
	}

	if err := s.writeConfig(modifiedLandscapeConfig); err != nil {
		return err
	}

	// TODO: check foreground/background
	cmd := s.backend.LandscapeConfigExecutable(ctx, "--config", landscapeConfigPath, "--silent", "--register-if-needed")
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
func normalizeLandscapeConfig(ctx context.Context, s *System, iniFile *ini.File) (modifiedLandscapeConfig string, didChange bool, err error) {
	clientSection, err := iniFile.GetSection("client")
	if err != nil {
		return "", false, err
	}

	// Add or refresh computer title
	distroName, err := s.WslDistroName(ctx)
	if err != nil {
		return "", false, err
	}
	titleChanged := false
	oldComputerTitle, err := clientSection.GetKey("computer_title")
	if err != nil {
		if _, err = clientSection.NewKey("computer_title", distroName); err != nil {
			return "", false, err
		}
		titleChanged = true
	} else if oldComputerTitle.String() != distroName {
		oldComputerTitle.SetValue(distroName)
		titleChanged = true
	}

	// Refresh SSL certificate path if any
	certChanged, err := overrideSSLCertificate(ctx, s, clientSection)
	if err != nil {
		return "", false, fmt.Errorf("could not override SSL certificate path: %v", err)
	}

	// Return the modified config as a string.
	w := &bytes.Buffer{}
	if _, err := iniFile.WriteTo(w); err != nil {
		return "", false, fmt.Errorf("could not regenerate modified config: %v", err)
	}

	didChange = titleChanged || certChanged
	return w.String(), didChange, nil
}

// overrideComputerTitle converts the ssl_public_key field in the Landscape config
// from a Windows path to a Linux path. Returns true if the value was changed.
func overrideSSLCertificate(ctx context.Context, s *System, section *ini.Section) (bool, error) {
	const key = "ssl_public_key"

	k, err := section.GetKey(key)
	if err != nil {
		// No certificate: nothing to transform
		return false, nil
	}

	pathWindows := k.String()

	if len(pathWindows) == 0 {
		// Empty paths are translated by wslpath as the current working directory, which is not what we want.
		return false, nil
	}

	cmd := s.backend.WslpathExecutable(ctx, "-ua", pathWindows)
	out, err := runCommand(cmd)
	if err != nil {
		return false, fmt.Errorf("could not translate SSL certificate path %q to a WSL path: %v", pathWindows, err)
	}

	pathLinux := s.Path(strings.TrimSpace(string(out)))
	k.SetValue(pathLinux)

	return pathWindows != pathLinux, nil
}
