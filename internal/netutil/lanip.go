// Package netutil contains small networking helpers for corsproxy.
package netutil

import "net"

// LANIP returns the machine's primary outbound IP on the local network.
//
// It uses the UDP-dial trick: no packet is sent (UDP is connectionless), the
// kernel just resolves which local address it would route through. Returns ""
// when offline so callers can skip the Network line in the banner.
func LANIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return ""
	}
	return addr.IP.String()
}
