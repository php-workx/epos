---
name: epos
description: "Use for all ticket operations. Create, query, edit, claim, and close tickets via the epos CLI. Never edit .tickets/ files directly — all mutations must go through epos commands. Triggers: create ticket, show ticket, edit ticket, ready tickets, blocked tickets, claim ticket, release ticket, close ticket, reopen ticket, export tickets, lint tickets, validate ticket, ticket dependencies."
---

# epos — Shared Ticket System CLI

Plain-markdown ticket tracker with YAML frontmatter, stored in `.tickets/`. No databases, no daemons — just files. Designed for multi-agent, multi-repo use.

## Overview

**epos** stores tickets as markdown files with YAML frontmatter in a `.tickets/` directory. Every ticket mutation goes through the CLI — never edit ticket files directly.

Key properties:

- Markdown + YAML: human- and agent-readable, optionally git-tracked
- Sidecar runtime state: claims and leases in `.tickets/.claims/`, never inlined into frontmatter
- Deterministic output: stable YAML serialization, no noisy diffs
- Multi-repo: point `--dir` at any `.tickets/` location

## Hard Rules

These constraints are non-negotiable:

1. **Never write ticket files directly** — no `apply_patch` on `.tickets/` files, no direct markdown edits, no shell redirects into `.tickets/`
2. **Always use the CLI to create or mutate tickets** — every create, update, status change, claim, or release goes through `epos` commands
3. **Validate before handing tickets to execution** — run `epos lint` or `epos validate <id>` before passing tickets to a harness or worker
4. **Prefer structured fields over prose when a field exists** — use `--parent`, `--deps`, `--priority`, `--tags`, `--ac` flags instead of burying metadata in the body
5. **Only enter freeform ticket prose in explicitly designated body sections** — use `--body` for description, `--note` for updates, `--intent` for high-level intent

## Prerequisites

- **epos CLI**: Install from the epos repo via `go install ./cmd/epos`
- **No other dependencies**: `.tickets/` directory is auto-created on first `epos new`

## Quick Reference

```bash
# Find work
epos ready                       # Open tickets with all deps resolved
epos ready <parent>              # Ready children under a parent
epos blocked                     # Tickets with unresolved deps
epos blocked <parent>            # Blocked children under a parent

# Ticket lifecycle
epos new <title> [flags]         # Create ticket, prints ID
epos show <id>                   # Display ticket
epos close <id> [-r <reason>]    # Set status → closed
epos reopen <id> [-r <reason>]   # Set status → open
epos edit <id> [flags]           # Modify ticket fields

# Create/edit flags (shared)
  -t, --type string         type: epic, task, issue, feature, bug, chore, spike, doc
  -p, --priority int        priority (higher = more important)
      --parent string       parent ticket ID
      --deps strings        comma-separated dependency IDs
      --tags strings        comma-separated tags
      --ac stringArray      acceptance criterion (repeatable)
      --assignee string     assignee name
      --body string         ticket description body
      --body-file string    path to file with body content
      --intent string       high-level intent
      --note stringArray    note to append (repeatable)
      --stdin               read spec as JSON from stdin

# Validation
epos validate <id>               # Validate a single ticket
epos lint                        # Validate all tickets in store

# Claims
epos claim <id> -o <agent>       # Claim a ticket for an agent
epos release <id> -o <agent>     # Release a claim

# Export
epos export [parent]             # Export tickets as JSON
```

## Examples

### Creating a New Task

```bash
epos new "Fix auth token validation" \
  --parent epo-abc123 \
  --priority 2 \
  --type bug \
  --body "The token validation middleware does not handle expired tokens correctly. \
Returns 500 instead of 401."
# Prints: epo-d4f7
```

### Checking What's Ready

```bash
epos ready --json
# Returns JSON array of ready tickets with deps resolved
```

```bash
epos ready epo-abc123 --json
# Returns ready children under the given parent
```

### Claiming and Closing a Ticket

```bash
epos claim epo-d4f7 -o agent-1
# Claim recorded in .tickets/.claims/epo-d4f7.json

epos close epo-d4f7 -r "Fixed token validation; added expiry check middleware"
# Ticket status → closed
```

## The `--json` Flag

Always prefer `--json` for machine parsing. Supported by all non-interactive CLI commands (skill command surface); not accepted by interactive commands such as `epos tui`.

```bash
epos show epo-d4f7 --json     # Structured output for scripts
epos ready --json             # Array of ready tickets
epos export --json            # Full JSON dump
```

Without `--json`, output is formatted for human readability.
**Important:** In JSON mode, `epos lint --json` and `epos validate --json` exit with code `0` even when validation errors exist. Always inspect the JSON payload — never rely on exit codes for validation results in JSON mode.

## Multi-Repo Usage

Point `--dir` (`-d`) at any `.tickets/` location to work across repos:

```bash
epos --dir ../other-project ready --json
epos -d /path/to/repo show epo-d4f7
```

Default is the current directory.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `epos: command not found` | CLI not installed | `go install ./cmd/epos` from the epos repo |
| `no .tickets directory found` | Store not initialized | Run `epos new "Title"` to auto-create, or `mkdir .tickets` |
| `ambiguous partial ID` | Short ID matches multiple tickets | Provide more characters of the full ID |
| `validation failed` | Ticket violates schema rules | Run `epos validate <id>` for detailed errors; fix with `epos edit` |
| `claim failed: ticket already claimed` | Another agent holds the claim | Check with `epos show <id> --json`; wait for release or expiry |

## Anti-Patterns

- **`apply_patch` on `.tickets/` files** — never patch ticket files; use `epos edit <id>` instead
- **Direct YAML editing** — no `sed`, `awk`, or editor on frontmatter; use `epos edit`
- **Shell redirects into `.tickets/`** — no `cat > .tickets/epo-xxx.md`; use `epos new` or `epos edit --body-file`
- **Burying metadata in prose** — don't write "depends on epo-abc" in the body; use `--deps epo-abc`
- **Skipping validation before execution** — always run `epos lint` or `epos validate` before handing tickets to workers
- **Mixing runtime state into frontmatter** — not even manually; claims and leases live in sidecars
