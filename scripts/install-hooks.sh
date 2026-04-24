#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
git -C "$repo_root" config --unset core.hooksPath 2>/dev/null || true
hooks_dir="$(cd "$repo_root" && git rev-parse --path-format=absolute --git-path hooks 2>/dev/null)" ||
    hooks_dir="$repo_root/.git/hooks"

mkdir -p "$hooks_dir"

for hook in pre-commit pre-push; do
    src="$script_dir/$hook"
    dst="$hooks_dir/$hook"
    if [ ! -f "$src" ] || [ ! -r "$src" ]; then
        echo "missing hook source: $src" >&2
        exit 1
    fi
    cp "$src" "$dst"
    chmod +x "$dst"
    echo "Installed $hook hook."
done

echo "Done. Git hooks installed in $hooks_dir."