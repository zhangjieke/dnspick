# Failure Report Design

## Goal

Make it easy to diagnose why a DNS server has a lower success rate even when
its average latency looks good.

When benchmark queries fail, dnspick should write a plain-text failure report
into the system temp directory so the user can inspect which domains failed and
what errors occurred for each DNS server.

The report must:

- be generated only when at least one measured query fails
- live in the system temp directory (`os.TempDir()`, typically `/tmp`)
- group failures by DNS server
- be easy to search with tools like `grep` or `less`

## Current State

Today the benchmark pipeline only keeps aggregate success statistics:

- successes
- total queries
- average latency
- composite score

Per-query failure context is discarded after aggregation. As a result, the user
can see that a server has a lower success rate, but cannot tell:

- which domains failed
- how often each domain failed
- whether the failures were timeouts, HTTP errors, DNS response code errors, or
  "unreachable" short-circuit errors

## Requirements

### Functional

- Preserve failure detail for each measured query.
- Generate one report file per benchmark run when failures occurred.
- Group entries by DNS server.
- Within each server group, group failures by domain.
- Within each domain group, summarize the error strings and occurrence counts.
- Print the report path at the end of the run when a report was written.
- Do not generate a report when every measured query succeeds.

### Output Format

The report should be plain text, optimized for searchability rather than fancy formatting.

Recommended structure:

1. Run header
2. One section per server
3. Per-server summary counts
4. Per-domain failure counts
5. Per-domain error breakdown

Example shape:

```text
dnspick failure report
generated_at: 2026-07-06T15:30:45+08:00

server: Cloudflare (DoH3)
address: https://cloudflare-dns.com/dns-query
protocol: doh3
total_failures: 2

  domain: google.com
  failures: 1
    error[1]: HTTP status 503

  domain: youtube.com
  failures: 1
    error[1]: server unreachable; remaining queries skipped
```

### Scope

- Record measured-query failures only.
- Do not include the warm-up query in the report, because it is not part of the
  success-rate statistics shown to the user.
- Keep JSON output unchanged for now.

## Design

### Benchmark Data Model

Extend the benchmark pipeline so measured-query failures retain enough context
to produce a report later.

#### `queryResult`

Add:

- `domain string`

This lets aggregation know which domain each measured query belonged to.

#### Failure records

Introduce a lightweight exported failure structure, for example:

- `FailureRecord`
  - server name
  - address
  - protocol
  - domain
  - error text
  - system-DNS flag if useful for completeness

The error is stored as text rather than as an `error` value so the reporting
layer remains serialization-friendly and detached from transport-specific types.

### Benchmark API Shape

To minimize disruption, keep the existing simple API and add a richer one.

- Keep `Run(opts, progress) []Result`
- Add `RunDetailed(opts, progress) RunOutput`

Suggested structure:

- `RunOutput`
  - `Results []Result`
  - `Failures []FailureRecord`

`Run` becomes a thin wrapper around `RunDetailed`.

That keeps existing callers stable while allowing `main.go` to access the
failure report data.

### Aggregation

The current aggregation pass already reads every measured query. Extend it so
that when `result.err != nil` it also appends a failure record capturing:

- server metadata
- domain
- normalized error string

Success aggregation remains unchanged.

### Report Writer

Add a small reporting helper outside `dnsbench`, because writing files is an
output concern, not a benchmarking concern.

Recommended placement:

- `internal/ui/failurereport.go`

Responsibilities:

- accept `[]dnsbench.FailureRecord`
- group by server
- group by domain
- count repeated error strings
- write a timestamped text file under `os.TempDir()`
- return the generated path

Suggested API:

- `WriteFailureReport(failures []dnsbench.FailureRecord, now time.Time) (string, error)`

The `now` argument makes the filename deterministic in tests.

### File Naming

Use a timestamped filename to avoid overwriting earlier runs:

- `dnspick-failures-YYYYMMDD-HHMMSS.txt`

Place it under:

- `filepath.Join(os.TempDir(), filename)`

### Main Flow

`main.go` changes:

1. Run benchmark through `RunDetailed`.
2. Print results and recommendations as before.
3. If failures exist:
   - write the report
   - print a one-line notice with the path
4. If report writing fails:
   - do not fail the benchmark after results are already computed
   - print a short warning instead

This keeps diagnosis additive and does not hide the main benchmark outcome.

## Error Handling

### Report Creation Failure

If the temp file cannot be written:

- benchmark results should still be printed
- report failure should be surfaced as a warning

This avoids turning a successful benchmark into an overall command failure only
because temp-file output was unavailable.

### Empty Failure Set

If there are zero failures:

- no report is written
- no extra path message is printed

## Testing

Add or update tests for:

- `queryResult` carrying the domain through aggregation
- `RunDetailed` returning both aggregate results and failure records
- report writer grouping failures by server and domain
- report writer counting repeated error strings
- report writer producing a deterministic filename when given a fixed time
- no report creation when there are no failures
- main flow behavior when report writing succeeds
- main flow behavior when report writing fails

Keep existing success-rate and score tests intact.

## Documentation Changes

At minimum, user-visible output should make the new behavior discoverable:

- print the report path when generated

README changes are optional for this iteration unless the new feature becomes a
documented troubleshooting workflow.

## Tradeoffs

### Why not print all failure detail directly to stdout

Directly dumping failures into the terminal would make normal runs noisy and
harder to scan, especially when many servers fail across many domains.

Writing a text report to the temp directory keeps the main output concise while
still preserving diagnosability.

### Why not add failure detail to JSON now

That would widen the machine-readable contract and increase scope. The current
goal is human debugging for unexpectedly low success rates.

## Out Of Scope

- Extending the JSON schema with failure details
- Changing recommendation/scoring logic
- Recording warm-up query failures in the user-facing report
