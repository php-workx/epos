#!/usr/bin/env bash
set -euo pipefail

repo="php-workx/epos"

log() {
  printf '==> %s\n' "$*" >&2
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

download_file() {
  local url="$1"
  local out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$out" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$out" "$url"
  else
    fail "curl or wget is required"
  fi
}

sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | awk '{print $2}'
  else
    fail "sha256sum, shasum, or openssl is required"
  fi
}

epos_archive_name() {
  local tag="$1"
  local os="$2"
  local arch="$3"
  local version="${tag#v}"
  local ext="tar.gz"
  if [ "$os" = "windows" ]; then
    ext="zip"
  fi
  printf 'epos_%s_%s_%s.%s\n' "$version" "$os" "$arch" "$ext"
}

epos_checksum_for() {
  local checksums_file="$1"
  local archive_name="$2"
  awk -v target="$archive_name" '{ name=$2; sub(/^\*/, "", name); if (name == target) { print $1; exit } }' "$checksums_file"
}

detect_platform() {
  local os arch
  case "$(uname -s)" in
    Darwin) os="darwin" ;;
    Linux) os="linux" ;;
    MINGW*|MSYS*|CYGWIN*)
      fail "Windows detected. Use: irm https://raw.githubusercontent.com/${repo}/main/install.ps1 | iex"
      ;;
    *) fail "unsupported OS: $(uname -s)" ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac

  printf '%s %s\n' "$os" "$arch"
}

latest_tag() {
  local json
  json="$(mktemp)"
  download_file "https://api.github.com/repos/${repo}/releases/latest" "$json"
  sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$json" | head -1
  rm -f "$json"
}

install_epos() {
  local tag="${EPOS_VERSION:-}"
  if [ -z "$tag" ]; then
    log "Fetching latest epos release"
    tag="$(latest_tag)"
  fi
  [ -n "$tag" ] || fail "could not determine latest release"

  read -r os arch < <(detect_platform)
  local archive
  archive="$(epos_archive_name "$tag" "$os" "$arch")"
  local base_url="https://github.com/${repo}/releases/download/${tag}"

  local tmp
  tmp="$(mktemp -d)"
  trap "rm -rf '$tmp'" EXIT

  log "Downloading ${archive}"
  download_file "${base_url}/${archive}" "${tmp}/${archive}"
  download_file "${base_url}/checksums.txt" "${tmp}/checksums.txt"

  local expected actual
  expected="$(epos_checksum_for "${tmp}/checksums.txt" "$archive")"
  [ -n "$expected" ] || fail "no checksum found for ${archive}"
  actual="$(sha256_file "${tmp}/${archive}")"
  [ "$expected" = "$actual" ] || fail "checksum mismatch for ${archive}"
  log "Checksum verified"

  mkdir -p "${tmp}/extract"
  tar -xzf "${tmp}/${archive}" -C "${tmp}/extract"
  [ -x "${tmp}/extract/epos" ] || fail "archive did not contain executable epos"

  local install_dir="${EPOS_INSTALL_DIR:-}"
  if [ -z "$install_dir" ]; then
    if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
      install_dir="/usr/local/bin"
    else
      install_dir="${HOME}/.local/bin"
    fi
  fi
  mkdir -p "$install_dir"
  mv "${tmp}/extract/epos" "${install_dir}/epos"
  chmod +x "${install_dir}/epos"
  log "Installed epos to ${install_dir}/epos"

  if [[ ":$PATH:" != *":${install_dir}:"* ]]; then
    log "${install_dir} is not on PATH"
  fi
}

if [ "${EPOS_INSTALL_LIB_ONLY:-0}" != "1" ]; then
  install_epos "$@"
fi
