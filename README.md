# epos

Shared ticket system for the php-workx toolchain.

> Full documentation coming in a later issue. See `IMPLEMENTATION_PLAN.md` for the current architecture reference.

## Module

```text
github.com/php-workx/epos
```

## Packages

| Package | Purpose |
|---------|---------|
| `ticket` | Core types: `Ticket`, `Status`, errors, validation |
| `ticket/markdown` | YAML frontmatter marshal/unmarshal, Markdown body handling |
| `ticket/store` | `FileStore`: CRUD, atomic writes, file locking |
| `ticket/runtime` | Claim/lease sidecar read/write |
| `ticket/graph` | Pure dependency-graph functions (no I/O) |
| `ticket/testutil` | Shared test helpers |
| `cmd/epos` | CLI binary |

## Requirements

- Go 1.26.2+
- `gofumpt` for formatting (`go install mvdan.cc/gofumpt@latest`)
