package landscape

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape/distroinstall"
	"github.com/ubuntu/decorate"
	"github.com/ubuntu/gowsl"
)

// executor is in charge of executing commands received from the Landscape server.
type executor struct {
	serviceData
}

func (e executor) exec(ctx context.Context, command *landscapeapi.Command) (err error) {
	log.Infof(ctx, "Landcape: received command %s", commandString(command))
	err = func() error {
		switch cmd := command.GetCmd().(type) {
		case *landscapeapi.Command_AssignHost_:
			return e.assignHost(ctx, cmd.AssignHost)
		case *landscapeapi.Command_Start_:
			return e.start(ctx, cmd.Start)
		case *landscapeapi.Command_Stop_:
			return e.stop(ctx, cmd.Stop)
		case *landscapeapi.Command_Install_:
			return e.install(ctx, cmd.Install)
		case *landscapeapi.Command_Uninstall_:
			return e.uninstall(ctx, cmd.Uninstall)
		case *landscapeapi.Command_SetDefault_:
			return e.setDefault(ctx, cmd.SetDefault)
		case *landscapeapi.Command_ShutdownHost_:
			return e.shutdownHost(ctx, cmd.ShutdownHost)
		default:
			return fmt.Errorf("unknown command type %T: %v", command.GetCmd(), command.GetCmd())
		}
	}()

	if err != nil {
		return fmt.Errorf("could not execute command %s: %v", commandString(command), err)
	}
	log.Infof(ctx, "Landcape: completed command %s", commandString(command))

	return nil
}

func commandString(command *landscapeapi.Command) string {
	switch cmd := command.GetCmd().(type) {
	case *landscapeapi.Command_AssignHost_:
		return fmt.Sprintf("Assign host (uid: %q)", cmd.AssignHost.GetUid())
	case *landscapeapi.Command_Start_:
		return fmt.Sprintf("Start (id: %q)", cmd.Start.GetId())
	case *landscapeapi.Command_Stop_:
		return fmt.Sprintf("Stop (id: %q)", cmd.Stop.GetId())
	case *landscapeapi.Command_Install_:
		return fmt.Sprintf("Install (id: %q)", cmd.Install.GetId())
	case *landscapeapi.Command_Uninstall_:
		return fmt.Sprintf("Uninstall (id: %q)", cmd.Uninstall.GetId())
	case *landscapeapi.Command_SetDefault_:
		return fmt.Sprintf("SetDefault (id: %q)", cmd.SetDefault.GetId())
	case *landscapeapi.Command_ShutdownHost_:
		return "ShutdownHost"
	default:
		return "Unknown"
	}
}

func (e executor) assignHost(ctx context.Context, cmd *landscapeapi.Command_AssignHost) error {
	conf := e.config()

	if uid, err := conf.LandscapeAgentUID(); err != nil {
		log.Warningf(ctx, "Possibly overriding current landscape client UID: could not read current Landscape UID: %v", err)
	} else if uid != "" {
		log.Warning(ctx, "Overriding current landscape client UID")
	}

	uid := cmd.GetUid()
	if uid == "" {
		return errors.New("UID is empty")
	}

	if err := conf.SetLandscapeAgentUID(uid); err != nil {
		return err
	}

	landscapeConf, _, err := conf.LandscapeClientConfig()
	if err != nil {
		return err
	}

	distributeConfig(ctx, e.database(), landscapeConf, uid)

	return nil
}

//nolint:unparam // Unused context so that all commands have the same signature.
func (e executor) start(ctx context.Context, cmd *landscapeapi.Command_Start) (err error) {
	d, ok := e.database().Get(cmd.GetId())
	if !ok {
		return fmt.Errorf("distro %q not in database", cmd.GetId())
	}

	return d.LockAwake()
}

//nolint:unparam // Unused context so that all commands have the same signature.
func (e executor) stop(ctx context.Context, cmd *landscapeapi.Command_Stop) (err error) {
	d, ok := e.database().Get(cmd.GetId())
	if !ok {
		return fmt.Errorf("distro %q not in database", cmd.GetId())
	}

	return d.ReleaseAwake()
}

func (e executor) install(ctx context.Context, cmd *landscapeapi.Command_Install) (err error) {
	log.Debugf(ctx, "Landscape: received command Install. Target: %s", cmd.GetId())

	if cmd.GetId() == "" {
		return errors.New("empty distro name")
	}

	distro := gowsl.NewDistro(ctx, cmd.GetId())
	if registered, err := distro.IsRegistered(); err != nil {
		return err
	} else if registered {
		return errors.New("already installed")
	}

	if err := e.cloudInit().WriteDistroData(cmd.GetId(), cmd.GetCloudinit()); err != nil {
		return fmt.Errorf("skipped installation: %v", err)
	}

	defer func() {
		if err == nil {
			return
		}
		// Avoid error states by cleaning up on error
		err := distro.Uninstall(ctx)
		if err != nil {
			log.Warningf(ctx, "Landscape Install: failed to clean up %q after failed Install: %v", distro.Name(), err)
		}
	}()

	if rootfs := cmd.GetRootfsURL(); rootfs != "" {
		u, err := url.Parse(rootfs)
		if err != nil {
			return err
		}

		id := distro.Name()
		reserved := regexp.MustCompile(`Ubuntu-[0-9]{2}\.[0-9]{2}`)
		if id == "Ubuntu" || id == "Ubuntu-Preview" || reserved.Match([]byte(id)) {
			return fmt.Errorf("target distro ID %s is reserved for installation from MS Store", id)
		}

		if err = installFromURL(ctx, e.homeDir(), e.downloadDir(), distro, u); err != nil {
			return err
		}
	} else {
		if err = installFromMicrosoftStore(ctx, distro); err != nil {
			return err
		}
	}

	if cmd.GetCloudinit() != "" {
		return nil
	}

	// TODO: The rest of this function will need to be rethought once cloud-init support exists.
	windowsUser, err := user.Current()
	if err != nil {
		return err
	}

	userName := windowsUser.Username
	if !distroinstall.UsernameIsValid(userName) {
		userName = "ubuntu"
	}

	uid, err := distroinstall.CreateUser(ctx, distro, userName, windowsUser.Name)
	if err != nil {
		return err
	}

	if err := distro.DefaultUID(uid); err != nil {
		return fmt.Errorf("could not set user as default: %v", err)
	}

	return nil
}

func (e executor) uninstall(ctx context.Context, cmd *landscapeapi.Command_Uninstall) (err error) {
	d, ok := e.database().Get(cmd.GetId())
	if !ok {
		return errors.New("distro not in database")
	}

	if err := d.Uninstall(ctx); err != nil {
		return err
	}

	if err := e.cloudInit().RemoveDistroData(d.Name()); err != nil {
		log.Warningf(ctx, "Landscape uninstall: distro %q: %v", d.Name(), err)
	}

	return nil
}

func (e executor) setDefault(ctx context.Context, cmd *landscapeapi.Command_SetDefault) error {
	d := gowsl.NewDistro(ctx, cmd.GetId())
	return d.SetAsDefault()
}

//nolint:unparam // cmd is not used, but kep here for consistency with other commands.
func (e executor) shutdownHost(ctx context.Context, cmd *landscapeapi.Command_ShutdownHost) error {
	return gowsl.Shutdown(ctx)
}

func installFromMicrosoftStore(ctx context.Context, distro gowsl.Distro) (err error) {
	defer decorate.OnError(&err, "can't install from Microsoft Store")

	if err := gowsl.Install(ctx, distro.Name()); err != nil {
		return err
	}

	if err := distroinstall.InstallFromExecutable(ctx, distro); err != nil {
		return err
	}

	return nil
}

func installFromURL(ctx context.Context, homeDir string, downloadDir string, distro gowsl.Distro, rootfsURL *url.URL) (err error) {
	defer decorate.OnError(&err, "can't install from URL: %q", rootfsURL)

	tmpDir := filepath.Join(downloadDir, distro.Name())
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return err
	}
	// Remove tarball once installed
	defer os.RemoveAll(tmpDir)

	tarball := filepath.Join(tmpDir, distro.Name()+".tar.gz")

	f, err := os.Create(tarball)
	if err != nil {
		return err
	}
	defer f.Close()

	err = download(ctx, f, rootfsURL)
	if err != nil {
		return err
	}

	// Create the directory that will contain the vhdx
	vhdxDir := filepath.Join(homeDir, "WSL", distro.Name())
	if err := os.MkdirAll(vhdxDir, 0700); err != nil {
		return err
	}

	if _, err := gowsl.Import(ctx, distro.Name(), tarball, vhdxDir); err != nil {
		rmErr := os.RemoveAll(vhdxDir)
		if rmErr != nil {
			log.Warningf(ctx, "could not cleanup install directory: %v", rmErr)
		}
		return err
	}
	return nil
}

// download downloads the rootfs from the given URL and writes it to the given writer while verifying its checksum.
// The checksum is read from the SHA256SUMS file found alongside the rootfs URL, as done in cloud-images.ubuntu.com.
func download(ctx context.Context, f io.Writer, u *url.URL) (err error) {
	defer decorate.OnError(&err, "could not download %q", u)

	checksum, err := wantRootfsChecksum(u)
	if err != nil {
		return err
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http request failed with code %d", resp.StatusCode)
	}

	// Verify checksum and write file to disk
	r := io.TeeReader(resp.Body, f)
	if checksum != "" {
		match, err := checksumMatches(ctx, r, checksum)
		if err != nil {
			return err
		}
		if !match {
			return fmt.Errorf("checksum %s for %s does not match", checksum, u)
		}
	} else {
		if _, err := io.Copy(io.Discard, r); err != nil {
			return err
		}
	}

	return nil
}

// wantRootfsChecksum fetches the checksum from the SHA256SUMS file found alongside the rootfs URL matching the rootfs file name.
//
// The SHA256SUMS file is expected to contain multiple lines of the format:
//
// SHA256 *filename
//
// For example:
//
// 03c7f7c75fb450c7dd576a0da20986e62e0d72bd2ccee4c01296bab9f415c7ab *jammy-server-cloudimg-amd64-azure.vhd.tar.gz
// 0dc4d78f08e871ce6325e027e1b8421fd1cde1e76158644e35343a36d8f67bf4 *jammy-server-cloudimg-amd64-root.tar.xz
// 103ee8b5693bdb7c23a378453c624d8605445eb07e2e550d3fad831da865f5ea *jammy-server-cloudimg-riscv64.release.20240514.20240601.image_changelog.json
// 1eaa1df5794122e3419c963d88f043121c164936b9b828adac650c9f5e22c3e6 *jammy-server-cloudimg-amd64.img
// 1fcd2edf4fda78e0a6f3bc0c3684286c29371e4dd7863a59b39d2cfcff79b5e1 *jammy-server-cloudimg-amd64-root.manifest
// 1fcd2edf4fda78e0a6f3bc0c3684286c29371e4dd7863a59b39d2cfcff79b5e1 *jammy-server-cloudimg-amd64.squashfs.manifest
// 2646292d657f4c9ef5dfce804a5a1e66d8c1324c74147b8bc9b1bf154d7feaf8 *jammy-server-cloudimg-arm64-root.tar.xz
//
// ...
func wantRootfsChecksum(u *url.URL) (string, error) {
	imageName := filepath.Base(u.Path)
	shasRelativeURL, err := url.Parse("../SHA256SUMS")
	if err != nil {
		return "", fmt.Errorf("could not assemble SHA256SUMS location: %v", err)
	}
	checksumsURL := u.ResolveReference(shasRelativeURL)

	resp, err := http.Get(checksumsURL.String())
	if err != nil {
		return "", fmt.Errorf("could not download checksums file <%s>: %v", checksumsURL, err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}

		if strings.TrimPrefix(fields[1], "*") == imageName && len(fields[0]) > 0 {
			return fields[0], nil
		}
	}

	return "", fmt.Errorf("could not find checksum for %s in %s", imageName, checksumsURL)
}

func checksumMatches(ctx context.Context, reader io.Reader, wantChecksum string) (match bool, err error) {
	defer decorate.OnError(&err, "error checking checksum for: %q", reader)

	// Checksum of the rootfs
	h := sha256.New()
	if _, err := io.Copy(h, reader); err != nil {
		return false, err
	}
	gotChecksum := fmt.Sprintf("%x", h.Sum(nil))
	log.Debugf(ctx, "Want checksum: %s, Got checksum: %s", wantChecksum, gotChecksum)

	// Compare checksums
	return wantChecksum == gotChecksum, nil
}
