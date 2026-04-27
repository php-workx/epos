# epos

A task and issue tracker for AI agents and their Humans.

epos provides a canonical ticket model, a file-based store with YAML frontmatter Markdown storage, and a CLI for creating, querying, and managing tickets. It is designed as the single source of truth for task tracking with claim/release functionality.

Provided as a CLI and Golang package.

## Architecture

```text
┌──────────────────────────────────────────────────────────────┐
│                        CLI (cmd/epos)                        │
│  new · show · claim · release · close · reopen · ready ·    │
│  blocked · validate · lint · export · status                 │
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
├── .tickets/
│   ├── <ticket-id>.md          # Ticket with YAML frontmatter + body
│   └── .claims/
│       └── <ticket-id>.json    # Sidecar runtime state (claim, lease, heartbeat)
└── cmd/epos/                   # CLI binary
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

### Show a ticket

```bash
epos show epo-implement-auth-module-a1b2
# Or use a partial ID:
epos show epo-implement
```

### Query tickets

```bash
# List tickets ready to work (no open dependencies)
epos ready

# List tickets blocked by open dependencies
epos blocked

# Export all tickets as JSON
epos export --json

# Export children of a specific parent
epos export epo-parent-id --json
```

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
--json         Output in JSON format
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
