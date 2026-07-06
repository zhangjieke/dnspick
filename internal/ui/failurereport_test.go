package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zhangjieke/dnspick/internal/dnsbench"
)

func TestWriteFailureReport(t *testing.T) {
	origPreferred := preferredFailureReportDir
	origSystemTempDir := systemTempDir
	origWriteFile := writeFailureReportFile
	preferredFailureReportDir = t.TempDir()
	systemTempDir = func() string { return t.TempDir() }
	writeFailureReportFile = os.WriteFile
	defer func() {
		preferredFailureReportDir = origPreferred
		systemTempDir = origSystemTempDir
		writeFailureReportFile = origWriteFile
	}()

	failures := []dnsbench.FailureRecord{
		{ServerName: "Fast DNS", Address: "1.1.1.1", Protocol: dnsbench.UDP, Domain: "google.com", Error: "timeout"},
		{ServerName: "Fast DNS", Address: "1.1.1.1", Protocol: dnsbench.UDP, Domain: "google.com", Error: "timeout"},
		{ServerName: "Fast DNS", Address: "1.1.1.1", Protocol: dnsbench.UDP, Domain: "youtube.com", Error: "servfail"},
		{ServerName: "DoH DNS", Address: "https://dns.google/dns-query", Protocol: dnsbench.DOH, Domain: "google.com", Error: "HTTP status 503"},
	}

	now := time.Date(2026, 7, 6, 15, 30, 45, 0, time.FixedZone("CST", 8*3600))
	path, err := WriteFailureReport(failures, now)
	if err != nil {
		t.Fatalf("WriteFailureReport: %v", err)
	}
	wantPath := filepath.Join(preferredFailureReportDir, "dnspick-failures-20260706-153045.txt")
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"dnspick failure report",
		"server: Fast DNS",
		"domain: google.com",
		"failures: 2",
		"error[2]: timeout",
		"server: DoH DNS",
		"protocol: doh",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q:\n%s", want, text)
		}
	}
}

func TestWriteFailureReportFallsBackFromPreferredDir(t *testing.T) {
	origPreferred := preferredFailureReportDir
	origSystemTempDir := systemTempDir
	origWriteFile := writeFailureReportFile
	preferredFailureReportDir = "/tmp/preferred-missing"
	fallbackDir := t.TempDir()
	systemTempDir = func() string { return fallbackDir }
	writeFailureReportFile = func(path string, data []byte, perm os.FileMode) error {
		if strings.HasPrefix(path, preferredFailureReportDir) {
			return errors.New("preferred dir unavailable")
		}
		return os.WriteFile(path, data, perm)
	}
	defer func() {
		preferredFailureReportDir = origPreferred
		systemTempDir = origSystemTempDir
		writeFailureReportFile = origWriteFile
	}()

	now := time.Date(2026, 7, 6, 15, 30, 45, 0, time.UTC)
	path, err := WriteFailureReport([]dnsbench.FailureRecord{
		{ServerName: "Fast DNS", Address: "1.1.1.1", Protocol: dnsbench.UDP, Domain: "google.com", Error: "timeout"},
	}, now)
	if err != nil {
		t.Fatalf("WriteFailureReport: %v", err)
	}

	wantPath := filepath.Join(fallbackDir, "dnspick-failures-20260706-153045.txt")
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected fallback report file to exist: %v", err)
	}
}

func TestWriteFailureReportNoFailures(t *testing.T) {
	path, err := WriteFailureReport(nil, time.Now())
	if err != nil {
		t.Fatalf("WriteFailureReport: %v", err)
	}
	if path != "" {
		t.Fatalf("expected empty path, got %q", path)
	}
}
