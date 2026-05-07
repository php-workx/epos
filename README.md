# epos

A task and issue tracker for AI agents and their Humans.

epos provides a canonical ticket model, a file-based store with YAML frontmatter Markdown storage, and a CLI for creating, querying, and managing tickets. It is designed as the single source of truth for task tracking with claim/release functionality.

Provided as a CLI and Golang package.

## Architecture

```text
┌──────────────────────────────────────────────────────────────┐
│                        CLI (cmd/epos)                        │
│  new · edit · show · claim · release · close · reopen ·     │
│  ready · blocked · validate · lint · export · tui            │
└─────────────────────┬────────────────────────────────────────┘
                      │
          ┌───────────┴───────────┐
          │       ticket/          │
          │  Ticket · Status       │
          │  Validate · Errors     │
          │  RuntimeState          │
          └───┬───────┬───────┬───┘
              │       │       │
   ┌──────────┐ ┌─────┴────┐ ┌────────────┐
   │ store/   │ │ runtime/  │ │  graph/    │
   │FileStore │ │Claim·Lease│ │Ready·Blocked│
   │CRUD·ID   │ │Sidecars   │ │Cycles·Children│
   └────┬─────┘ └──────────┘ └────────────┘
        │
   ┌────┴─────┐
   │ markdown/│
   │YAML FM   │
   │Marshal   │
   │Unmarshal │
   └──────────┘
```

**Storage layout:**

```text
<repo-root>/
└── .tickets/
    ├── <ticket-id>.md          # Ticket with YAML frontmatter + body
    └── .claims/
        └── <ticket-id>.json    # Sidecar runtime state (claim, lease, heartbeat)
```

**Key design decisions:**

- **D1:** Tickets are stored as Markdown files with YAML frontmatter, compatible with the `tk` CLI format.
- **D6:** Claims and leases live in sidecar `.claims/<id>.json` files, not in the ticket frontmatter.
- **D7:** Dependencies use `deps` (directional) and `parent` (hierarchy). `links` (symmetric) are preserved but inert in MVP.

## Packages

| Package | Purpose |
|---------|---------|
| `ticket` | Core types: `Ticket`, `Status`, errors, validation |
| `ticket/markdown` | YAML frontmatter marshal/unmarshal, Markdown body handling |
| `ticket/store` | `FileStore`: CRUD, atomic writes, ID generation, partial ID resolution |
| `ticket/runtime` | Claim/lease sidecar read/write with file locking |
| `ticket/graph` | Pure dependency-graph functions (no I/O) |
| `ticket/testutil` | Shared test helpers (`NewTestStore`, `NewTestTicket`, `MustCreateTicket`) |
| `cmd/epos` | CLI binary |

## Installation

```bash
go install ./cmd/epos
```

## Usage

### Create a ticket

```bash
epos new "Implement auth module" --type feature --priority 3
# Output: epo-implement-auth-module-a1b2
```

Create a ticket with rich content:

```bash
epos new "Atomic writes" \
  --type task --priority 3 \
  --body "Implement fsync+rename in ticket/store/atomic.go" \
  --ac "grep -r 'os.WriteFile' ticket/store/store.go returns no matches" \
  --ac "go test ./ticket/store/... exits 0" \
  --note "gofrs/flock is already in go.mod" \
  --assignee agent-1 \
  --tags "store,reliability" \
  --intent "prevent partial writes on crash"
```

Or via JSON stdin (useful for programmatic ticket creation):

```bash
echo '{
  "title": "JSON-created ticket",
  "type": "task",
  "body": "Created from stdin",
  "acceptance_criteria": ["criterion one", "criterion two"],
  "assignee": "agent-1",
  "tags": ["backend", "api"]
}' | epos new --stdin
```

**`epos new` flags:**

| Flag | Description |
|------|-------------|
| `--type` | Ticket type: `epic`, `task`, `issue`, `feature`, `bug`, `chore`, `spike`, `doc` (default: `task`) |
| `--priority` | Priority integer, higher = more important (default: 0) |
| `--parent` | Parent ticket ID |
| `--deps` | Comma-separated dependency ticket IDs |
| `--body` | Narrative description |
| `--body-file` | Path to file whose content becomes the body |
| `--ac` | Acceptance criterion (repeatable; one value per flag) |
| `--note` | Initial note (repeatable) |
| `--assignee` | Assignee name or identifier |
| `--tags` | Comma-separated tags |
| `--intent` | High-level intent for the ticket |
| `--stdin` | Read ticket spec as JSON from stdin (flags override stdin values) |

### Edit a ticket

Update fields on an existing ticket without touching its body content:

```bash
epos edit epo-auth --priority 5 --assignee agent-2
epos edit epo-auth --ac "new acceptance criterion" --note "follow-up note"
```

Or via JSON stdin:

```bash
echo '{"body": "updated narrative", "acceptance_criteria": ["new criterion"]}' \
  | epos edit epo-auth --stdin
```

**`epos edit` flags:** same as `epos new` except `--type` (type cannot change after creation).

### Show a ticket

```bash
epos show epo-implement-auth-module-a1b2
# Or use a partial ID:
epos show epo-implement
```

### Query tickets

```bash
# List unclaimed tickets ready to work (no open dependencies)
epos ready

# Include tickets with active claim sidecars
epos ready --include-claimed

# List tickets blocked by open dependencies
epos blocked

# Export all tickets as JSON
epos export --json

# Export children of a specific parent
epos export epo-parent-id --json
```

### Interactive TUI

```bash
# Open the grouped ticket operator UI
epos tui

# Limit the view to children of a parent ticket
epos tui epo-parent-id

# Set the owner used by claim/release actions
epos tui --owner agent-1

# Change polling refresh interval; 0 disables polling
epos tui --refresh 10s
```

The TUI shows grouped ticket lists (`ready`, `blocked`, `claimed`, `open`, `closed`, `all`) with a detail pane. Use `j`/`k` or arrows to move, `tab`/`shift+tab` to switch groups, `/` to search, `r` to refresh, `enter` for detail, `n` to add a note, `c`/`u` to claim/release, `x`/`o` to close/reopen, and `q` to quit.

For claim/release actions, `--owner` wins over `EPOS_OWNER`, then `USER`, then `USERNAME`. `epos tui` is interactive and rejects the global `--json` flag.

### Claim and release

```bash
# Claim a ticket for an agent
epos claim epo-auth --owner agent-1

# Release a ticket
epos release epo-auth --owner agent-1
```

### Transition status

```bash
# Close a ticket
epos close epo-auth

# Reopen a closed ticket
epos reopen epo-auth --reason "regression found"
```

### Validate

```bash
# Validate a single ticket
epos validate epo-auth

# Lint all tickets for errors and cycles
epos lint
```

### Global flags

```bash
--dir string   Directory containing the .tickets folder (default ".")
--json         Output in JSON format for supported commands; rejected by epos tui
```

### Exit codes

| Code | Error type |
|------|-----------|
| 0 | Success |
| 1 | Generic error |
| 2 | `ValidationError` |
| 3 | `TicketNotFoundError` |
| 4 | `AmbiguousIDError` |

## Requirements

- Go 1.26.2+
- `gofumpt` for formatting (`go install mvdan.cc/gofumpt@latest`)

## Module

```text
github.com/php-workx/epos
```
