package landscape

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape/distroinstall"
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
	if cmd.GetCloudinit() != "" {
		return fmt.Errorf("Cloud Init support is not yet available")
	}

	distro := gowsl.NewDistro(ctx, cmd.GetId())
	if registered, err := distro.IsRegistered(); err != nil {
		return err
	} else if registered {
		return errors.New("already installed")
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

	if rootfs := cmd.GetRootfs(); rootfs != nil {
		if err = installFromURL(ctx, e.homeDir(), distro, rootfs); err != nil {
			return err
		}
	} else {
		if err = installFromMicrosoftStore(ctx, distro); err != nil {
			return err
		}
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
		return fmt.Errorf("distro %q not in database", cmd.GetId())
	}

	return d.Uninstall(ctx)
}

func (e executor) setDefault(ctx context.Context, cmd *landscapeapi.Command_SetDefault) error {
	d := gowsl.NewDistro(ctx, cmd.GetId())
	return d.SetAsDefault()
}

//nolint:unparam // cmd is not used, but kep here for consistency with other commands.
func (e executor) shutdownHost(ctx context.Context, cmd *landscapeapi.Command_ShutdownHost) error {
	return gowsl.Shutdown(ctx)
}

func installFromMicrosoftStore(ctx context.Context, distro gowsl.Distro) error {
	if err := gowsl.Install(ctx, distro.Name()); err != nil {
		return err
	}

	if err := distroinstall.InstallFromExecutable(ctx, distro); err != nil {
		return err
	}

	return nil
}

func installFromURL(ctx context.Context, homeDir string, distro gowsl.Distro, rootfs *landscapeapi.Command_Install_Rootfs) (err error) {
	// Since we are going to remove the tarball soon, I'd say it's fine to write it to a subfolder of the current working directory,
	// as it will be somewhere inside the Windows Apps private folders:
	// %USERPROFILE%\AppData\Local\Packages\CanonicalGroupLimited.Ubuntu_79rhkp1fndgsc\LocalState\
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	tmpDir := filepath.Join(cwd, distro.Name())
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

	if err := download(f, rootfs.GetUrl()); err != nil {
		return err
	}

	if err := checksumMatches(tarball, rootfs.GetSha256Sum()); err != nil {
		return err
	}

	// Create the directory that will contain the vhdx
	vhdxDir := filepath.Join(homeDir, "WSL", distro.Name())
	if err := os.MkdirAll(vhdxDir, 0700); err != nil {
		return err
	}

	if _, err := gowsl.Import(ctx, distro.Name(), tarball, vhdxDir); err != nil {
		_ = os.RemoveAll(vhdxDir)
		return err
	}
	return nil
}

func download(w io.Writer, url string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not download %q: %v", url, err)
		}
	}()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http request failed with code %d", resp.StatusCode)
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		return err
	}

	return nil
}

func checksumMatches(path, wantChecksum string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("error checking checksum for: %q: %v", path, err)
		}
	}()

	if wantChecksum == "" {
		return nil
	}

	// Checksum of the rootfs
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	gotChecksum := fmt.Sprintf("%x", h.Sum(nil))

	out, err := os.ReadFile(path)

	// Compare checksums
	if gotChecksum != wantChecksum {
		return fmt.Errorf("checksums do not match (want: %s got: %s) (from %s)", wantChecksum, gotChecksum, string(out))
	}

	return nil
}
