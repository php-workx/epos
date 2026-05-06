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
4. **Use structured fields over prose** — `--parent`, `--deps`, `--priority`, `--tags`, `--ac`, `--assignee`, `--intent` flags exist for a reason; freeform prose only in `--body` and `--note`

## Detailed Guides

- Multi-step workflows (session start, epic-with-children, claim→work→close, dependency edits, multi-repo, post-compaction recovery): see [references/WORKFLOWS.md](references/WORKFLOWS.md)
- Full anti-pattern catalog with rationale and concrete wrong/right examples: see [references/ANTI_PATTERNS.md](references/ANTI_PATTERNS.md)

Read these references when the inline summary below is insufficient.

## Prerequisites

- **epos CLI**: install from the epos repo via `go install ./cmd/epos` (or use a published binary release once available)
- **Verify**: `epos --version` should print a version string
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

# Common create/edit flags (run `epos new --help` for full list)
  -t, --type           epic|task|issue|feature|bug|chore|spike|doc
  -p, --priority       int (higher = more important)
      --parent         parent ticket ID
      --deps           comma-separated dependency IDs
      --tags           comma-separated tags
      --ac             acceptance criterion (repeatable)
      --body           description (or --body-file <path>, or --stdin for JSON)
      --note           append note (repeatable)

# Validation
epos validate <id>               # Validate a single ticket
epos lint                        # Validate all tickets in store

# Claims
epos claim <id> -o <agent>       # Claim a ticket for an agent
epos release <id> -o <agent>     # Release a claim

# Export
epos export [parent]             # Export tickets as JSON
```

## Minimal Example

Always capture the new ID via `--json | jq -r .id` — never parse human-formatted output.

```bash
id=$(epos new "Fix auth token validation" --type bug --priority 2 --parent epo-abc123 \
  --body "Token middleware returns 500 on expired tokens; should return 401." \
  --json | jq -r .id)

epos claim "$id" -o "$AGENT_ID" || { echo "claim failed; pick another"; exit 0; }
epos validate "$id" --json       # inspect payload before working
# ... do the work ...
epos close "$id" -r "Added expiry check middleware"
```

For epic-with-children, dependency edits, post-compaction recovery, and unhappy paths (claim conflict, blocker, validation fail), see [references/WORKFLOWS.md](references/WORKFLOWS.md).

## Claim Semantics

Claims are how multiple agents coordinate. Get these right:

- **Owner ID**: pick a stable string per agent session and reuse it. The CLI does not generate one. Convention: `agent-<role>-<short-session-id>` (e.g. `agent-impl-3f7a`). Set once as `AGENT_ID=...`, pass via `-o "$AGENT_ID"` to every `claim` and `release`.
- **`--assignee` vs `-o`**: `--assignee` is metadata in the ticket frontmatter (display only). `-o <owner>` is the lock holder in the sidecar. They are independent — claim with `-o`, optionally also set `--assignee` for human readability.
- **Lease duration**: 15 minutes from the last `epos claim` call. After expiry another agent can claim. There is no separate `epos renew` command — re-running `epos claim <id> -o "$AGENT_ID"` with the same owner is **idempotent** and extends the lease. Use this as a heartbeat.
- **Heartbeat cadence**: for any task expected to run longer than ~10 minutes, re-claim every 5–10 minutes. A simple loop: `while working; do epos claim "$id" -o "$AGENT_ID"; sleep 300; done` (in the background of the actual work).
- **`epos ready` filters out actively claimed tickets** (since v0.2.0). A ticket with a non-expired claim sidecar is hidden from the listing so concurrent agents do not race on it. A small window remains between listing and claiming, so still *attempt* `epos claim` and treat failure as "skip, try the next one." Pass `--include-claimed` to surface them anyway when debugging.
- **Eligibility**: only `open`, `pending`, or `repair_pending` tickets can be claimed. `closed`, `done`, `failed`, `claimed-by-other` will reject.
- **Release returns status to `pending`**: after `epos release`, the ticket reappears in `epos ready` for any agent (including the same one) to pick up. Add a `--note` first if explaining why.

### Exit codes

| Code | Meaning | Common cause |
|------|---------|--------------|
| 0 | success | — |
| 1 | generic error | already-claimed, lock contention, IO |
| 2 | validation error | malformed input, schema violation |
| 3 | not found | unknown ticket ID |
| 4 | ambiguous ID | short ID matches multiple tickets |

`AlreadyClaimedError` exits **1**, not a dedicated code — distinguish via stderr text or `--json` payload.

In `--json` mode, `epos lint` and `epos validate` always exit `0`. Inspect the payload, not `$?`.

## The `--json` Flag

Always prefer `--json` for machine parsing. Supported by all non-interactive CLI commands; not accepted by interactive commands such as `epos tui`.

```bash
epos show epo-d4f7 --json     # Structured output for scripts
epos ready --json             # Array of ready tickets
epos export --json            # Full JSON dump
epos new "title" ... --json   # Returns full ticket — extract id via | jq -r .id
```

Without `--json`, output is formatted for human readability. See **Exit codes** above for the JSON-mode validation gotcha.

## Session Start

Drop into an unfamiliar repo:

```bash
[ -d .tickets ] && epos ready --json | jq -r '.[] | "\(.id)\t\(.priority)\t\(.title)"'
```

If `.tickets/` exists, this is an epos repo — pick the highest-priority ready ticket and `epos claim` it. If empty, run `epos blocked --json` to see what's waiting on deps.

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
| `epos: command not found` | CLI not installed or not on `$PATH` | `go install ./cmd/epos` from the epos repo, then verify with `epos --version` |
| `no .tickets directory found` | Store not initialized | Run `epos new "Title"` to auto-create, or `mkdir .tickets` |
| `ambiguous partial ID` | Short ID matches multiple tickets | Provide more characters of the full ID |
| `validation failed` | Ticket violates schema rules | Run `epos validate <id>` for detailed errors; fix with `epos edit` |
| `claim failed: ticket already claimed` | Another agent holds the claim (exits 1) | Skip and pick another from `epos ready`; do not force-claim |
| Claim disappeared mid-work | 15-minute lease expired without re-claim | Re-run `epos claim <id> -o "$AGENT_ID"` every 5–10 min as heartbeat |
| Need to drop one dep | `--deps` replaces the full list | Read with `epos show <id> --json \| jq -r '.deps[]'`, reconstruct, re-set |
| Ticket reappears in `ready` after release | `epos release` resets status to `pending` | Expected behavior; add `--note` before release to explain |

## Top Anti-Patterns

The three that cause the most data corruption:

- **Direct file edits on `.tickets/`** — no `apply_patch`, `sed`, `cat >`, or shell redirects; use `epos edit` / `epos new`
- **Burying structured metadata in `--body` prose** — use `--deps`, `--priority`, `--tags`, `--parent` so query commands can see it
- **Mixing runtime state into frontmatter** — claims and leases live in `.tickets/.claims/` sidecars only; never inline

Full catalog (9 items, with rationale and worked examples) in [references/ANTI_PATTERNS.md](references/ANTI_PATTERNS.md). Read it before using `apply_patch`, `sed`, or any tool that writes inside `.tickets/`.
