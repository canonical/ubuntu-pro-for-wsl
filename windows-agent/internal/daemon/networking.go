package daemon

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
)

// getWslIP returns the loopback address if the networking mode is mirrored or iterates over the network adapters to find the IP address of the WSL one.
func getWslIP(i hostIpconfig, w wslSystemDistro) (net.IP, error) {
	isMirrored, err := networkIsMirrored(w)
	if err != nil {
		log.Warningf(context.Background(), "could not determine if WSL network is mirrored (assuming NAT): %v", err)
		isMirrored = false
	}
	if isMirrored {
		return net.IPv4(127, 0, 0, 1), nil
	}

	const targetDesc = "Hyper-V Virtual Ethernet Adapter"
	const vEthernetName = "vEthernet (WSL"

	head, err := i.getAdaptersAddresses() // blow on nil?
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

// networkIsMirrored detects whether the WSL network is mirrored or not.
func networkIsMirrored(wsl wslSystemDistro) (bool, error) {
	// It does so by launching the system distribution.
	cmd := wsl.Command(context.Background(), "wslinfo", "--networking-mode", "-n")

	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("could not get networking mode: %w\n%s", err, string(out))
	}
	return string(out) == "mirrored", nil
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
