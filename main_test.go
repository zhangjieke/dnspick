package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/zhangjieke/dnspick/internal/dnsbench"
)

func TestLangFromArgs(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"--version"}, ""},
		{[]string{"--lang", "zh"}, "zh"},
		{[]string{"--lang=zh"}, "zh"},
		{[]string{"--lang=en"}, "en"},
		{[]string{"-d", "example.com", "--lang", "zh"}, "zh"},
		{[]string{"--lang"}, ""}, // --lang without value
	}
	for _, tt := range tests {
		if got := langFromArgs(tt.args); got != tt.want {
			t.Errorf("langFromArgs(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestBenchmarkDomainsUsesConfigWhenFlagUnset(t *testing.T) {
	origEnsure := ensureDomainList
	origLoad := loadDomainEntries
	ensureDomainList = func([]string) error { return nil }
	loadDomainEntries = func() ([]string, bool, error) {
		return []string{"example.com"}, true, nil
	}
	defer func() {
		ensureDomainList = origEnsure
		loadDomainEntries = origLoad
	}()

	cmd := newTestCommand(t)
	got, err := benchmarkDomains(cmd)
	if err != nil {
		t.Fatalf("benchmarkDomains: %v", err)
	}

	if !containsDomain(got, "example.com") {
		t.Fatalf("expected config domain to be used, got %v", got)
	}
	if containsDomain(got, "google.com") {
		t.Fatalf("expected built-in domains to be ignored when config exists, got %v", got)
	}
}

func TestBenchmarkDomainsSkipsConfigWhenFlagSet(t *testing.T) {
	origEnsure := ensureDomainList
	origLoad := loadDomainEntries
	ensureDomainList = func([]string) error {
		t.Fatal("ensure should not be called when --domains is set")
		return nil
	}
	loadDomainEntries = func() ([]string, bool, error) {
		t.Fatal("config loader should not be called when --domains is set")
		return nil, false, nil
	}
	defer func() {
		ensureDomainList = origEnsure
		loadDomainEntries = origLoad
	}()

	domainsStr = "cli.com,example.com"
	t.Cleanup(func() { domainsStr = "" })

	cmd := newTestCommand(t)
	if err := cmd.Flags().Set("domains", domainsStr); err != nil {
		t.Fatalf("Set domains flag: %v", err)
	}

	got, err := benchmarkDomains(cmd)
	if err != nil {
		t.Fatalf("benchmarkDomains: %v", err)
	}
	want := []dnsbench.Domain{
		{Name: "cli.com", Category: dnsbench.CategoryCustom},
		{Name: "example.com", Category: dnsbench.CategoryCustom},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBenchmarkServersUsesConfigAndDedupsSystemDNS(t *testing.T) {
	origEnsure := ensureServerList
	origLoad := loadServerEntries
	origDetect := detectSystemDNS
	ensureServerList = func([]string) error { return nil }
	loadServerEntries = func() ([]string, bool, error) {
		return []string{"9.9.9.9", "tls://dns.google"}, true, nil
	}
	detectSystemDNS = func(_, _ string) []dnsbench.Server {
		return []dnsbench.Server{
			{Name: "System Duplicate", Address: "9.9.9.9", Protocol: dnsbench.UDP, IsSystem: true},
			{Name: "System New", Address: "8.8.8.8", Protocol: dnsbench.UDP, IsSystem: true},
		}
	}
	defer func() {
		ensureServerList = origEnsure
		loadServerEntries = origLoad
		detectSystemDNS = origDetect
	}()

	noSystemDNS = false
	t.Cleanup(func() { noSystemDNS = false })

	cmd := newTestCommand(t)
	got, err := benchmarkServers(cmd, "System DNS", "System DNS %d")
	if err != nil {
		t.Fatalf("benchmarkServers: %v", err)
	}

	if countServer(got, dnsbench.UDP, "9.9.9.9") != 1 {
		t.Fatalf("expected server dedup for config/system DNS, got %v", got)
	}
	if containsServer(got, dnsbench.UDP, "223.5.5.5") {
		t.Fatalf("expected built-in servers to be ignored when config exists, got %v", got)
	}
	if !containsServer(got, dnsbench.DOT, "dns.google") {
		t.Fatalf("expected config DoT server, got %v", got)
	}
	if !containsServer(got, dnsbench.UDP, "8.8.8.8") {
		t.Fatalf("expected unique system DNS, got %v", got)
	}
}

func TestBenchmarkServersSkipsConfigWhenFlagSet(t *testing.T) {
	origEnsure := ensureServerList
	origLoad := loadServerEntries
	origDetect := detectSystemDNS
	ensureServerList = func([]string) error {
		t.Fatal("ensure should not be called when --servers is set")
		return nil
	}
	loadServerEntries = func() ([]string, bool, error) {
		t.Fatal("config loader should not be called when --servers is set")
		return nil, false, nil
	}
	detectSystemDNS = func(_, _ string) []dnsbench.Server { return nil }
	defer func() {
		ensureServerList = origEnsure
		loadServerEntries = origLoad
		detectSystemDNS = origDetect
	}()

	serversStr = "1.1.1.1,tls://dns.google"
	t.Cleanup(func() { serversStr = "" })
	noSystemDNS = true
	t.Cleanup(func() { noSystemDNS = false })

	cmd := newTestCommand(t)
	if err := cmd.Flags().Set("servers", serversStr); err != nil {
		t.Fatalf("Set servers flag: %v", err)
	}

	got, err := benchmarkServers(cmd, "System DNS", "System DNS %d")
	if err != nil {
		t.Fatalf("benchmarkServers: %v", err)
	}
	want := []dnsbench.Server{
		{Name: "1.1.1.1 (UDP)", Address: "1.1.1.1", Protocol: dnsbench.UDP},
		{Name: "dns.google (DoT)", Address: "dns.google", Protocol: dnsbench.DOT},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBenchmarkDomainsConfigLoadError(t *testing.T) {
	origEnsure := ensureDomainList
	origLoad := loadDomainEntries
	ensureDomainList = func([]string) error { return nil }
	loadDomainEntries = func() ([]string, bool, error) {
		return nil, false, errors.New("boom")
	}
	defer func() {
		ensureDomainList = origEnsure
		loadDomainEntries = origLoad
	}()

	cmd := newTestCommand(t)
	if _, err := benchmarkDomains(cmd); err == nil {
		t.Fatal("expected config load error")
	}
}

func TestBenchmarkServersConfigLoadError(t *testing.T) {
	origEnsure := ensureServerList
	origLoad := loadServerEntries
	ensureServerList = func([]string) error { return nil }
	loadServerEntries = func() ([]string, bool, error) {
		return nil, false, errors.New("boom")
	}
	defer func() {
		ensureServerList = origEnsure
		loadServerEntries = origLoad
	}()

	noSystemDNS = true
	t.Cleanup(func() { noSystemDNS = false })

	cmd := newTestCommand(t)
	if _, err := benchmarkServers(cmd, "System DNS", "System DNS %d"); err == nil {
		t.Fatal("expected config load error")
	}
}

func TestBenchmarkDomainsFallsBackWhenConfigMissing(t *testing.T) {
	origEnsure := ensureDomainList
	origLoad := loadDomainEntries
	ensureDomainList = func([]string) error { return nil }
	loadDomainEntries = func() ([]string, bool, error) {
		return nil, false, nil
	}
	defer func() {
		ensureDomainList = origEnsure
		loadDomainEntries = origLoad
	}()

	cmd := newTestCommand(t)
	got, err := benchmarkDomains(cmd)
	if err != nil {
		t.Fatalf("benchmarkDomains: %v", err)
	}
	if !containsDomain(got, "google.com") {
		t.Fatalf("expected built-in fallback when config is missing, got %v", got)
	}
}

func TestBenchmarkServersFallsBackWhenConfigMissing(t *testing.T) {
	origEnsure := ensureServerList
	origLoad := loadServerEntries
	origDetect := detectSystemDNS
	ensureServerList = func([]string) error { return nil }
	loadServerEntries = func() ([]string, bool, error) {
		return nil, false, nil
	}
	detectSystemDNS = func(_, _ string) []dnsbench.Server { return nil }
	defer func() {
		ensureServerList = origEnsure
		loadServerEntries = origLoad
		detectSystemDNS = origDetect
	}()

	noSystemDNS = false
	t.Cleanup(func() { noSystemDNS = false })

	cmd := newTestCommand(t)
	got, err := benchmarkServers(cmd, "System DNS", "System DNS %d")
	if err != nil {
		t.Fatalf("benchmarkServers: %v", err)
	}
	if !containsServer(got, dnsbench.UDP, "223.5.5.5") {
		t.Fatalf("expected built-in fallback when config is missing, got %v", got)
	}
}

func TestBenchmarkDomainsEnsureError(t *testing.T) {
	origEnsure := ensureDomainList
	ensureDomainList = func([]string) error { return errors.New("boom") }
	defer func() { ensureDomainList = origEnsure }()

	cmd := newTestCommand(t)
	if _, err := benchmarkDomains(cmd); err == nil {
		t.Fatal("expected ensure error")
	}
}

func TestBenchmarkServersEnsureError(t *testing.T) {
	origEnsure := ensureServerList
	ensureServerList = func([]string) error { return errors.New("boom") }
	defer func() { ensureServerList = origEnsure }()

	noSystemDNS = true
	t.Cleanup(func() { noSystemDNS = false })

	cmd := newTestCommand(t)
	if _, err := benchmarkServers(cmd, "System DNS", "System DNS %d"); err == nil {
		t.Fatal("expected ensure error")
	}
}

func TestDefaultDomainEntries(t *testing.T) {
	got := defaultDomainEntries()
	if len(got) != len(dnsbench.DefaultDomains) {
		t.Fatalf("got %d entries, want %d", len(got), len(dnsbench.DefaultDomains))
	}
	if got[0] != dnsbench.DefaultDomains[0].Name {
		t.Fatalf("first entry = %q, want %q", got[0], dnsbench.DefaultDomains[0].Name)
	}
}

func TestServerConfigEntry(t *testing.T) {
	tests := []struct {
		server dnsbench.Server
		want   string
	}{
		{server: dnsbench.Server{Address: "1.1.1.1", Protocol: dnsbench.UDP}, want: "1.1.1.1"},
		{server: dnsbench.Server{Address: "dns.google", Protocol: dnsbench.DOT}, want: "tls://dns.google"},
		{server: dnsbench.Server{Address: "https://dns.google/dns-query", Protocol: dnsbench.DOH}, want: "https://dns.google/dns-query"},
		{server: dnsbench.Server{Address: "https://dns.google/dns-query", Protocol: dnsbench.DOH3}, want: "h3://dns.google/dns-query"},
	}
	for _, tt := range tests {
		if got := serverConfigEntry(tt.server); got != tt.want {
			t.Fatalf("serverConfigEntry(%+v) = %q, want %q", tt.server, got, tt.want)
		}
	}
}

func newTestCommand(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("domains", "", "")
	cmd.Flags().String("servers", "", "")
	return cmd
}

func containsDomain(domains []dnsbench.Domain, name string) bool {
	return countDomain(domains, name) > 0
}

func countDomain(domains []dnsbench.Domain, name string) int {
	count := 0
	for _, d := range domains {
		if normalizeDomainName(d.Name) == normalizeDomainName(name) {
			count++
		}
	}
	return count
}

func normalizeDomainName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func containsServer(servers []dnsbench.Server, protocol dnsbench.Protocol, address string) bool {
	return countServer(servers, protocol, address) > 0
}

func countServer(servers []dnsbench.Server, protocol dnsbench.Protocol, address string) int {
	count := 0
	for _, s := range servers {
		if s.Protocol == protocol && s.Address == address {
			count++
		}
	}
	return count
}
