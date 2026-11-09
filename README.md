# mevs-dns-checker

> **Vibecoded slop disclaimer:** this was AI-assisted / vibecoded. Treat it as experimental software with sharp edges. You are responsible for what you do with any information it shows, and you should confirm results against other sources before acting on them.

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

Installers download the **latest GitHub Release** binary for your platform, put it on `PATH`, and (on Linux/Omarchy) register a searchable **DNS Checker** launcher entry.

### Linux / macOS / Omarchy

```bash
curl -fsSL https://raw.githubusercontent.com/garymevs/mevs-dns-checker/master/scripts/install.sh | bash
```

Re-run anytime to upgrade. Options:

```bash
# specific tag
curl -fsSL .../install.sh | bash -s -- --version v0.1.0

# build from source instead (needs git + Go)
curl -fsSL .../install.sh | bash -s -- --from-source
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/garymevs/mevs-dns-checker/master/scripts/install.ps1 | iex
```

```powershell
.\scripts\install.ps1 -Version v0.1.0
.\scripts\install.ps1 -FromSource
```

### Manual / developers

```bash
go install github.com/garymevs/mevs-dns-checker/cmd/dnscheck@latest
# or
go build -o dnscheck ./cmd/dnscheck
```

## Releasing

Tag a version and push — GitHub Actions builds multi-platform archives and publishes a Release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Assets: `dnscheck_Linux_x86_64.tar.gz`, `dnscheck_Linux_arm64.tar.gz`, `dnscheck_Darwin_x86_64.tar.gz`, `dnscheck_Darwin_arm64.tar.gz`, `dnscheck_Windows_x86_64.zip`, plus `checksums.txt`.

## Usage

```bash
dnscheck                         # interactive TUI
dnscheck example.com             # A records across resolvers
dnscheck example.com -t MX
dnscheck example.com -t EMAIL
dnscheck example.com -dkim-selector google,selector1
dnscheck example.com -t EMAIL -plain
dnscheck -version
```

### Flags

| Flag | Description |
|------|-------------|
| `-t` / `-type` | `A`, `AAAA`, `CNAME`, `MX`, `NS`, `TXT`, `EMAIL` (default `A`) |
| `-dkim-selector` | Comma-separated selectors (selects `EMAIL`) |
| `-timeout` | Per-resolver timeout seconds (default `3`) |
| `-plain` | Text output instead of TUI |
| `-V` / `-version` | Print version |

### TUI keys

| Key | Action |
|-----|--------|
| `enter` | Run check |
| `tab` / `shift+tab` | Change type (does not auto-query) |
| `←` `→` | Change type on results (does not auto-query) |
| `r` | Recheck current/selected type (results) |
| `s` | Set DKIM selectors, then re-run EMAIL (results) |
| `↑` `↓` / PgUp / PgDn | Scroll results |
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
scripts/               install.sh / install.ps1
.github/workflows/     release builds
```
