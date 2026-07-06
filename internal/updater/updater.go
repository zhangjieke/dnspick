// Package updater checks for and applies in-place updates of dnspick itself
// from GitHub Releases.
package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/minio/selfupdate"
	"golang.org/x/mod/semver"
)

const (
	owner = "zhangjieke"
	repo  = "dnspick"
	app   = "dnspick"

	// maxDownloadSize caps the update archive read to prevent unbounded
	// memory usage from abnormally large or malicious responses.
	maxDownloadSize = 200 << 20 // 200 MB
)

// DefaultTimeout is the suggested timeout for a check/update operation.
const DefaultTimeout = 60 * time.Second

// newHTTPClient builds an HTTP client with explicit timeouts instead of relying
// on http.DefaultClient (which has no timeout at all).
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: DefaultTimeout}
}

// release is the subset of GitHub API release fields we use.
type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// CheckResult describes the outcome of an update check.
type CheckResult struct {
	Current   string // current version (may be "dev")
	Latest    string // latest released version, e.g. v2.1.0
	HasUpdate bool   // whether an update is available
	URL       string // URL of the latest release page
}

// Update checks for and, when a newer version exists, replaces the current
// executable in place. Returns the version updated to; if already up to date,
// updated is false.
func Update(ctx context.Context, current string) (latest string, updated bool, err error) {
	res, err := Check(ctx, current)
	if err != nil {
		return "", false, err
	}
	if !res.HasUpdate {
		return res.Latest, false, nil
	}

	bin, err := downloadBinary(ctx, res.Latest)
	if err != nil {
		return res.Latest, false, err
	}
	if err := selfupdate.Apply(bytes.NewReader(bin), selfupdate.Options{}); err != nil {
		// On failure, attempt to roll back to the original file.
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return res.Latest, false, fmt.Errorf("update failed and rollback failed: %w", rerr)
		}
		return res.Latest, false, fmt.Errorf("update failed (rolled back): %w", err)
	}
	return res.Latest, true, nil
}

// Check queries the latest release on GitHub and compares it to current.
func Check(ctx context.Context, current string) (*CheckResult, error) {
	rel, err := latestRelease(ctx)
	if err != nil {
		return nil, err
	}
	return &CheckResult{
		Current:   current,
		Latest:    rel.TagName,
		HasUpdate: isNewer(current, rel.TagName),
		URL:       rel.HTMLURL,
	}, nil
}

func latestRelease(ctx context.Context) (*release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query the latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query the latest release: GitHub returned %s", resp.Status)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("no release found")
	}
	return &rel, nil
}

// isNewer reports whether latest is newer than current. When current is not a
// valid semver (e.g. "dev" or a dirty build), an update is always assumed.
func isNewer(current, latest string) bool {
	if !semver.IsValid(current) {
		return true
	}
	return semver.Compare(current, latest) < 0
}

// downloadBinary downloads the archive for the given version matching the
// current platform and extracts the executable from it.
func downloadBinary(ctx context.Context, tag string) ([]byte, error) {
	base := fmt.Sprintf("%s-%s-%s", app, runtime.GOOS, runtime.GOARCH)
	isZip := runtime.GOOS == "windows"
	asset := base + ".tar.gz"
	if isZip {
		asset = base + ".zip"
	}
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, asset)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download the update package: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download the update package: %s returned %s", asset, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read the update package: %w", err)
	}

	if isZip {
		return binaryFromZip(data)
	}
	return binaryFromTarGz(data)
}

// binaryFromTarGz returns the contents of the first regular file in the tar.gz
// (the archive contains a single binary).
func binaryFromTarGz(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decompress the update package: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decompress the update package: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("no executable found in the update package")
}

// binaryFromZip returns the contents of the first .exe file in the zip.
func binaryFromZip(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to decompress the update package: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to decompress the update package: %w", err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read the update package: %w", err)
		}
		return b, nil
	}
	return nil, fmt.Errorf("no executable found in the update package")
}
