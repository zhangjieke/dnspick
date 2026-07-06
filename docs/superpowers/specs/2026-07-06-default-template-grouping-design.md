# Default Template Grouping Design

## Goal

Improve the readability and maintainability of the auto-generated default
configuration files under `~/.config/dnspick` by grouping and sorting their
contents for humans.

At the same time, update the built-in default domain set to match the
maintained domain template currently in use, while keeping the built-in default
server set general-purpose and not tied to machine-local assumptions.

## Requirements

### Functional

- Update `dnsbench.DefaultDomains` to include the additional maintained domain
  entries currently present in the user's domain list.
- Do not add `127.0.0.1` to `dnsbench.DefaultServers`.
- Keep `dnsbench.DefaultServers` suitable for general users without requiring a
  locally running DNS service.
- When generating the default `domain.list` template, output:
  - domestic domains
  - one blank line
  - foreign domains
- When generating the default `server.list` template, output:
  - UDP entries
  - one blank line
  - DoT entries
  - one blank line
  - DoH entries
  - one blank line
  - DoH3 entries
- Within each group, sort entries lexicographically.

### Scope

- The grouping/sorting requirement applies to generated default config file
  content only.
- Runtime benchmark execution order does not need to be changed to match the
  grouped template layout.
- Existing user config files must not be rewritten.

## Default Domain Set Update

The built-in default domains should be updated to include these domestic domains:

- `163.com`
- `58.com`
- `aliyun.com`
- `autohome.com.cn`
- `baidu.com`
- `bilibili.com`
- `douyin.com`
- `gov.cn`
- `jd.com`
- `jianshu.com`
- `meituan.com`
- `qq.com`
- `taobao.com`
- `weibo.com`
- `zhihu.com`

And these foreign domains:

- `anthropic.com`
- `apple.com`
- `bing.com`
- `chatgpt.com`
- `cloudflare.com`
- `facebook.com`
- `frenchystyle.com`
- `github.com`
- `google.com`
- `quora.com`
- `reddit.com`
- `tiktok.com`
- `x.com`
- `youtube.com`

These become the new built-in defaults used when config files are missing and
the initial template content for newly created `domain.list`.

## Server Default Policy

Although the current local `server.list` includes `127.0.0.1`, that entry must
not be added to `dnsbench.DefaultServers`.

Reason:

- it assumes a local DNS service is running
- that assumption is not valid for general users
- including it in built-in defaults would create misleading benchmark results
  or guaranteed failures for many users

The local config file may still contain `127.0.0.1`; this change does not alter
user-owned config files.

## Design

### Template Export Functions

The current template export functions in `main.go` flatten defaults directly in
their source order.

Replace that behavior with explicit formatting helpers:

- `defaultDomainEntries()`
  - partition by `CategoryDomestic` and `CategoryForeign`
  - sort each group lexicographically
  - join groups with one empty-line separator

- `defaultServerEntries()`
  - convert servers into their config-file entry form
  - partition by protocol: UDP, DoT, DoH, DoH3
  - sort each group lexicographically
  - join groups with one empty-line separator

The helpers should return a flat `[]string`, where empty strings represent
group separators. The existing config-file creation path already writes the
provided lines as-is, so this integrates without changing file-writing policy.

### Formatting Rules

#### `domain.list`

Example shape:

```text
163.com
58.com
aliyun.com
...
zhihu.com

anthropic.com
apple.com
...
youtube.com
```

#### `server.list`

Example shape:

```text
1.0.0.1
1.1.1.1
...
223.6.6.6

tls://dns.alidns.com
...

https://cloudflare-dns.com/dns-query
...

h3://cloudflare-dns.com/dns-query
...
```

### Runtime Behavior

No change to runtime parsing semantics:

- blank lines are already ignored
- existing files are still never overwritten
- benchmark source selection logic is unchanged

This is a template-maintainability improvement plus a built-in default-domain
refresh.

## Testing

Add or update tests for:

- `defaultDomainEntries()` returns domestic + blank + foreign layout
- each domain group is sorted lexicographically
- `defaultServerEntries()` returns UDP + blank + DoT + blank + DoH + blank + DoH3 layout
- each server group is sorted lexicographically
- `127.0.0.1` is not present in generated default server entries
- the updated built-in domain set contains the intended additions

No config read/write behavior changes need separate documentation or broad test
changes beyond the export helpers.

## Documentation Changes

README changes are not required for this iteration because user-visible feature
behavior is unchanged. The default generated files simply become easier to edit.

## Out Of Scope

- Rewriting existing user config files
- Changing runtime benchmark ordering to match template ordering
- Adding comment headers to the generated templates
- Adding `127.0.0.1` to built-in default servers
