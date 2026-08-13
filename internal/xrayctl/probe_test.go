package xrayctl

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// fakeSOCKS5Server is a minimal SOCKS5 CONNECT-only server for testing
// Probe/dialSOCKS5 without a real xray binary: it accepts the standard
// no-auth handshake, then dials the requested domain:port itself and
// splices the two connections together.
func fakeSOCKS5Server(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			go handleFakeSOCKS5Conn(conn)
		}
	}()

	return ln.Addr().String()
}

func handleFakeSOCKS5Conn(conn net.Conn) {
	defer conn.Close()

	greeting := make([]byte, 2)
	if _, err := io.ReadFull(conn, greeting); err != nil {
		return
	}
	nMethods := int(greeting[1])
	if _, err := io.CopyN(io.Discard, conn, int64(nMethods)); err != nil {
		return
	}
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil { // no-auth selected
		return
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	var host string
	switch header[3] {
	case 0x03: // domain name
		lenByte := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenByte); err != nil {
			return
		}
		domain := make([]byte, lenByte[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return
		}
		host = string(domain)
	case 0x01: // IPv4
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	default:
		return
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(portBytes)

	target, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(int(port))))
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // general failure
		return
	}
	defer target.Close()

	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil { // success
		return
	}

	done := make(chan struct{}, 2)
	go func() { io.Copy(target, conn); done <- struct{}{} }()
	go func() { io.Copy(conn, target); done <- struct{}{} }()
	<-done
}

func TestProbe_Success(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	socksAddr := fakeSOCKS5Server(t)

	err := Probe(context.Background(), ProbeOptions{
		SOCKSAddr: socksAddr,
		URL:       backend.URL,
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
}

func TestProbe_UpstreamError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	socksAddr := fakeSOCKS5Server(t)

	err := Probe(context.Background(), ProbeOptions{
		SOCKSAddr: socksAddr,
		URL:       backend.URL,
		Timeout:   2 * time.Second,
	})
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestProbe_NoSOCKSServer(t *testing.T) {
	err := Probe(context.Background(), ProbeOptions{
		SOCKSAddr: "127.0.0.1:1", // nothing listens on tcpmux here
		URL:       "https://example.invalid/",
		Timeout:   500 * time.Millisecond,
	})
	if err == nil {
		t.Error("expected error when SOCKS5 proxy is unreachable")
	}
}
