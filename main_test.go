package main

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

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
	split := slices.Index(got, "")
	if split < 0 {
		t.Fatalf("expected blank separator, got %v", got)
	}
	domestic := got[:split]
	foreign := got[split+1:]
	if !slices.IsSorted(domestic) {
		t.Fatalf("domestic group not sorted: %v", domestic)
	}
	if !slices.IsSorted(foreign) {
		t.Fatalf("foreign group not sorted: %v", foreign)
	}
	if domestic[0] != "163.com" || domestic[len(domestic)-1] != "zhihu.com" {
		t.Fatalf("unexpected domestic range: %v", domestic)
	}
	if foreign[0] != "anthropic.com" || foreign[len(foreign)-1] != "youtube.com" {
		t.Fatalf("unexpected foreign range: %v", foreign)
	}
}

func TestDefaultServerEntries(t *testing.T) {
	got := defaultServerEntries()
	groups := splitOnEmpty(got)
	if len(groups) != 4 {
		t.Fatalf("expected 4 protocol groups, got %d: %v", len(groups), groups)
	}
	for i := 1; i < len(groups); i++ {
		if !slices.IsSorted(groups[i]) {
			t.Fatalf("group %d not sorted: %v", i, groups[i])
		}
	}
	if containsString(got, "127.0.0.1") {
		t.Fatalf("default server entries should not include localhost: %v", got)
	}
	wantUDP := []string{
		"1.0.0.1",
		"1.1.1.1",
		"8.8.4.4",
		"8.8.8.8",
		"9.9.9.9",
		"52.80.52.52",
		"114.114.114.114",
		"114.114.115.115",
		"117.50.10.10",
		"119.28.28.28",
		"119.29.29.29",
		"180.76.76.76",
		"180.184.1.1",
		"180.184.2.2",
		"208.67.220.220",
		"208.67.222.222",
		"223.5.5.5",
		"223.6.6.6",
	}
	if !slices.Equal(groups[0], wantUDP) {
		t.Fatalf("unexpected UDP group: %v", groups[0])
	}
	if groups[1][0] != "tls://dns.alidns.com" {
		t.Fatalf("unexpected DoT group: %v", groups[1])
	}
	if groups[2][0] != "https://cloudflare-dns.com/dns-query" {
		t.Fatalf("unexpected DoH group: %v", groups[2])
	}
	if groups[3][0] != "h3://cloudflare-dns.com/dns-query" {
		t.Fatalf("unexpected DoH3 group: %v", groups[3])
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

func TestReportFailureOutputPrintsPath(t *testing.T) {
	origWriter := writeFailureReport
	origStdout := stdoutWriter
	writeFailureReport = func([]dnsbench.FailureRecord, time.Time) (string, error) {
		return "/tmp/dnspick-failures-test.txt", nil
	}
	var out bytes.Buffer
	stdoutWriter = &out
	defer func() {
		writeFailureReport = origWriter
		stdoutWriter = origStdout
	}()

	failures := []dnsbench.FailureRecord{{ServerName: "X", Address: "1.1.1.1", Protocol: dnsbench.UDP, Domain: "a.com", Error: "boom"}}
	reportFailureOutput(failures)
	stdout := out.String()
	if !strings.Contains(stdout, "/tmp/dnspick-failures-test.txt") {
		t.Fatalf("stdout = %q, want failure report path", stdout)
	}
}

func TestReportFailureOutputWarnsOnWriteError(t *testing.T) {
	origWriter := writeFailureReport
	origStderr := stderrWriter
	writeFailureReport = func([]dnsbench.FailureRecord, time.Time) (string, error) {
		return "", errors.New("boom")
	}
	var errOut bytes.Buffer
	stderrWriter = &errOut
	defer func() {
		writeFailureReport = origWriter
		stderrWriter = origStderr
	}()

	failures := []dnsbench.FailureRecord{{ServerName: "X", Address: "1.1.1.1", Protocol: dnsbench.UDP, Domain: "a.com", Error: "boom"}}
	reportFailureOutput(failures)
	stderr := errOut.String()
	if !strings.Contains(stderr, "failed to write failure report") {
		t.Fatalf("stderr = %q, want warning", stderr)
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

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func splitOnEmpty(items []string) [][]string {
	var groups [][]string
	var current []string
	for _, item := range items {
		if item == "" {
			if len(current) > 0 {
				groups = append(groups, current)
				current = nil
			}
			continue
		}
		current = append(current, item)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
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
