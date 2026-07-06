# Config Files As Default Source Design

## Goal

Change the benchmark input source model so that the user-editable files under
`~/.config/dnspick` become the default source of truth once they exist.

The new behavior must:

- Automatically create `~/.config/dnspick/domain.list` and `~/.config/dnspick/server.list` with the current built-in defaults
- Create those files on first run when they are missing
- Never overwrite existing files, including during later runs or upgrades
- Use the file contents as the default benchmark list when the file exists
- Fall back to the built-in defaults only when the corresponding file does not exist
- Keep `--domains` and `--servers` as explicit overrides of the default source

## Current State

The previous design and implementation added file-backed extensions to the built-in defaults:

- missing config files meant "use built-ins"
- present config files meant "built-ins plus file entries"

That model makes the files useful for appending custom items, but it does not
make them the single place the user can inspect and maintain the effective
default lists. After exporting the built-ins into the files, the runtime still
internally starts from the built-in lists and deduplicates.

## Requirements

### Functional

- On startup, ensure `domain.list` and `server.list` exist under `~/.config/dnspick`.
- If a list file does not exist, create it with the current built-in defaults.
- If a list file already exists, never modify it automatically.
- Domain and server files are handled independently.
- If `domain.list` exists, the default domain source is exactly that file.
- If `domain.list` does not exist, the default domain source is the built-in domain list.
- If `server.list` exists, the default server source is exactly that file.
- If `server.list` does not exist, the default server source is the built-in server list.
- System DNS is still appended afterwards unless `--no-system-dns` is set.
- `--domains` and `--servers` continue to bypass the default source selection entirely.

### Semantics Of Existing Files

- An existing empty file is treated as an explicit user-owned configuration.
- An existing comment-only file is treated as an explicit user-owned configuration.
- In both cases the result is an empty parsed list, which then follows the existing error behavior for "no valid domains" or "no valid servers".

### Upgrade Behavior

- Re-running the program must not overwrite user-maintained files.
- Future install/update flows may also create missing files, but must not overwrite existing ones.
- This design focuses on runtime creation because it works consistently across install methods.

## Design

### Runtime Bootstrap

Before selecting default domains or servers, the program bootstraps the config directory and list files:

1. Ensure `~/.config/dnspick` exists.
2. If `domain.list` is missing, create it using the built-in domain defaults.
3. If `server.list` is missing, create it using the built-in server defaults.
4. If either file already exists, leave it untouched.

This means the first successful run converts the program into file-backed defaults for all later runs.

### Source Selection Rules

Domain source:

1. If `--domains` is provided, use CLI domains only.
2. Otherwise, ensure `domain.list` exists.
3. If `domain.list` exists, parse and use it as the entire default domain source.
4. Only if `domain.list` still does not exist because creation or detection is skipped unexpectedly, fall back to built-in defaults.

Server source:

1. If `--servers` is provided, use CLI servers only.
2. Otherwise, ensure `server.list` exists.
3. If `server.list` exists, parse and use it as the entire default server source.
4. Only if `server.list` still does not exist because creation or detection is skipped unexpectedly, fall back to built-in defaults.
5. Append system DNS afterwards unless disabled.

### Config Package Responsibilities

`internal/config` grows from a read-only helper into a tiny bootstrap-and-read layer.

Responsibilities:

- locate `~/.config/dnspick`
- ensure the directory exists
- create `domain.list` and `server.list` if missing
- read list files when present
- report whether a file exists independently from its parsed content

Suggested API shape:

- `EnsureDomainList(defaults []string) error`
- `EnsureServerList(defaults []string) error`
- `LoadDomainEntries() (entries []string, exists bool, err error)`
- `LoadServerEntries() (entries []string, exists bool, err error)`

The `exists` flag is necessary to distinguish:

- file missing: fall back to built-ins
- file present but empty/comment-only: explicit empty configuration

### Template Content

#### `domain.list`

Write one domain per line, matching the current built-in defaults in order.

#### `server.list`

Write one server entry per line in user-editable transport syntax:

- UDP servers as raw IPs/hosts
- DoT servers as `tls://host`
- DoH servers as `https://...`
- DoH3 servers as `h3://...`

This keeps the file aligned with the public CLI syntax instead of leaking internal display names.

### Parsing And Deduplication

Parsing remains line-based with empty lines and `#` comment lines ignored.

Deduplication remains as currently implemented:

- domains: case-insensitive, preserve first occurrence
- servers: protocol plus normalized address, preserve first occurrence

Because the default source is now a single file rather than "built-ins plus file additions", deduplication matters mainly for:

- repeated entries inside the file
- overlap between file-defined servers and detected system DNS
- CLI-provided lists when they contain duplicates

## Error Handling

- Failure to resolve the home directory is an error.
- Failure to create the config directory is an error.
- Failure to create a missing list file is an error.
- Failure to read an existing list file is an error.
- Invalid individual server lines are still skipped, matching current tolerant parsing behavior.
- Empty parsed results from an existing file follow current user-facing validation errors.

## User-Visible Behavior

### First Run

- The program auto-creates both config files with the current built-in defaults.
- The benchmark result is effectively unchanged because the generated files match the prior built-ins.

### Later Runs

- The program reads the files as the default source.
- Editing the files changes the next run immediately.
- Built-in defaults are no longer merged in when the file exists.

### CLI Overrides

- `--domains` ignores `domain.list`
- `--servers` ignores `server.list`
- `--no-system-dns` still controls only system DNS appending

## Testing

Add or update tests for:

- ensuring a missing config directory and file are created
- ensuring generated file content matches the built-in defaults
- ensuring existing files are not overwritten
- distinguishing "file missing" from "file exists but empty"
- using file-only defaults when the file exists
- falling back to built-ins only when the file does not exist
- preserving CLI override behavior
- continuing to deduplicate repeated file entries and system DNS overlap

## Documentation Changes

Update README/help text to reflect:

- first run auto-creates both files
- existing files are never overwritten
- default runtime source is the file when present
- built-ins are only fallback content for missing files

## Tradeoffs

### Why runtime creation instead of install-script-only creation

Runtime creation covers every installation path:

- official install script
- manual binary download
- local builds
- `go install`

That makes the behavior predictable and removes dependence on the distribution path.

### Why not overwrite files on update

Overwriting would destroy user customizations and make the config files untrustworthy as a maintenance surface. Preserving existing files is the safer contract.

## Out Of Scope

- Merging newly added future built-in defaults into already-existing user files
- Syncing comments or metadata into existing files
- Making the config location configurable
