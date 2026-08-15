package keeneticroute

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// lanInterfaceCandidates are tried in this order -- Keenetic's LAN/Home
// network lives on one of these. Never add a WAN-side name here
// (GigabitEthernet*, Vlan*, UsbLte*, MwsMobile*): on a 4G/USB-modem WAN
// those can carry a private-looking CGNAT address that isn't the LAN.
var lanInterfaceCandidates = []string{"Bridge0", "Home", "br0"}

// IsPrivateIPv4 reports whether ip is a plausible LAN address: four
// in-range numeric octets (rejecting empty strings, garbage, and a
// CIDR-suffixed value like "192.168.1.1/24" -- the suffix fails to
// parse as its own octet) within RFC1918 space.
func IsPrivateIPv4(ip string) bool {
	octets := strings.Split(ip, ".")
	if len(octets) != 4 {
		return false
	}
	vals := make([]int, 4)
	for i, o := range octets {
		n, err := strconv.Atoi(o)
		if err != nil || n < 0 || n > 255 {
			return false
		}
		vals[i] = n
	}
	switch {
	case vals[0] == 10:
		return true
	case vals[0] == 192 && vals[1] == 168:
		return true
	case vals[0] == 172 && vals[1] >= 16 && vals[1] <= 31:
		return true
	default:
		return false
	}
}

type lanIPReader func(run Runner, iface string) (string, error)

// Tried in this order for each candidate interface before moving to the
// next interface, matching the shell project's detect_router_lan_ip.
var lanIPReaders = []lanIPReader{
	lanIPFromRunningConfig,
	lanIPFromShowInterface,
	lanIPFromKernelInterface,
}

// DetectLANIP finds the router's own LAN-facing IPv4 address -- the
// address the Proxy0 interface's upstream must point at. KeeneticOS
// (ndm) and the Entware userland where the SOCKS5 server actually
// listens are not the same thing as far as ndm's own proxy client is
// concerned, so 127.0.0.1 does not work there even though it's the same
// physical box; this must be a real routable LAN address.
//
// On success the address is cached to cacheFile so a later call can
// fall back to it if live detection fails (a temporary ndmc hiccup
// should not be treated the same as "no LAN found"). Detection never
// invents a loopback address -- if nothing is found and there's no
// usable cache, callers must leave any existing upstream untouched
// rather than guess.
func DetectLANIP(run Runner, cacheFile string) (string, error) {
	for _, iface := range lanInterfaceCandidates {
		for _, read := range lanIPReaders {
			ip, err := read(run, iface)
			if err == nil && IsPrivateIPv4(ip) {
				writeLANIPCache(cacheFile, ip)
				return ip, nil
			}
		}
	}

	if ip, err := readLANIPCache(cacheFile); err == nil && IsPrivateIPv4(ip) {
		return ip, nil
	}

	return "", fmt.Errorf("could not detect router LAN IP on any of %v, and no usable cached value at %s", lanInterfaceCandidates, cacheFile)
}

// lanIPFromRunningConfig parses `show running-config`, looking for the
// first "ip address <ip> <mask>" line inside the named interface's
// block. This is the primary reader: unlike `show interface`, ndmc's
// running-config output never glues a CIDR suffix onto the address.
func lanIPFromRunningConfig(run Runner, iface string) (string, error) {
	out, err := run("show running-config")
	if err != nil {
		return "", err
	}
	inBlock := false
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "interface" {
			inBlock = len(fields) > 1 && fields[1] == iface
			continue
		}
		if inBlock && len(fields) >= 3 && fields[0] == "ip" && fields[1] == "address" {
			return fields[2], nil
		}
	}
	return "", fmt.Errorf("no ip address found for interface %s in running-config", iface)
}

// lanIPFromShowInterface parses `show interface <iface>`'s "address:"
// line. A secondary reader: some firmware versions print the address
// with a "/NN" suffix here, which IsPrivateIPv4 correctly rejects
// rather than trying to strip.
func lanIPFromShowInterface(run Runner, iface string) (string, error) {
	out, err := run(fmt.Sprintf("show interface %s", iface))
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "address:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("no address line found for interface %s", iface)
}

// lanIPFromKernelInterface reads the address directly from the kernel's
// own interface list (only meaningful for "br0", the literal Linux
// ifname among the candidates -- ndm's logical names like "Home" aren't
// necessarily visible this way). Ignores run: this doesn't go through
// ndmc at all.
func lanIPFromKernelInterface(_ Runner, iface string) (string, error) {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return "", err
	}
	addrs, err := ifi.Addrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address on interface %s", iface)
}

func writeLANIPCache(path, ip string) {
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte(ip+"\n"), 0o600)
}

func readLANIPCache(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("no cache file configured")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
