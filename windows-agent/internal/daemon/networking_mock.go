//go:build linux || (windows && gowslmock)

package daemon

import (
	"net"
)

func getWslIP() (net.IP, error) {
	return net.IP([]byte{127, 0, 0, 1}), nil
}
