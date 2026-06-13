// Package buildinfo holds version information injected at build time via
// -ldflags -X.
package buildinfo

import "fmt"

// These variables are overridden at build time by
// -ldflags "-X .../buildinfo.Version=..."; a plain go run / go build (no
// injection) keeps the defaults below.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a one-line description suitable for `--version` output.
func String() string {
	return fmt.Sprintf("dnspick %s (commit %s, built %s)", Version, Commit, Date)
}
