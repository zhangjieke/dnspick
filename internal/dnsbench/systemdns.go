package dnsbench

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"github.com/miekg/dns"
)

// DetectSystemDNS 探测当前系统配置的默认 DNS 服务器（运营商或路由器下发），
// 返回可参与基准测试的 Server 列表。无法检测时返回 nil（功能优雅跳过）。
func DetectSystemDNS() []Server {
	var ips []string
	if runtime.GOOS == "windows" {
		ips = windowsDNS()
	} else {
		ips = systemDNSFromResolvConf("/etc/resolv.conf")
	}
	return buildSystemServers(ips)
}

// buildSystemServers 把 IP 列表去重并转换为带 IsSystem 标记的 Server。
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
	// 命名：单个为「当前默认 DNS」，多个则编号。
	for i := range servers {
		if len(servers) == 1 {
			servers[i].Name = "当前默认 DNS"
		} else {
			servers[i].Name = fmt.Sprintf("当前默认 DNS %d", i+1)
		}
	}
	return servers
}

// systemDNSFromResolvConf 解析 resolv.conf(5) 风格文件，返回 nameserver 列表。
func systemDNSFromResolvConf(path string) []string {
	cfg, err := dns.ClientConfigFromFile(path)
	if err != nil {
		return nil
	}
	return cfg.Servers
}

// windowsDNS 通过 PowerShell 读取当前生效的 IPv4 DNS 服务器（best-effort）。
func windowsDNS() []string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-DnsClientServerAddress -AddressFamily IPv4).ServerAddresses")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}
