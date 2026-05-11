# epos Agent Skill Contract

**Status:** spec
**Audience:** agent skill implementors (Wave 2 `mciz`), agent harness authors
**Source ADR:** `fabrikk-kb/decisions/0004-shared-ticket-system-architecture.md` ("Skill requirements" section)
**Source plan:** `IMPLEMENTATION_PLAN.md` (E7-T1)

---

## Purpose

This document is the **complete, non-negotiable contract** for the `epos` agent
skill. Every agent that creates, reads, or mutates epos tickets must obey this
contract. A conformant skill implementation must expose the commands, flags,
JSON shapes, and invariants defined here — no additions, no omissions.

---

## Core Invariant: No Freehand Ticket Editing

**Rule:** Agents MUST NOT write, patch, or edit any file under `.tickets/`
directly. This includes:

- No `apply_patch` on `.tickets/*.md` files
- No `sed`, `awk`, `cat >`, or heredoc writes into `.tickets/`
- No direct YAML frontmatter manipulation
- No direct markdown body edits

**Why it matters:** epos relies on two mechanisms that are **silently bypassed**
when agents edit files directly:

1. **Sidecar runtime state** (`.tickets/.claims/*.json`) — claim/lease operations
   write to sidecar files, not the canonical ticket. If an agent edits the
   ticket markdown directly, the sidecar is never consulted or updated.
2. **Atomic writes** (`ticket/store/atomic.go`) — the FileStore uses `atomicWrite`
   with file locking to prevent concurrent-write data loss. Direct file edits
   skip locking entirely.

The CLI is the **only** path that preserves consistency across canonical ticket
data, sidecar runtime state, and atomic write safety.

**Enforcement:** The skill must refuse any agent instruction that proposes
editing `.tickets/` files directly. The skill must redirect the agent to the
equivalent CLI command.

---

## Command Surface

The skill exposes exactly **12 commands**. Two epos CLI commands are
**deliberately excluded**:

- `tui` — interactive terminal UI, not appropriate for non-interactive agents
- `completion` — shell setup, one-time, not part of ticket workflows

### Command Reference

Every command supports two global flags:

| Flag | Description | Required? |
|------|-------------|-----------|
| `-d, --dir` | Directory containing the `.tickets/` folder | No (default: `.`) |
| `--json` | Output in JSON format | No (default: human-readable) |

The `--json` flag is **strongly preferred** for agent consumption. Human-readable
output is for terminal display only.

---

### 1. `epos new` — Create a ticket

```
epos new <title> [flags]
```

#### Flags

| Flag | Type | Repeatable | Description |
|------|------|------------|-------------|
| `-t, --type` | string | No | Ticket type: `epic`, `task`, `issue`, `feature`, `bug`, `chore`, `spike`, `doc` (default: `task`) |
| `-p, --priority` | int | No | Ticket priority (higher = more important) |
| `--parent` | string | No | Parent ticket ID |
| `--deps` | strings | No | Comma-separated dependency ticket IDs |
| `--tags` | strings | No | Comma-separated tags |
| `--assignee` | string | No | Assignee name or identifier |
| `--body` | string | No | Ticket description / narrative body |
| `--body-file` | string | No | Path to file whose content becomes the ticket body |
| `--ac` | stringArray | Yes | Acceptance criterion (repeatable) |
| `--intent` | string | No | High-level intent for the ticket |
| `--note` | stringArray | Yes | Initial note (repeatable) |
| `--stdin` | bool | No | Read ticket spec as JSON from stdin |

#### JSON output shape

```json
{
  "id": "epo-<slug>-<4-char-suffix>",
  "title": "<title>",
  "type": "<type>",
  "status": "open",
  "parent": "<parent-id>",
  "deps": ["<dep-id>", "..."],
  "priority": 0,
  "tags": ["<tag>", "..."],
  "description": "<body content>",
  "notes": ["<note>", "..."],
  "intent": "<intent>",
  "acceptance_criteria": ["<ac>", "..."],
  "scope": {},
  "implementation_detail": {},
  "created": "<ISO 8601 timestamp>",
  "updated_at": "<ISO 8601 timestamp>",
  "assignee": "<assignee>",
  "order": 0,
  "extended_status": "open"
}
```

**Exit code:** `0` on success, `1` on usage error.

---

### 2. `epos show` — Display a ticket

```
epos show <id> [flags]
```

#### Flags

No command-specific flags. Supports partial ID matching.

#### JSON output shape

```json
{
  "id": "<ticket-id>",
  "title": "<title>",
  "type": "<type>",
  "status": "<status>",
  "parent": "<parent-id>",
  "deps": ["<dep-id>", "..."],
  "priority": 0,
  "tags": ["<tag>", "..."],
  "description": "<body content>",
  "notes": ["<note>", "..."],
  "requirement_ids": ["<req-id>", "..."],
  "source_refs": ["<ref>", "..."],
  "lineage_id": "<lineage>",
  "risk_level": "<risk>",
  "intent": "<intent>",
  "constraints": ["<constraint>", "..."],
  "warnings": ["<warning>", "..."],
  "scope": {
    "owned_paths": ["<path>", "..."],
    "read_only_paths": ["<path>", "..."],
    "shared_paths": ["<path>", "..."],
    "isolation_mode": "<mode>"
  },
  "files_likely_touched": ["<path>", "..."],
  "implementation_detail": {
    "approach": "<approach>",
    "files": [
      {"path": "<path>", "change": "<change>", "reason": "<reason>"}
    ],
    "notes": "<notes>"
  },
  "learning_context": [
    {"id": "<id>", "type": "<type>", "title": "<title>"}
  ],
  "acceptance_criteria": ["<ac>", "..."],
  "test_cases": ["<tc>", "..."],
  "validation_commands": ["<cmd>", "..."],
  "validation_checks": [
    {"command": "<cmd>", "expected": "<expected>", "description": "<desc>"}
  ],
  "review_threshold": "<threshold>",
  "runtime": "<preference>",
  "required_evidence": ["<evidence>", "..."],
  "reviewer_guidance": "<guidance>",
  "created": "<ISO 8601>",
  "updated_at": "<ISO 8601>",
  "assignee": "<assignee>",
  "etag": "<etag>",
  "created_from": "<source>",
  "order": 0,
  "grouping_reason": "<reason>",
  "grouped_requirement_ids": ["<id>", "..."],
  "links": ["<link-id>", "..."],
  "extended_status": "<ext-status>",
  "status_reason": "<reason>"
}
```

All `omitempty` fields are absent from the JSON when empty/unset.

**Exit code:** `0` on success, `1` if ticket not found.

---

### 3. `epos edit` — Modify ticket fields

```
epos edit <id> [flags]
```

#### Flags

| Flag | Type | Repeatable | Description |
|------|------|------------|-------------|
| `-p, --priority` | int | No | New priority |
| `--parent` | string | No | New parent ticket ID |
| `--deps` | strings | No | Replacement dependency list (comma-separated) |
| `--tags` | strings | No | Replacement tags (comma-separated) |
| `--assignee` | string | No | New assignee |
| `--body` | string | No | Replacement body text |
| `--body-file` | string | No | Path to file with replacement body |
| `--ac` | stringArray | Yes | Acceptance criterion (repeatable) |
| `--intent` | string | No | New intent |
| `--note` | stringArray | Yes | Additional note (repeatable) |
| `--stdin` | bool | No | Read edit spec as JSON from stdin |

#### `--stdin` JSON input schema

When `--stdin` is used, the payload is a JSON object whose keys mirror the long flag names:

| Key | Type | Description |
|-----|------|-------------|
| `description` | string | Replacement body text (formerly `"body"` — renamed in v0.2.x) |
| `acceptance_criteria` | array of strings | Replacement acceptance-criteria list |
| `intent` | string | New intent |
| `priority` | int | New priority |
| `assignee` | string | New assignee |
| `tags` | array of strings | Replacement tags |
| `deps` | array of strings | Replacement dependency list |
| `parent` | string | New parent ticket ID |
| `notes` | array of strings | Additional notes |

All keys are optional; only supplied keys are mutated.

> **Migration note:** The description field key was renamed from `"body"` to `"description"` when `editTicketSpec` was promoted to the public `ticket.TicketPatch` library type. Update any scripts that pass `{"body": "..."}` to use `{"description": "..."}` instead.

> **Key divergence — `new` vs `edit`:** `epos new --stdin` still uses `"body"` for the description field (because `newTicketSpec` is a CLI-internal struct that was not part of this refactor), while `epos edit --stdin` uses `"description"` (the `ticket.TicketPatch` field name). This divergence is intentional: the `edit` command's schema was updated when `editTicketSpec` was promoted to the library `ticket.TicketPatch` type; the `new` command's internal spec was left unchanged.

#### JSON output shape

Same shape as `epos show --json`. Returns the full ticket after mutation.

**Exit code:** `0` on success, `1` on usage error or ticket not found.

---

### 4. `epos ready` — List tickets ready to work

```
epos ready [parent] [flags]
```

#### Behavior

- Without `parent`: lists all open tickets whose dependencies are resolved
- With `parent`: lists ready children of the given parent ticket
- A ticket is "ready" when all `deps` reference closed tickets

#### JSON output shape

```json
[
  {
    "id": "<ticket-id>",
    "title": "<title>",
    "type": "<type>",
    "status": "<status>",
    "parent": "<parent-id>",
    "deps": ["<dep-id>", "..."],
    "priority": 0,
    "tags": ["<tag>", "..."],
    "description": "<body>",
    "acceptance_criteria": ["<ac>", "..."],
    "created": "<ISO 8601>",
    "updated_at": "<ISO 8601>",
    "assignee": "<assignee>",
    "order": 0,
    "extended_status": "<ext-status>"
  }
]
```

If no tickets are ready, returns `null` (not `[]`).

**Exit code:** `0`.

---

### 5. `epos blocked` — List tickets blocked by dependencies

```
epos blocked [parent] [flags]
```

#### Behavior

- Without `parent`: lists all open tickets with unresolved dependencies
- With `parent`: lists blocked children of the given parent ticket
- A ticket is "blocked" when at least one `deps` item references an open ticket

#### JSON output shape

Same as `epos ready`.

**Exit code:** `0`.

---

### 6. `epos claim` — Claim a ticket for an agent

```
epos claim <id> [flags]
```

#### Flags

| Flag | Type | Description |
|------|------|-------------|
| `-o, --owner` | string | Agent ID claiming the ticket |

#### Behavior

- Writes claim state to `.tickets/.claims/<id>.json` (sidecar) — does NOT modify the canonical ticket file
- Rejects claims on tickets with ineligible statuses (not `open`/`pending`/`repair_pending`)
- Allows same-owner idempotent re-claim
- Sets `extended_status` to `claimed`

#### JSON output shape

Same as `epos show --json` (the ticket after claim state is applied).

**Exit code:** `0` on success, `3` on claim conflict.

---

### 7. `epos release` — Release a ticket claim

```
epos release <id> [flags]
```

#### Flags

| Flag | Type | Description |
|------|------|-------------|
| `-o, --owner` | string | Agent ID releasing the ticket |

#### Behavior

- Removes claim state from `.tickets/.claims/<id>.json`
- Does NOT modify the canonical ticket file
- Sets `extended_status` back to `open`

#### JSON output shape

Same as `epos show --json`.

**Exit code:** `0` on success, `3` if owner doesn't hold the claim.

---

### 8. `epos close` — Close a completed ticket

```
epos close <id> [flags]
```

#### Flags

| Flag | Type | Description |
|------|------|-------------|
| `-r, --reason` | string | Reason for the status change |

#### Behavior

- Sets `status` to `closed`
- Sets `extended_status` to `done`
- Writes reason to `status_reason` if provided

#### JSON output shape

Same as `epos show --json`.

**Exit code:** `0` on success, `1` if ticket not found.

---

### 9. `epos reopen` — Reopen a closed ticket

```
epos reopen <id> [flags]
```

#### Flags

| Flag | Type | Description |
|------|------|-------------|
| `-r, --reason` | string | Reason for the status change |

#### Behavior

- Sets `status` to `open`
- Sets `extended_status` to `open`
- Writes reason to `status_reason` if provided

#### JSON output shape

Same as `epos show --json`.

**Exit code:** `0` on success, `1` if ticket not found.

---

### 10. `epos export` — Export tickets as JSON

```
epos export [parent] [flags]
```

#### Behavior

- Without `parent`: exports all tickets in the store
- With `parent`: exports children of the given parent only

#### JSON output shape

Array of ticket objects (same shape as `epos show --json` per item).

```json
[
  { /* full ticket object */ },
  { /* full ticket object */ }
]
```

If no tickets, returns `null`.

**Exit code:** `0`.

---

### 11. `epos lint` — Validate all tickets in the store

```
epos lint [flags]
```

#### Behavior

- Validates every ticket in the store for schema compliance and dependency cycle detection
- In JSON mode: returns structured validation results with exit code `0`
- In human-readable mode: returns non-zero exit code if errors found

#### JSON output shape

```json
{
  "cycles": [["<id1>", "<id2>"], null],
  "tickets": [
    {
      "id": "<ticket-id>",
      "errors": ["<error message>", "..."]
    }
  ]
}
```

`cycles` is an array of dependency cycles, each cycle is an array of ticket IDs.
`tickets` contains only tickets with errors; valid tickets are omitted.

If no errors and no cycles:

```json
{
  "cycles": null,
  "tickets": null
}
```

**Exit code:** `0` in JSON mode. In human-readable mode: `0` if clean, `2` if
validation errors found.

---

### 12. `epos validate` — Validate a specific ticket

```
epos validate <id> [flags]
```

#### Behavior

- Validates a single ticket for schema compliance
- Supports partial ID matching
- In JSON mode: returns structured result with exit code `0`
- In human-readable mode: returns non-zero exit code if ticket is invalid

#### JSON output shape

Returns `null` if valid, or a ticket errors object if invalid:

```json
{
  "id": "<ticket-id>",
  "errors": ["<error message>", "..."]
}
```

**Exit code:** `0` in JSON mode. In human-readable mode: `0` if valid, `2` if
invalid.

---

## Machine-Readable Output

### JSON Convention

All commands support `--json`. When this flag is present:

- Output is valid JSON on stdout
- Exit code is `0` except for usage errors (missing args, unknown flags)
- Validation results are embedded in the JSON payload, not signaled via exit code
- `omitempty` struct tags suppress absent fields — agents must handle missing keys gracefully

### Parsing Rules for Agents

1. **Always use `--json`** for programmatic consumption. Never parse human-readable output.
2. **Check exit code first.** If non-zero, the command failed and stdout may not be valid JSON.
3. **Handle missing keys.** Fields tagged `omitempty` are absent when zero-valued.
4. **IDs are strings.** Ticket IDs contain hyphens and alphanumeric characters.
5. **Timestamps use ISO 8601.** Parse with a standard ISO 8601 parser.

---

## Multi-Repo Discovery

### `--dir` Flag

The `-d, --dir` flag tells epos where to find the `.tickets/` directory.

```bash
epos <command> -d /path/to/project-root [args...]
```

#### Default behavior

- When `--dir` is omitted, epos uses the current working directory
- The CLI looks for `<dir>/.tickets/` — it does not search parent directories

#### Agent usage pattern

Agents working across repos should:

1. Identify the target repo root (from harness context, plan, or user instruction)
2. Pass the repo root as `-d <repo-root>`
3. If no `.tickets/` directory exists at that path, skip ticket enrichment
   silently — do not fail the workflow

```bash
# Check if a repo has tickets
test -d /path/to/repo/.tickets && echo "found" || echo "no tickets"

# Operate on that repo's tickets
epos ready -d /path/to/repo --json
epos show epo-54xg -d /path/to/repo --json
```

#### vakt-specific discovery

vakt's review pipeline already knows the target repo root. vakt passes that
path directly:

```bash
epos export -d <vakt-review-target> --json
```

If `epos` is not found in PATH, vakt skips ticket enrichment silently.

---

## Agent Workflow Rules (ADR 0004)

These five rules from ADR 0004 are binding on every agent that touches tickets.

### Rule 1 — Never write ticket files directly

**What it means:** No `apply_patch`, no file writes, no markdown manipulation
on anything under `.tickets/`.

**Enforcement:**
- The skill must refuse direct-file-edit instructions
- The skill must redirect to the correct CLI command
- If an agent proposes a workflow that includes `apply_patch` on a `.tickets/`
  path, the skill must block that step

### Rule 2 — Always use the CLI to create or mutate tickets

**What it means:** Ticket creation, field updates, status transitions, claim
operations — everything goes through `epos` CLI commands.

**Enforcement:**
- The skill wraps every mutation as a CLI invocation
- The skill never bypasses the CLI with direct store access or file I/O
- The skill never calls Go library functions from the `epos/ticket` package
  directly — the CLI is the interface

### Rule 3 — Validate before handing tickets to execution

**What it means:** After creating or editing a ticket, run validation before
declaring it ready for an executor (verk, human, or another agent).

**Required sequence:**
```bash
epos new "Implement feature X" --body "..." --ac "..." --json
# Check exit code, extract ID
epos validate <id> --json
# If errors returned, fix and re-validate
# Only proceed to claim/execute when validate returns null
epos lint --json
# Ensure no cycles were introduced
```

**Trigger points:**
- After `epos new` — validate the new ticket
- After `epos edit` — validate the modified ticket
- After `epos close` on a dependency — run `epos lint` to catch introduced cycles
- Before `epos claim` — the ticket must pass validation

### Rule 4 — Prefer structured fields over prose

**What it means:** When the CLI provides a dedicated flag for a field, use
it instead of embedding the information in `--body` text.

**Concrete mapping:**

| Information to convey | Use this flag | Don't use |
|-----------------------|---------------|-----------|
| This ticket depends on X | `--deps X` | `--body "Depends on X"` |
| This is a child of Y | `--parent Y` | `--body "Part of epic Y"` |
| Priority | `-p 3` | `--body "High priority"` |
| Tags / labels | `--tags backend,api` | `--body "Tags: backend, api"` |
| Who should work on it | `--assignee alice` | `--body "Alice owns this"` |
| Acceptance criteria | `--ac "..."` (repeatable) | `--body "AC: ..."` |
| What problem it solves | `--intent "..."` | `--body` only if narrative needed |
| Narrative / context | `--body "..."` | — (this is the right place) |

**Rationale:** Structured fields are queryable (`epos ready`, `epos blocked`),
validatable, and machine-readable. Prose is not.

### Rule 5 — Only enter freeform prose in designated body sections

**What it means:** Freeform text belongs in these fields only:

- `--body` / `--body-file` — narrative description of the ticket
- `--note` — timestamped commentary (repeatable)
- `--ac` — individual acceptance criteria (repeatable)
- `--intent` — high-level goal statement

**Do NOT:**
- Embed structured data as prose in `--body`
- Use note fields for structured metadata
- Put YAML-like key-value pairs in body text expecting them to be parsed

---

## Error Handling

### Exit Code Contract

| Exit code | Meaning | Agent action |
|-----------|---------|-------------|
| `0` | Success (or validation result in JSON) | Continue |
| `1` | Usage error (bad args, missing ticket, unknown flag, I/O error) | Stop. Report error. Do not retry with same args. |
| `2` | Validation failure (in human-readable mode only) | Fix reported errors, re-validate. In JSON mode, exit code is `0` — check JSON payload instead. |
| `3` | Claim conflict (already claimed by another agent) | Stop. Do not proceed with ticket execution. Report conflict. |

### Agent Handling Rules

1. **Never proceed after non-zero exit.** If `epos` exits `1`, `2`, or `3`, the
   agent must stop the current workflow and report the failure.
2. **In JSON mode, don't rely on exit code for validation.** `epos lint --json`
   and `epos validate --json` exit `0` even when errors exist — inspect the
   JSON payload.
3. **Claim conflicts are hard stops.** Exit code `3` means another agent holds
   the ticket. Do not attempt to force-claim or edit the claim sidecar.
4. **Usage errors are not retryable.** Exit code `1` means the command
   arguments are wrong. Fix the command, don't loop.

---

## Commands Deliberately Excluded

| Command | Reason for exclusion |
|---------|---------------------|
| `epos tui` | Interactive terminal UI. Non-interactive agents cannot operate a TUI. |
| `epos completion` | Shell autocompletion setup. One-time human operation, not part of ticket workflows. |

---

## Implementation Checklist for the Skill

A conformant skill implementation must:

- [ ] Wrap all 12 commands documented above
- [ ] Never call Go libraries from `github.com/php-workx/epos/ticket` directly — CLI only
- [ ] Always use `--json` for programmatic output
- [ ] Handle exit codes per the contract
- [ ] Enforce the "no freehand editing" invariant by refusing direct `.tickets/` writes
- [ ] Support `--dir` for multi-repo operation
- [ ] Run `epos validate` after every `epos new` or `epos edit`
- [ ] Run `epos lint` after closing tickets that have dependents
- [ ] Map all structured metadata to dedicated flags (Rule 4)
- [ ] Restrict freeform prose to designated fields (Rule 5)
- [ ] Work with partial ID matching on all ID-accepting commands
- [ ] Never expose `tui` or `completion` to agents

---

## References

- **ADR 0004** — `fabrikk-kb/decisions/0004-shared-ticket-system-architecture.md` (Skill requirements, Phase 5)
- **Implementation Plan** — `IMPLEMENTATION_PLAN.md` (Epic E7, D4-D8)
- **Canonical types** — `ticket/tickets.go` (Ticket struct, Status constants, field definitions)
- **Existing reference skill** — `~/.codex/skills/tk/SKILL.md` (structural pattern for CLI-wrapper skills)
