#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SKILL_NAME="epos"

SRC_SKILL="$REPO_ROOT/skill/$SKILL_NAME"

if [[ ! -d "$SRC_SKILL" ]]; then
  echo "error: missing source skill directory at $SRC_SKILL" >&2
  exit 1
fi

if [[ -d "$HOME/.claude/skills" ]]; then
  DEST_CLAUDE_BASE="${CLAUDE_SKILLS_DIR:-$HOME/.claude/skills}"
else
  DEST_CLAUDE_BASE="${CLAUDE_SKILLS_DIR:-$HOME/.agents/skills}"
fi
DEST_CODEX_BASE="${CODEX_SKILLS_DIR:-$HOME/.codex/skills}"

DEST_CLAUDE="$DEST_CLAUDE_BASE/$SKILL_NAME"
DEST_CODEX="$DEST_CODEX_BASE/$SKILL_NAME"

mkdir -p "$DEST_CLAUDE_BASE" "$DEST_CODEX_BASE"

# Atomic install: copy to temp dirs first, then move into place so a failed
# copy never leaves the destination in a broken state.
DEST_CLAUDE_TMP="${DEST_CLAUDE}.installing"
DEST_CODEX_TMP="${DEST_CODEX}.installing"
rm -rf "$DEST_CLAUDE_TMP" "$DEST_CODEX_TMP"

cp -R "$SRC_SKILL" "$DEST_CLAUDE_TMP" || {
  echo "error: failed to copy skill to Claude destination" >&2
  rm -rf "$DEST_CLAUDE_TMP"
  exit 1
}
cp -R "$SRC_SKILL" "$DEST_CODEX_TMP" || {
  echo "error: failed to copy skill to Codex destination" >&2
  rm -rf "$DEST_CLAUDE_TMP" "$DEST_CODEX_TMP"
  exit 1
}

# Atomic swap with rollback: back up existing destinations so a partial
# mv failure can be recovered.
DEST_CLAUDE_BAK="${DEST_CLAUDE}.bak"
DEST_CODEX_BAK="${DEST_CODEX}.bak"
rm -rf "$DEST_CLAUDE_BAK" "$DEST_CODEX_BAK"

# Rename existing → .bak (ok if they don't exist — mv fails silently)
mv "$DEST_CLAUDE" "$DEST_CLAUDE_BAK" 2>/dev/null || true
mv "$DEST_CODEX" "$DEST_CODEX_BAK" 2>/dev/null || true

# Move each tmp into place; on failure, restore from .bak
mv "$DEST_CLAUDE_TMP" "$DEST_CLAUDE" || {
  echo "error: failed to install Claude skill; restoring backup" >&2
  mv "$DEST_CLAUDE_BAK" "$DEST_CLAUDE" 2>/dev/null || true
  mv "$DEST_CODEX_BAK" "$DEST_CODEX" 2>/dev/null || true
  exit 1
}
mv "$DEST_CODEX_TMP" "$DEST_CODEX" || {
  echo "error: failed to install Codex skill; restoring backup" >&2
  mv "$DEST_CODEX_BAK" "$DEST_CODEX" 2>/dev/null || true
  # Claude install is already live at this point — keep it
  exit 1
}

# Clean up backups on success
rm -rf "$DEST_CLAUDE_BAK" "$DEST_CODEX_BAK"

cat <<EOF
Installed $SKILL_NAME:
- Claude: $DEST_CLAUDE
- Codex:  $DEST_CODEX
EOF
