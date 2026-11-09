# mevs-dns-checker

TUI DNS checker inspired by [dnschecker.org](https://dnschecker.org): query a domain across many public resolvers, or inspect SPF / DKIM / DMARC via the **EMAIL** tab.

## Features

- Concurrent lookups against Cloudflare, Google, Quad9, OpenDNS, AdGuard, and more
- Record types: **A**, **AAAA**, **CNAME**, **MX**, **NS**, **TXT**, **EMAIL**
- Consensus view highlighting resolver disagreements (propagation / cache drift)
- EMAIL tab: SPF, DMARC, DKIM (common selectors or your own)
- CLI args for one-shot checks, or no args for interactive TUI
- `-plain` for script-friendly text output

> This approximates multi-location checks by talking to well-known public resolvers. It is not a distributed geo-probe network.

## Install

```bash
go install github.com/garymevs/mevs-dns-checker/cmd/dnscheck@latest
```

Or from this repo:

```bash
go build -o dnscheck ./cmd/dnscheck
```

## Usage

```bash
dnscheck                         # interactive TUI
dnscheck example.com             # A records across resolvers
dnscheck example.com -t MX
dnscheck example.com -t EMAIL
dnscheck example.com -dkim-selector google,selector1
dnscheck example.com -t EMAIL -plain
```

### Flags

| Flag | Description |
|------|-------------|
| `-t` / `-type` | `A`, `AAAA`, `CNAME`, `MX`, `NS`, `TXT`, `EMAIL` (default `A`) |
| `-dkim-selector` | Comma-separated selectors (selects `EMAIL`) |
| `-timeout` | Per-resolver timeout seconds (default `3`) |
| `-plain` | Text output instead of TUI |

### TUI keys

| Key | Action |
|-----|--------|
| `enter` | Run check (form) |
| `tab` / `shift+tab` | Change type (includes EMAIL) |
| `←` `→` | Change type (results) |
| `r` | Recheck (results) |
| `i` / `esc` | Back to domain input |
| `q` | Quit (results; form uses `ctrl+c`) |
| `ctrl+c` | Quit |

## Layout

```
cmd/dnscheck/          entrypoint
internal/cli/          flag parsing
internal/dns/          multi-resolver client
internal/resolvers/    public resolver list
internal/email/        SPF / DKIM / DMARC
internal/tui/          Bubble Tea UI + plain renderer
```
