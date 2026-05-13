# epos — Domain Glossary

## Core Concepts

**Ticket**
The unit of work. Has an ID (store-assigned, lowercase-alphanumeric-hyphen), a type, a title, optional deps/links/tags, and a Present map tracking which fields are explicitly set.

**Store**
The read/write interface for a ticket repository. Two implementations: FileStore (filesystem, YAML frontmatter) and MemStore (in-memory, for tests). The Store boundary is pure data access — business logic (availability, validation) lives above it.

**Claim / Lease**
A time-bounded lock on a ticket held by an agent. Stored as a sidecar file alongside the ticket. A claim is active if it exists and has not expired. Expiry is checked at read time; no background cleanup runs.

**Availability**
The computed state of a ticket combining two orthogonal concepts: *readiness* (no open blocking dependencies) and *unclaimedness* (no active lease). A ticket is available when it is both ready and unclaimed. `ReadyTickets` and `ReadyChildren` are the canonical functions for this concept.

**Ready**
A ticket with no open blocking dependencies, as determined by the dependency graph. Readiness is a pure graph property — it does not consider claims.

**Present map**
The set of fields explicitly set on a Ticket or TicketPatch. Used by Apply to distinguish "field was set to zero value" from "field was not touched."

**TicketPatch**
A partial-update descriptor. Only fields explicitly set via setters (or decoded from a JSON object) are applied when Apply is called; absent fields leave the target Ticket unchanged.

## Validation

**ValidateNew**
Validates all user-supplied fields on a not-yet-stored Ticket. Does not check ID — IDs are store-assigned and do not exist at validation time. Returns the first validation error encountered (fail-fast). Returns an internal error (exit code 1) if called with a nil pointer.

**IsValidType**
Exported function that reports whether a string is a known ticket type (epic, task, issue, feature, bug, chore, spike, doc). Single source of truth; replaces the previously exported `ValidTypes` map to prevent external mutation.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Generic / internal error |
| 2 | Validation error |
| 3 | Ticket not found |
| 4 | Ambiguous ID |
| 5 | Dependency cycle detected |
| 6 | Claim conflict (already claimed / not claimed / not claim owner) |

**ExitCoder**
Interface `{ ExitCode() int }` implemented by domain error types that carry a deterministic CLI exit code. `main.go` checks this interface; errors without it fall through to exit code 1.
