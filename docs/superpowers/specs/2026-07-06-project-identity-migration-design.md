# Project Identity Migration Design

## Goal

Migrate the repository's active project identity from the upstream source
repository to this fork, while preserving the project name `dnspick`.

The runtime, build, install, update, and documentation paths must point to:

- `github.com/zhangjieke/dnspick`

At the same time:

- `LICENSE` remains unchanged
- the README files include a short explicit fork-attribution line for
  `palemoky/dnspick`

## Current State

The repository remote already points to:

- `git@github.com:zhangjieke/dnspick.git`

However, multiple parts of the codebase still identify the project as the
upstream repository:

- `go.mod` module path
- Go source imports
- `Makefile` build-info injection package path
- installer scripts
- updater GitHub owner/repo constants
- README badges, install commands, and release links

This creates an inconsistent identity:

- building from this repo still uses the upstream module path
- installation scripts download from upstream releases
- self-update checks query upstream releases
- documentation points users to upstream assets

## Requirements

### Functional

- Change the Go module path to `github.com/zhangjieke/dnspick`.
- Update all internal imports to the new module path.
- Update `Makefile` package injection paths accordingly.
- Update `install.sh` to download from `zhangjieke/dnspick`.
- Update `install.ps1` to download from `zhangjieke/dnspick`.
- Update updater GitHub release owner/repo constants to the fork.
- Update README installation commands, release links, badges, and report-card links to the fork.
- Keep the project name, binary name, and release asset names as `dnspick`.

### Attribution

- `LICENSE` must remain untouched.
- `README.md` and `README.zh-CN.md` must each include a short fork-origin note naming `palemoky/dnspick`.
- Outside those explicit fork-origin notes and the unchanged license, upstream repository identity should not remain in active build/runtime/install paths.

## Design

### Identity Surface Areas

The migration affects five main surfaces.

#### 1. Module And Imports

- `go.mod` becomes `module github.com/zhangjieke/dnspick`
- all imports of `github.com/palemoky/dnspick/...` become `github.com/zhangjieke/dnspick/...`

This ensures local builds, tests, and future external references align with the fork identity.

#### 2. Build Metadata Injection

`Makefile` uses a package path constant for linker-injected version variables.

That path must be updated to:

- `github.com/zhangjieke/dnspick/internal/buildinfo`

Otherwise `go build -ldflags ...` would point at a package path that no longer exists.

#### 3. Install And Download Entry Points

`install.sh` and `install.ps1` currently construct download URLs and raw-script examples from the upstream repository.

They must be switched to the fork repository while preserving:

- `dnspick` binary name
- current release asset naming pattern
- current install locations and platform detection behavior

#### 4. Runtime Self-Update

`internal/updater/updater.go` currently queries and downloads releases from the upstream GitHub repository.

The GitHub `owner` and `repo` constants must be updated to the fork so that:

- `dnspick update`
- background update checks

both resolve against the fork's GitHub Releases.

#### 5. Documentation And Badges

README files must be updated so user-facing links point to the fork:

- GitHub Actions badge
- Go Report Card badge
- install-script raw URLs
- GitHub Releases links

Both README files also gain a short fork-origin line near the top, for example:

- English: `This project is a fork of palemoky/dnspick.`
- Chinese: `本项目基于 palemoky/dnspick fork。`

That keeps attribution explicit without leaving upstream identity embedded in operational instructions.

## Implementation Strategy

1. Change `go.mod`.
2. Update all internal imports to the new module path.
3. Update `Makefile`.
4. Update installer scripts.
5. Update updater repository constants.
6. Update README links and add fork-origin notes.
7. Run a repository-wide search to confirm upstream identity remains only where intentionally preserved.

This order reduces temporary broken states while editing.

## Error Handling And Compatibility

### Module Path Compatibility

Changing the module path is a deliberate compatibility break for anyone trying to import this fork using the upstream module path.

That is acceptable and expected because the goal is to establish the fork as an independent active project identity.

### Release Asset Compatibility

Installer and updater logic continue to assume the existing asset naming convention:

- `dnspick-<os>-<arch>.tar.gz`
- `dnspick-windows-<arch>.zip`

The fork's release pipeline must continue publishing assets with those names.

### License Safety

Leaving `LICENSE` untouched avoids introducing unnecessary licensing risk or accidental attribution regressions.

## Testing

Verification should include:

- `go test ./...`
- repository-wide search confirming active paths no longer reference `palemoky/dnspick`
- manual inspection of installer and updater URL construction

Allowed remaining upstream references:

- the explicit README fork-origin note
- the unchanged `LICENSE`

## Documentation Changes

Update both README files so that:

- badges point to the fork
- install commands fetch scripts from the fork
- releases links point to the fork
- a short fork-origin note is present

No broader branding rewrite is required beyond identity-correct links and the concise fork note.

## Out Of Scope

- Editing the license text
- Renaming the project or binary
- Changing release asset naming
- Editing GitHub-hosted settings outside files in this repository
- Rewriting broader product copy beyond identity-correct links and minimal fork attribution
