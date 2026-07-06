# Configurable Test Lists Design

## Goal

Allow users to extend the built-in benchmark domain list and DNS server list by editing files under `~/.config/dnspick`, without recompiling the program and without changing existing command-line behavior.

The change must support:

- `~/.config/dnspick/domain.list` as a plain-text file with one domain per line
- `~/.config/dnspick/server.list` as a plain-text file with one DNS server entry per line
- Appending custom entries to the built-in defaults when the matching CLI flag is not provided
- Preserving the current "CLI overrides default sources" behavior when `--domains` or `--servers` is explicitly set
- Deduplicating merged entries while preserving the first occurrence and its display name/order

## Current State

Today the built-in test domains and built-in DNS servers are hard-coded in `internal/dnsbench/servers.go`.

At runtime:

- `--domains` replaces the built-in domain list
- `--servers` replaces the built-in server list
- System DNS servers are appended unless `--no-system-dns` is set

There is no file-backed configuration for test inputs, so changing the benchmark scope requires either CLI flags every time or code changes.

## Requirements

### Functional

- When `--domains` is not provided, the benchmark domain set must be `built-in domains + domain.list`.
- When `--domains` is provided, only the CLI domains must be used; `domain.list` must not be read.
- When `--servers` is not provided, the benchmark server set must be `built-in servers + server.list`.
- When `--servers` is provided, only the CLI servers must be used; `server.list` must not be read.
- System DNS detection and appending must remain unchanged, except that duplicates must be removed against the already assembled server list.
- Editing either config file must affect the next run immediately, with no cache or generated state.

### File Format

`domain.list`:

- One domain per line
- Empty lines ignored
- Lines starting with `#` ignored

`server.list`:

- One server entry per line
- Empty lines ignored
- Lines starting with `#` ignored
- Supported entry syntax must match the current CLI parsing behavior:
  - `1.1.1.1`
  - `udp://8.8.8.8`
  - `tls://dns.google`
  - `https://dns.google/dns-query`
  - `h3://cloudflare-dns.com/dns-query`

### Error Handling

- Missing config directory or missing list files must be treated as "no custom additions", not as an error.
- If a file exists but cannot be read, the command must fail with an explicit error.
- Invalid individual server lines must be skipped, matching the current tolerant parsing behavior.
- Empty or comment-only files must behave the same as absent files.

## Design

### Configuration Layer

Add a small file-loading package at `internal/config`.

Responsibilities:

- Resolve the user config directory via `os.UserHomeDir()` and `filepath.Join(home, ".config", "dnspick")`
- Read `domain.list` and `server.list` if present
- Return line-based entries after trimming whitespace and removing empty/comment lines

This package must not know benchmark semantics such as domain categories, protocol scoring, or system DNS. It only handles file discovery and text ingestion.

Proposed API shape:

- `LoadDomainEntries() ([]string, error)`
- `LoadServerEntries() ([]string, error)`

The functions return:

- `nil, nil` when the file does not exist
- filtered entries when the file is readable
- a non-nil error when the path exists but cannot be read or is otherwise invalid for use

### Assembly Flow

`main.go` remains the orchestration point that decides which sources participate in the run.

Domain assembly:

1. If `--domains` was explicitly set, parse CLI domains and stop there.
2. Otherwise start from `dnsbench.DefaultDomains`.
3. Load `domain.list`.
4. Parse custom domains from file entries.
5. Merge built-in and custom domains with deduplication.

Server assembly:

1. If `--servers` was explicitly set, parse CLI servers and stop there.
2. Otherwise start from `dnsbench.DefaultServers`.
3. Load `server.list`.
4. Parse custom servers from file entries.
5. Merge built-in and custom servers with deduplication.
6. Unless `--no-system-dns` is set, detect system DNS and append it through the same deduplicating merge path.

This preserves current CLI behavior while adding a passive extension point for frequent local customization.

### Deduplication Rules

Deduplication must preserve first occurrence, because first occurrence also defines:

- output order
- chosen display name
- whether the built-in label or user-generated label is shown

#### Domains

Use a canonical key:

- `strings.ToLower(strings.TrimSpace(name))`

Behavior:

- Built-in domains stay first
- Custom file domains only add new entries
- Repeated domains in the config file collapse to one

#### Servers

Use a canonical key composed from:

- protocol
- normalized address

Normalization rules:

- UDP and DoT: trim outer whitespace and use the parsed address as-is
- DoH and DoH3: use the parsed URL after current scheme handling
- System DNS goes through the same server deduplication path, so a system resolver already present in built-ins or config is not tested twice

The deduplication key is:

- `string(protocol) + "|" + normalizedAddress`

This avoids collisions such as the same host used through different transports.

### Parsing Reuse

Avoid duplicating parsing logic between CLI and file-backed inputs.

Refactor `internal/dnsbench` parsing helpers so they can be reused by:

- existing comma-separated CLI parsing
- new line-based config parsing

One acceptable structure:

- keep `ParseDomains(raw string)` and `ParseServers(raw string)` for CLI compatibility
- add helpers that parse from `[]string`
- route both CLI and config callers through the same item-level parsing and dedup primitives

This keeps protocol support consistent across all input paths.

## Components

### `internal/config`

- Finds config directory
- Loads file content
- Splits content into filtered line entries

### `internal/dnsbench`

- Owns domain and server parsing
- Owns merge and dedup helpers for `[]Domain` and `[]Server`
- Continues to own built-in defaults

### `main.go`

- Selects active input sources based on flags
- Handles user-facing errors from config loading
- Preserves current system DNS flag behavior

## Data Flow

Default run without CLI overrides:

1. `main.go` starts with built-in defaults.
2. `internal/config` loads optional extra entries from `~/.config/dnspick`.
3. `internal/dnsbench` parses and deduplicates the merged domain and server sets.
4. `main.go` appends system DNS via the same dedup-aware merge.
5. Benchmark execution proceeds unchanged.

Run with CLI overrides:

1. `main.go` sees `--domains` and/or `--servers`.
2. Matching config file is skipped entirely.
3. CLI values are parsed using existing rules.
4. System DNS handling remains unchanged unless disabled.

## User-Visible Behavior

Examples:

- With no config files: behavior is unchanged from today.
- With `domain.list` present: its domains are added to the built-in domain pool.
- With `server.list` present: its servers are added to the built-in server pool.
- With `--domains`: only CLI domains are used, regardless of `domain.list`.
- With `--servers`: only CLI servers are used, regardless of `server.list`.
- With `--no-system-dns`: system DNS is not appended, even if config files exist.

No files are auto-generated. Users create or edit the files manually when they want local customization.

## Testing

Add tests for:

- Reading line-based config files, including empty lines and `#` comments
- Missing config files returning no entries and no error
- Existing but unreadable/invalid config files returning an error
- Domain merge behavior preserving built-in-first ordering and deduplication
- Server merge behavior preserving built-in-first ordering and deduplication
- Duplicate elimination across built-in, config, and system DNS sources
- CLI override behavior: config files are skipped when the corresponding flag is set

Keep existing parser tests and extend them only where needed for normalization or new helper behavior.

## Documentation Changes

Update `README.md` and `README.zh-CN.md` to document:

- config directory path: `~/.config/dnspick`
- `domain.list` and `server.list` formats
- merge semantics: built-ins plus config additions, with deduplication
- CLI precedence over config files
- continued role of `--no-system-dns`

## Tradeoffs

### Why not fully externalize the built-in defaults

Keeping built-in defaults in code avoids:

- first-run file generation
- template migration problems on upgrades
- ambiguity around whether user-edited files should be overwritten

This approach provides configurable extensions without adding lifecycle complexity.

### Why not use JSON or YAML

Plain-text line-based files match the user requirement and the existing CLI mental model. They are also easier to edit quickly for ad hoc testing.

## Out of Scope

- Auto-generating default config files
- Making the config directory path itself configurable
- Adding structured configuration for unrelated runtime options
- Changing benchmark scoring, protocols, or system DNS detection behavior
