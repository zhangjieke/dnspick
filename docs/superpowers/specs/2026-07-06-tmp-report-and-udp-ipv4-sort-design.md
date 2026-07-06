# Tmp Report And UDP IPv4 Sort Design

## Goal

Refine two human-facing details introduced by the recent reporting and template
formatting work:

- failure reports should prefer `/tmp` so users can find them easily
- UDP entries in the generated default `server.list` should sort IPv4 addresses
  numerically by octet, instead of by raw string comparison

## Requirements

### Failure report path

- Prefer writing failure reports under `/tmp`.
- Keep the existing filename format:
  - `dnspick-failures-YYYYMMDD-HHMMSS.txt`
- If `/tmp` is unavailable or cannot be used, fall back to `os.TempDir()`.
- Print the actual path used.

### UDP sorting

- In generated default `server.list` content, UDP IPv4 entries must be sorted
  numerically by octet.
- Example order:
  - `1.0.0.1`
  - `1.1.1.1`
  - `8.8.4.4`
  - `8.8.8.8`
  - `9.9.9.9`
- DoT / DoH / DoH3 groups continue using simple lexicographic sorting.

## Design

### Failure report output directory

Change the failure report writer to try:

1. `/tmp`
2. `os.TempDir()` fallback

The implementation should validate writability by attempting the final write,
not by assuming `/tmp` is always usable.

### UDP IPv4 sort

Replace the current string sort for the UDP group with a comparator:

- if both items parse as IPv4, compare octets numerically
- otherwise fall back to string comparison

This change applies only to generated default template content, not runtime
benchmark ordering.

## Testing

Add or update tests for:

- failure report path is under `/tmp` when available
- failure report still succeeds through fallback logic when `/tmp` cannot be used
- UDP default template entries are ordered by IPv4 octets
- DoT / DoH / DoH3 template groups remain lexicographically sorted

## Out Of Scope

- Changing runtime benchmark server ordering
- Changing failure report file format
