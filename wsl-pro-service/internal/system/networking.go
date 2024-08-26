package system

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/ubuntu/decorate"
)

// WindowsHostAddress returns the IP that maps to Windows' localhost.
func (s *System) WindowsHostAddress(ctx context.Context) (ip net.IP, err error) {
	defer decorate.OnError(&err, "coud not find address mapping to the Windows host")

	mode, err := s.networkingMode(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not ascertain the network mode: %v", err)
	}

	if mode != "nat" {
		return net.IPv4(127, 0, 0, 1), nil
	}

	return s.defaultGateway()
}

func (s *System) networkingMode(ctx context.Context) (string, error) {
	cmd := s.backend.WslinfoExecutable(ctx, "--networking-mode", "-n")

	out, err := runCommand(cmd)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// defaultGateway returns the default gateway of the machine.
func (s *System) defaultGateway() (ip net.IP, err error) {
	/*
		Implemented by parsing /proc/net/route. We could use `ip route`, but that would mean
		calling a subprocess which is a pain to test. This is easier to mock.

		Showing first two lines of /proc/net/route:

		Iface   Destination     Gateway         Flags   RefCnt  Use     Metric  Mask            MTU     Window  IRTT
		eth0    00000000        012019AC        0003    0       0       0       00000000        0       0       0

		The default gateway is in the first row of the table. It is encoded as a little-endian hex.
		In this example the default gateway is 012019AC:
		Byte 0: 01 -> 1
		Byte 1: 20 -> 32
		Byte 2: 19 -> 25
		Byte 3: AC -> 172

		Flipped due to little-endianness:
		172.25.32.1
	*/

	const fileName = "/proc/net/route"
	defer decorate.OnError(&err, "could not parse %s", fileName)

	f, err := os.Open(s.Path(fileName))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// Ignore header
	if ok := scanner.Scan(); !ok {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("line 1: could not scan: %v", err)
		}
		return nil, fmt.Errorf("line 1: file too short")
	}

	// Parse first row
	if ok := scanner.Scan(); !ok {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("line 2: could not scan: %v", err)
		}
		return nil, fmt.Errorf("line 2: file too short")
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) < 3 {
		return nil, fmt.Errorf("line 2: too few fields (found %d, needs least 3)", len(fields))
	}

	// Convert hex string to a byte array
	gatewayRaw, err := strconv.ParseUint(fields[2], 0x10, 32)
	if err != nil {
		return nil, fmt.Errorf("line 2: field 3: could not parse address %q as a 32-bit hex", fields[2])
	}

	b := make([]byte, 4)
	//nolint:gosec // Value is guaranteed by strconv.ParseUint to fit in uint32 (due the bitSize argument)
	binary.LittleEndian.PutUint32(b, uint32(gatewayRaw))

	return net.IP(b), nil
}
