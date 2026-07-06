package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	configDirName  = "dnspick"
	configRootDir  = ".config"
	domainListFile = "domain.list"
	serverListFile = "server.list"
)

// EnsureDomainList creates ~/.config/dnspick/domain.list with defaults when it
// is missing. Existing files are left untouched.
func EnsureDomainList(defaults []string) error {
	return ensureNamedList(domainListFile, defaults)
}

// EnsureServerList creates ~/.config/dnspick/server.list with defaults when it
// is missing. Existing files are left untouched.
func EnsureServerList(defaults []string) error {
	return ensureNamedList(serverListFile, defaults)
}

// LoadDomainEntries reads ~/.config/dnspick/domain.list and reports whether the
// file exists. Empty lines and # comments are ignored.
func LoadDomainEntries() ([]string, bool, error) {
	return loadNamedList(domainListFile)
}

// LoadServerEntries reads ~/.config/dnspick/server.list and reports whether the
// file exists. Empty lines and # comments are ignored.
func LoadServerEntries() ([]string, bool, error) {
	return loadNamedList(serverListFile)
}

func ensureNamedList(name string, defaults []string) error {
	path, err := listPath(name)
	if err != nil {
		return err
	}
	return ensureListFile(path, defaults)
}

func loadNamedList(name string) ([]string, bool, error) {
	path, err := listPath(name)
	if err != nil {
		return nil, false, err
	}
	return loadListFile(path)
}

func listPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configRootDir, configDirName, name), nil
}

func ensureListFile(path string, defaults []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	if len(defaults) == 0 {
		return nil
	}

	content := strings.Join(defaults, "\n") + "\n"
	_, err = f.WriteString(content)
	return err
}

func loadListFile(path string) ([]string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer f.Close()

	var entries []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, true, err
	}
	return entries, true, nil
}
