package daemon

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
)

// getWslIP returns the loopback address if the networking mode is mirrored or iterates over the network adapters to find the IP address of the WSL one.
func getWslIP(ctx context.Context, i hostIpconfig, w wslSystemDistro) (net.IP, error) {
	mode, err := networkingMode(ctx, w)
	if err != nil {
		log.Warningf(ctx, "could not determine if WSL network is mirrored (assuming NAT): %v", err)
		mode = "nat"
	}

	switch mode {
	case "mirrored":
		return net.IPv4(127, 0, 0, 1), nil
	case "nat":
		return findWslAdapterIP(i)
	default:
		return nil, fmt.Errorf("unknown networking mode: %s", mode)
	}
}

func findWslAdapterIP(i hostIpconfig) (net.IP, error) {
	const targetDesc = "Hyper-V Virtual Ethernet Adapter"
	const vEthernetName = "vEthernet (WSL"

	head, err := i.getAdaptersAddresses()
	if err != nil {
		return nil, err
	}

	// Filter the adapters by description.
	var candidates []ipAdapterAddresses
	for node := head; node != nil; node = node.Next() {
		desc := node.Description()
		// The adapter description could be "Hyper-V Virtual Ethernet Adapter #No"
		if !strings.Contains(desc, targetDesc) {
			continue
		}

		candidates = append(candidates, node)
	}

	if len(candidates) == 1 {
		return candidates[0].IP(), nil
	}

	// Desambiguates the adapters by friendly name.
	for _, node := range candidates {
		if !strings.Contains(node.FriendlyName(), vEthernetName) {
			continue
		}

		return node.IP(), nil
	}

	return nil, fmt.Errorf("could not find WSL adapter")
}

// networkingMode detects whether the WSL network is mirrored or not.
func networkingMode(ctx context.Context, wsl wslSystemDistro) (string, error) {
	// It does so by launching the system distribution.
	cmd := wsl.Command(ctx, "wslinfo", "--networking-mode", "-n")

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := bytes.TrimSpace(stdout.Bytes())
	if err != nil {
		return "", fmt.Errorf("%s: error: %v.\n    Stdout: %s\n    Stderr: %s", cmd.Path, err, out, stderr.String())
	}

	return strings.Split(string(out), "\n")[0], nil
}

type wslSystemDistro interface {
	Command(ctx context.Context, name string, arg ...string) *exec.Cmd
}

type hostIpconfig interface {
	getAdaptersAddresses() (ipAdapterAddresses, error)
}

type ipAdapterAddresses interface {
	Next() ipAdapterAddresses
	Description() string
	IP() net.IP
	FriendlyName() string
}
