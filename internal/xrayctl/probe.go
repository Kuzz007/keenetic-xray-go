package xrayctl

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// ProbeOptions configures a single health-check attempt.
type ProbeOptions struct {
	SOCKSAddr string // e.g. "127.0.0.1:1080"
	URL       string // health-check target, e.g. https://www.gstatic.com/generate_204
	Timeout   time.Duration
}

// Probe performs one HTTP GET to URL through the SOCKS5 proxy at
// SOCKSAddr and reports whether it got back a non-error response. This is
// the only supported health check: a bare ICMP ping or a raw TCP connect
// to the VLESS server wouldn't prove the VLESS/TLS/auth path actually
// works, and ICMP is often filtered independently of tunnel health.
func Probe(ctx context.Context, opts ProbeOptions) error {
	client := &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, portStr, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, fmt.Errorf("splitting dial address %q: %w", addr, err)
				}
				port, err := strconv.Atoi(portStr)
				if err != nil {
					return nil, fmt.Errorf("invalid port in dial address %q: %w", addr, err)
				}
				return dialSOCKS5(ctx, opts.SOCKSAddr, host, port)
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.URL, nil)
	if err != nil {
		return fmt.Errorf("building probe request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("probe request failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("probe returned status %s", resp.Status)
	}
	return nil
}

// dialSOCKS5 performs a minimal SOCKS5 CONNECT handshake (RFC 1928) with
// no authentication, then returns a connection to targetHost:targetPort
// tunneled through the SOCKS5 proxy at proxyAddr. This project has no
// third-party dependencies, so it implements just enough of SOCKS5 to
// reach Xray's local SOCKS5 inbound -- not a general-purpose client.
func dialSOCKS5(ctx context.Context, proxyAddr, targetHost string, targetPort int) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("connecting to SOCKS5 proxy %s: %w", proxyAddr, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil { // VER=5, 1 method, no-auth
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 greeting: %w", err)
	}
	greetReply := make([]byte, 2)
	if _, err := io.ReadFull(conn, greetReply); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 greeting reply: %w", err)
	}
	if greetReply[0] != 0x05 || greetReply[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 proxy rejected no-auth (method %d)", greetReply[1])
	}

	if len(targetHost) > 255 {
		conn.Close()
		return nil, fmt.Errorf("target host name too long for SOCKS5: %q", targetHost)
	}
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(targetHost))}
	req = append(req, targetHost...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(targetPort))
	req = append(req, portBytes...)
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect request: %w", err)
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect reply: %w", err)
	}
	if header[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect failed, reply code %d", header[1])
	}
	var addrLen int
	switch header[3] {
	case 0x01: // IPv4
		addrLen = 4
	case 0x03: // domain name, next byte is its length
		lenByte := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenByte); err != nil {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5 connect reply domain length: %w", err)
		}
		addrLen = int(lenByte[0])
	case 0x04: // IPv6
		addrLen = 16
	default:
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect reply: unknown address type %d", header[3])
	}
	if _, err := io.CopyN(io.Discard, conn, int64(addrLen+2)); err != nil { // bound address + port
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect reply address: %w", err)
	}

	_ = conn.SetDeadline(time.Time{}) // clear the handshake deadline; the caller owns timeouts from here
	return conn, nil
}
