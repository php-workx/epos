#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

EPOS_INSTALL_LIB_ONLY=1 source "$repo_root/scripts/install.sh"

fail() {
  echo "install_test: $*" >&2
  exit 1
}

assert_eq() {
  local got="$1"
  local want="$2"
  local label="$3"
  if [ "$got" != "$want" ]; then
    fail "$label: got '$got', want '$want'"
  fi
}

assert_eq "$(epos_archive_name "v1.2.3" "darwin" "arm64")" "epos_1.2.3_darwin_arm64.tar.gz" "darwin arm64 archive"
assert_eq "$(epos_archive_name "v1.2.3" "darwin" "amd64")" "epos_1.2.3_darwin_amd64.tar.gz" "darwin amd64 archive"
assert_eq "$(epos_archive_name "v1.2.3" "linux" "amd64")" "epos_1.2.3_linux_amd64.tar.gz" "linux amd64 archive"

checksum_file="$(mktemp)"
trap 'rm -f "$checksum_file"' EXIT
cat > "$checksum_file" <<'EOF'
abc123  epos_1.2.3_darwin_arm64.tar.gz
def456 *epos_1.2.3_linux_amd64.tar.gz
EOF

assert_eq "$(epos_checksum_for "$checksum_file" "epos_1.2.3_darwin_arm64.tar.gz")" "abc123" "plain checksum lookup"
assert_eq "$(epos_checksum_for "$checksum_file" "epos_1.2.3_linux_amd64.tar.gz")" "def456" "star checksum lookup"

echo "install_test: ok"
