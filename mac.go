package bluetooth

import (
	"errors"
	"fmt"
)

// MAC represents a MAC address, in little endian format.
type MAC [6]byte

var errInvalidMAC = errors.New("bluetooth: failed to parse MAC address")

// ParseMAC parses the given MAC address, which must be in 11:22:33:AA:BB:CC
// format. If it cannot be parsed, an error is returned.
func ParseMAC(s string) (MAC, error) {
	var mac MAC
	_, err := fmt.Sscanf(s, "%02X:%02X:%02X:%02X:%02X:%02X", &mac[0], &mac[1], &mac[2], &mac[3], &mac[4], &mac[5])
	if err != nil {
		return [6]byte{}, err
	}
	return mac, nil
}

// String returns a human-readable version of this MAC address, such as
// 11:22:33:AA:BB:CC.
func (mac MAC) String() string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}
