package printer

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// ErrDisabled is returned when the caller tries to print but the printer
// integration is disabled in config.
var ErrDisabled = errors.New("printer is disabled")

// DispatchTCP opens a raw TCP connection to address (host:port), writes the
// receipt bytes, and closes. Suitable for the vast majority of network thermal
// printers (Epson TM-T, Star, generic ESC/POS-over-LAN units).
//
// The timeout governs both connect and write; default 5s if zero.
func DispatchTCP(address string, payload []byte, timeout time.Duration) error {
	if address == "" {
		return errors.New("printer address is empty")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return fmt.Errorf("dial printer %s: %w", address, err)
	}
	defer conn.Close()
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}
	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("write to printer: %w", err)
	}
	return nil
}
