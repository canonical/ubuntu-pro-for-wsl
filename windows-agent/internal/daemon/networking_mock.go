//go:build linux || (windows && gowslmock)

package daemon

import (
	"errors"
	"net"
)

var wslIPErr bool

func getWslIP() (net.IP, error) {
	if wslIPErr {
		return nil, errors.New("mock error")
	}

	return net.IP([]byte{127, 0, 0, 1}), nil
}
