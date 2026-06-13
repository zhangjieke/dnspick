package dnsbench

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"github.com/miekg/dns"

	"github.com/palemoky/dnspick/internal/i18n"
)

// DetectSystemDNS probes the system's configured default DNS servers (handed
// out by the ISP or router) and returns Servers ready to be benchmarked.
// Returns nil when detection is not possible (the feature degrades gracefully).
func DetectSystemDNS() []Server {
	var ips []string
	if runtime.GOOS == "windows" {
		ips = windowsDNS()
	} else {
		ips = systemDNSFromResolvConf("/etc/resolv.conf")
	}
	return buildSystemServers(ips)
}

// buildSystemServers deduplicates the IP list and converts it into Servers
// flagged with IsSystem.
func buildSystemServers(ips []string) []Server {
	seen := make(map[string]struct{})
	var servers []Server
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" || net.ParseIP(ip) == nil {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		servers = append(servers, Server{Address: ip, Protocol: UDP, IsSystem: true})
	}
	// Naming: a single server is unnumbered, multiple servers are numbered.
	for i := range servers {
		if len(servers) == 1 {
			servers[i].Name = i18n.L().SystemDNSName
		} else {
			servers[i].Name = fmt.Sprintf(i18n.L().SystemDNSNameN, i+1)
		}
	}
	return servers
}

// systemDNSFromResolvConf parses a resolv.conf(5)-style file and returns its nameserver list.
func systemDNSFromResolvConf(path string) []string {
	cfg, err := dns.ClientConfigFromFile(path)
	if err != nil {
		return nil
	}
	return cfg.Servers
}

// windowsDNS reads the currently effective IPv4 DNS servers via PowerShell (best-effort).
func windowsDNS() []string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-DnsClientServerAddress -AddressFamily IPv4).ServerAddresses")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}
