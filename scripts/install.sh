#!/usr/bin/env bash
# Install mevs-dns-checker from GitHub Releases (or build from source with --from-source).
set -euo pipefail

REPO="${REPO:-garymevs/mevs-dns-checker}"
REPO_URL="${REPO_URL:-https://github.com/${REPO}.git}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
BIN_NAME="dnscheck"
APP_ID="mevs-dns-checker"
APP_NAME="DNS Checker"
FROM_SOURCE=0
VERSION="${VERSION:-latest}" # tag like v0.1.0, or "latest"

die() { echo "error: $*" >&2; exit 1; }
info() { echo "==> $*"; }

usage() {
  cat <<EOF
Usage: install.sh [--from-source] [--version <tag>]

  Default: download the latest (or --version) release binary for this platform.
  --from-source   clone/build with Go instead of using Releases
  --version TAG   install a specific release (e.g. v0.1.0); default latest

Env:
  BIN_DIR   install directory (default: ~/.local/bin)
  REPO      GitHub owner/name (default: ${REPO})
EOF
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not found in PATH"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --from-source) FROM_SOURCE=1; shift ;;
    --version)
      [[ $# -ge 2 ]] || die "--version requires a tag"
      VERSION="$2"
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1 (see --help)" ;;
  esac
done

detect_target() {
  local os arch
  os="$(uname -s)"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="x86_64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "unsupported architecture: $arch" ;;
  esac
  case "$os" in
    Linux|Darwin) ;;
    *) die "unsupported OS: $os (use install.ps1 on Windows)" ;;
  esac
  echo "${os}_${arch}"
}

ensure_path() {
  mkdir -p "$BIN_DIR"
  case ":$PATH:" in
    *":$BIN_DIR:"*) return 0 ;;
  esac

  local line="export PATH=\"$BIN_DIR:\$PATH\""
  local rc="$HOME/.profile"
  if [[ "${SHELL:-}" == *zsh ]]; then
    rc="$HOME/.zshrc"
  elif [[ "${SHELL:-}" == *bash ]]; then
    rc="$HOME/.bashrc"
  fi

  touch "$rc"
  if ! grep -Fq "$BIN_DIR" "$rc" 2>/dev/null; then
    info "Adding $BIN_DIR to PATH in $rc"
    printf '\n# mevs-dns-checker\n%s\n' "$line" >>"$rc"
  fi
  export PATH="$BIN_DIR:$PATH"
  echo "Note: open a new shell (or source $rc) so PATH picks up $BIN_DIR"
}

install_icon() {
  local icon_dir="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/scalable/apps"
  mkdir -p "$icon_dir"
  local dest="$icon_dir/${APP_ID}.svg"
  local url="https://raw.githubusercontent.com/${REPO}/master/assets/icon.svg"
  if curl -fsSL "$url" -o "$dest" 2>/dev/null; then
    :
  elif [[ -f "${REPO_DIR:-}/assets/icon.svg" ]]; then
    cp -f "${REPO_DIR}/assets/icon.svg" "$dest"
  else
    return 0
  fi
  if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -f -t "${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor" >/dev/null 2>&1 || true
  fi
}

install_desktop_entry() {
  [[ "$(uname -s)" == "Linux" ]] || return 0

  local apps_dir="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
  mkdir -p "$apps_dir"
  local desktop="$apps_dir/${APP_ID}.desktop"
  local bin_path="$BIN_DIR/$BIN_NAME"
  local exec_line terminal=true

  if command -v omarchy-launch-tui >/dev/null 2>&1 || [[ -x /usr/share/omarchy/bin/omarchy-launch-tui ]]; then
    local launcher
    launcher="$(command -v omarchy-launch-tui 2>/dev/null || echo /usr/share/omarchy/bin/omarchy-launch-tui)"
    exec_line="${launcher} --app-id=org.omarchy.${APP_ID} ${bin_path}"
    terminal=false
    info "Registering Omarchy launcher entry"
  elif command -v xdg-terminal-exec >/dev/null 2>&1; then
    exec_line="xdg-terminal-exec --app-id=org.mevs.${APP_ID} -e ${bin_path}"
    terminal=false
    info "Registering desktop entry via xdg-terminal-exec"
  else
    exec_line="${bin_path}"
    terminal=true
    info "Registering desktop entry (Terminal=true)"
  fi

  cat >"$desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=${APP_NAME}
GenericName=DNS Checker
Comment=Multi-resolver DNS checker with SPF, DKIM, and DMARC
Exec=${exec_line}
Icon=${APP_ID}
Terminal=${terminal}
Categories=Network;Utility;ConsoleOnly;
Keywords=dns;spf;dkim;dmarc;email;propagation;checker;mevs;
StartupNotify=false
EOF

  if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$apps_dir" >/dev/null 2>&1 || true
  fi
  info "Launcher entry: $desktop"
}

release_api_url() {
  if [[ "$VERSION" == "latest" ]]; then
    echo "https://api.github.com/repos/${REPO}/releases/latest"
  else
    echo "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}"
  fi
}

download_release() {
  need_cmd curl
  need_cmd tar

  local target asset api tmp json tag url
  target="$(detect_target)"
  asset="dnscheck_${target}.tar.gz"
  api="$(release_api_url)"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN

  info "Fetching release metadata ($VERSION)"
  if ! json="$(curl -fsSL -H 'Accept: application/vnd.github+json' "$api")"; then
    die "could not fetch release info from $api — publish a tag release, or use --from-source"
  fi

  tag="$(printf '%s' "$json" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
  url="$(printf '%s' "$json" | tr '"' '\n' | grep -E "/${asset}$" | head -1 || true)"
  [[ -n "$url" ]] || die "no asset named $asset in release ${tag:-$VERSION} (platforms: Linux/Darwin x86_64|arm64)"

  info "Downloading ${asset} (${tag})"
  curl -fsSL -o "$tmp/$asset" "$url"
  tar -xzf "$tmp/$asset" -C "$tmp"
  [[ -f "$tmp/$BIN_NAME" ]] || die "archive did not contain $BIN_NAME"

  mkdir -p "$BIN_DIR"
  install -m 755 "$tmp/$BIN_NAME" "$BIN_DIR/$BIN_NAME"
  info "Installed ${tag}: $BIN_DIR/$BIN_NAME"
}

install_from_source() {
  need_cmd git
  need_cmd go

  local src="${REPO_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/mevs-dns-checker/src}"
  REPO_DIR="$src"

  info "Source install from $REPO_URL"
  if [[ -d "$src/.git" ]]; then
    if [[ -n "$(git -C "$src" status --porcelain 2>/dev/null)" ]]; then
      info "Local changes detected — skipping pull, building current tree"
    else
      git -C "$src" fetch --quiet origin
      local branch
      branch="$(git -C "$src" rev-parse --abbrev-ref HEAD)"
      git -C "$src" merge --ff-only --quiet "origin/${branch}" 2>/dev/null \
        || info "Could not fast-forward ${branch}; building current tree"
    fi
  else
    mkdir -p "$(dirname "$src")"
    git clone --depth 1 "$REPO_URL" "$src"
  fi

  info "Building $BIN_NAME"
  mkdir -p "$BIN_DIR"
  (
    cd "$src"
    go build -trimpath -ldflags "-s -w" -o "$BIN_DIR/$BIN_NAME" ./cmd/dnscheck
  )
  info "Installed: $BIN_DIR/$BIN_NAME"
}

main() {
  if [[ "$FROM_SOURCE" -eq 1 ]]; then
    install_from_source
  else
    download_release
  fi

  ensure_path
  install_icon
  install_desktop_entry

  if command -v "$BIN_NAME" >/dev/null 2>&1; then
    "$BIN_NAME" -version 2>/dev/null || "$BIN_NAME" -h | head -2 || true
  else
    "$BIN_DIR/$BIN_NAME" -version 2>/dev/null || "$BIN_DIR/$BIN_NAME" -h | head -2 || true
  fi
  echo
  echo "Try: $BIN_NAME example.com"
  echo "     $BIN_NAME          # interactive TUI"
  if [[ -f "${XDG_DATA_HOME:-$HOME/.local/share}/applications/${APP_ID}.desktop" ]]; then
    echo "Or search for \"${APP_NAME}\" / \"dns\" in your app launcher."
  fi
}

main "$@"
