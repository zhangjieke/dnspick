package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/zhangjieke/dnspick/internal/dnsbench"
)

var (
	preferredFailureReportDir = "/tmp"
	systemTempDir             = os.TempDir
	writeFailureReportFile    = os.WriteFile
)

type failureGroup struct {
	serverName string
	address    string
	protocol   dnsbench.Protocol
	isSystem   bool
	total      int
	domains    map[string]map[string]int
}

// WriteFailureReport writes a plain-text failure report, preferring /tmp for a
// stable user-visible path and falling back to the system temp directory when
// needed. The timestamp is provided by the caller so tests can make the
// filename deterministic.
func WriteFailureReport(failures []dnsbench.FailureRecord, now time.Time) (string, error) {
	if len(failures) == 0 {
		return "", nil
	}

	filename := "dnspick-failures-" + now.Format("20060102-150405") + ".txt"
	var b strings.Builder
	writeFailureReport(&b, failures, now)

	var lastErr error
	for _, path := range failureReportPaths(filename) {
		if err := writeFailureReportFile(path, []byte(b.String()), 0o644); err != nil {
			lastErr = err
			continue
		}
		return path, nil
	}
	return "", lastErr
}

func writeFailureReport(b *strings.Builder, failures []dnsbench.FailureRecord, now time.Time) {
	groups := groupFailures(failures)

	b.WriteString("dnspick failure report\n")
	b.WriteString("generated_at: ")
	b.WriteString(now.Format(time.RFC3339))
	b.WriteString("\n\n")

	for i, g := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("server: ")
		b.WriteString(g.serverName)
		if g.isSystem {
			b.WriteString(" (system)")
		}
		b.WriteString("\n")
		b.WriteString("address: ")
		b.WriteString(g.address)
		b.WriteString("\n")
		b.WriteString("protocol: ")
		b.WriteString(string(g.protocol))
		b.WriteString("\n")
		b.WriteString("total_failures: ")
		b.WriteString(fmt.Sprintf("%d", g.total))
		b.WriteString("\n\n")

		domains := make([]string, 0, len(g.domains))
		for domain := range g.domains {
			domains = append(domains, domain)
		}
		slices.Sort(domains)

		for di, domain := range domains {
			errCounts := g.domains[domain]
			if di > 0 {
				b.WriteString("\n")
			}
			b.WriteString("  domain: ")
			b.WriteString(domain)
			b.WriteString("\n")

			failuresForDomain := 0
			for _, count := range errCounts {
				failuresForDomain += count
			}
			b.WriteString("  failures: ")
			b.WriteString(fmt.Sprintf("%d", failuresForDomain))
			b.WriteString("\n")

			errors := make([]string, 0, len(errCounts))
			for errText := range errCounts {
				errors = append(errors, errText)
			}
			slices.Sort(errors)
			for _, errText := range errors {
				b.WriteString("    error[")
				b.WriteString(fmt.Sprintf("%d", errCounts[errText]))
				b.WriteString("]: ")
				b.WriteString(errText)
				b.WriteString("\n")
			}
		}
	}
}

func failureReportPaths(filename string) []string {
	var paths []string
	appendPath := func(dir string) {
		if dir == "" {
			return
		}
		path := filepath.Join(dir, filename)
		if len(paths) > 0 && paths[len(paths)-1] == path {
			return
		}
		paths = append(paths, path)
	}

	appendPath(preferredFailureReportDir)
	appendPath(systemTempDir())
	return paths
}

func groupFailures(failures []dnsbench.FailureRecord) []failureGroup {
	order := make([]string, 0)
	groupByServer := make(map[string]*failureGroup)

	for _, f := range failures {
		key := string(f.Protocol) + "|" + f.Address + "|" + f.ServerName
		group, ok := groupByServer[key]
		if !ok {
			group = &failureGroup{
				serverName: f.ServerName,
				address:    f.Address,
				protocol:   f.Protocol,
				isSystem:   f.IsSystem,
				domains:    make(map[string]map[string]int),
			}
			groupByServer[key] = group
			order = append(order, key)
		}
		group.total++
		if group.domains[f.Domain] == nil {
			group.domains[f.Domain] = make(map[string]int)
		}
		group.domains[f.Domain][f.Error]++
	}

	out := make([]failureGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *groupByServer[key])
	}
	return out
}
