# Implementation Plan: Shared Ticket System (`epos`)

**Status:** Draft
**Date:** 2026-04-23
**Source ADR:** `fabrikk-kb/decisions/0004-shared-ticket-system-architecture.md`
**Source extraction spec:** `fabrikk/docs/specs/public-task-package.md`

---

## 0. Decisions Made

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Go directly to `epos` shared repo** — skip the `fabrikk/ticket` intermediate stage | The `epos` repo already exists and is empty. Two migration hops (internal → fabrikk/ticket → epos) create unnecessary churn. The fabrikk extraction spec is used as a *reference* for which types move and how, but the target package is `epos` from day one. |
| D2 | **CLI name: `epos`** | Short, unique, no conflict with `tk`. The ADR placeholder `cmd/<ticket-cli>` resolves to `cmd/epos`. |
| D3 | **Go module path: `github.com/php-workx/epos`** | Matches the repo. Packages live under this module. |
| D4 | **tk-style markdown compatibility is a hard requirement for Phase 1, migration bridge only after** | Both `fabrikk` and `verk` already produce/consume tk-compatible markdown. Breaking that in Phase 1 would block adoption. After both repos are integrated, the CLI can evolve the format with versioned schemas. |
| D5 | **`vakt` is read-only in Phase 1** | The ADR says vakt is read-mostly. No write integration until there's a proven use case. |
| D6 | **Claims/leases live in sidecar files, not inline in the ticket frontmatter** | This is a core ADR decision. `fabrikk` currently puts claim fields (`claimed_by`, `claim_backend`, `claim_expires`, `claim_heartbeat`) inline; `verk` already uses sidecar JSON files (`.claims/<ticket-id>.json`). The shared package standardizes on sidecars. |
| D7 | **Dependency semantics for MVP: `deps` (directional) + `parent` (hierarchy)** | Both repos already use these. `links` (symmetric) exists in fabrikk's format but is stubbed. No new dependency types in MVP. |
| D8 | **Flat layout without `pkg/` prefix, singular `ticket` package name** | `ticket/`, `ticket/markdown/`, `ticket/runtime/`, `ticket/graph/`, `ticket/store/`, `cmd/epos`. No `pkg/` nesting — Go convention for library-oriented repos is flat packages under the module root. Singular `ticket` (not `tickets`) matches the primary type name. |

---

## 1. Target Architecture

```text
github.com/php-workx/epos/
├── cmd/
│   └── epos/                    # CLI binary
│       └── main.go
├── internal/
│   └── tui/                     # Bubble Tea ticket operator UI
├── ticket/
│   ├── tickets.go               # Core types: Ticket, Status, NewTicket, TicketOption
│   ├── runtime.go               # Runtime types: Claim, Lease, Heartbeat, RuntimeState
│   ├── validation.go            # Schema validation rules
│   ├── errors.go                # Custom error types (not sentinels)
│   ├── testutil/
│   │   └── helpers.go           # NewTestStore, NewTestTicket helpers
│   ├── markdown/
│   │   ├── frontmatter.go       # YAML frontmatter marshal/unmarshal (custom MarshalYAML)
│   │   ├── body.go              # Markdown body handling, section parsing
│   │   ├── compat.go            # tk compatibility: status mapping, unknown field pass-through
│   │   └── compat_test.go
│   ├── runtime/
│   │   ├── claim.go             # Claim/lease sidecar read/write
│   │   └── claim_test.go
│   ├── graph/
│   │   ├── deps.go              # Pure functions: ReadyFilter, BlockedFilter, DetectCycles, FilterChildren, FilterReadyChildren
│   │   └── deps_test.go
│   └── store/
│       ├── store.go             # FileStore: CRUD + I/O-bound child discovery (ListAllChildren, ListReadyChildren, HasChildren)
│       ├── id.go                # ID generation, partial matching
│       ├── atomic.go             # Atomic writes, file locking
│       └── store_test.go
├── skill/                       # Agent skill wrapping the CLI (future)
├── go.mod
└── go.sum
```

### 1.1 Canonical Ticket Model

The canonical `Ticket` struct merges the best of both repos into one schema with four facets:

```go
// Core graph — shared by all consumers
type Ticket struct {
    ID       string   `yaml:"id"       json:"id"`
    Title    string   `yaml:"title"    json:"title"`
    Type     string   `yaml:"type"     json:"type"`
    Status   Status   `yaml:"status"   json:"status"`
    Parent   string   `yaml:"parent"   json:"parent,omitempty"`
    Deps     []string `yaml:"deps"     json:"deps,omitempty"`
    Priority int      `yaml:"priority" json:"priority"`
    Tags     []string `yaml:"tags"     json:"tags,omitempty"`

    // Planning facet (authored by fabrikk)
    RequirementIDs       []string             `yaml:"requirement_ids"        json:"requirement_ids,omitempty"`
    SourceRefs            []string             `yaml:"source_refs"            json:"source_refs,omitempty"`
    LineageID             string               `yaml:"lineage_id"             json:"lineage_id,omitempty"`
    RiskLevel             string               `yaml:"risk_level"             json:"risk_level,omitempty"`
    Intent                string               `yaml:"intent"                 json:"intent,omitempty"`
    Constraints           []string             `yaml:"constraints"             json:"constraints,omitempty"`
    Warnings              []string             `yaml:"warnings"                json:"warnings,omitempty"`
    Scope                 TaskScope            `yaml:",inline"                json:"scope,omitempty"`
    FilesLikelyTouched    []string             `yaml:"files_likely_touched"   json:"files_likely_touched,omitempty"`
    ImplementationDetail  ImplementationDetail `yaml:"implementation_detail"  json:"implementation_detail,omitempty"`
    LearningContext       []LearningRef        `yaml:"learning_context"       json:"learning_context,omitempty"`

    // Static execution facet (authored by fabrikk, consumed by verk)
    AcceptanceCriteria    []string             `yaml:"acceptance_criteria"    json:"acceptance_criteria,omitempty"`
    TestCases             []string             `yaml:"test_cases"             json:"test_cases,omitempty"`
    ValidationCommands    []string             `yaml:"validation_commands"   json:"validation_commands,omitempty"`
    ValidationChecks      []ValidationCheck    `yaml:"validation_checks"     json:"validation_checks,omitempty"`
    ReviewThreshold       string               `yaml:"review_threshold"       json:"review_threshold,omitempty"`
    RuntimePreference     string               `yaml:"runtime"                json:"runtime,omitempty"`
    RequiredEvidence      []string             `yaml:"required_evidence"      json:"required_evidence,omitempty"`
    ReviewerGuidance      string               `yaml:"reviewer_guidance"      json:"reviewer_guidance,omitempty"`

    // Metadata
    Created    string            `yaml:"created"    json:"created"`
    UpdatedAt  string            `yaml:"updated_at" json:"updated_at,omitempty"`
    Assignee   string            `yaml:"assignee"   json:"assignee,omitempty"`
    ETag       string            `yaml:"etag"       json:"etag,omitempty"`
    CreatedFrom string           `yaml:"created_from" json:"created_from,omitempty"`
    Order      int               `yaml:"order"      json:"order"`

    // Grouping (fabrikk-specific for now, may generalize)
    GroupingReason         string   `yaml:"grouping_reason"         json:"grouping_reason,omitempty"`
    GroupedRequirementIDs []string `yaml:"grouped_requirement_ids" json:"grouped_requirement_ids,omitempty"`

    // Symmetric links (fabrikk has these; verk does not)
    Links []string `yaml:"links,omitempty" json:"links,omitempty"`

    // Extended status (maps to tk 3-status model)
    ExtendedStatus string `yaml:"extended_status" json:"extended_status,omitempty"`
    StatusReason   string `yaml:"status_reason"   json:"status_reason,omitempty"`

    // Forward compatibility
    Extra map[string]any `yaml:",inline" json:"-"`

    // Round-trip tracking (exported for programmatic ticket creation)
    // Present tracks which fields were explicitly set in frontmatter.
    // TitleDerived tracks whether title was extracted from body heading.
    // Both are critical for round-trip idempotency with tk.
    Present       map[string]bool `yaml:"-" json:"-"`
    TitleDerived  bool            `yaml:"-" json:"-"`
}

// NewTicket creates a ticket with sensible defaults and correctly
// populated Present map. Use TicketOption functions to set fields.
func NewTicket(opts ...TicketOption) *Ticket { ... }

type TicketOption func(*Ticket)
func WithTitle(title string) TicketOption { ... }
func WithType(typ string) TicketOption    { ... }
func WithStatus(s Status) TicketOption    { ... }
func WithParent(parent string) TicketOption { ... }
func WithDeps(deps ...string) TicketOption { ... }

type TaskScope struct {
    OwnedPaths    []string `yaml:"owned_paths"     json:"owned_paths,omitempty"`
    ReadOnlyPaths []string `yaml:"read_only_paths" json:"read_only_paths,omitempty"`
    SharedPaths   []string `yaml:"shared_paths"    json:"shared_paths,omitempty"`
    IsolationMode string   `yaml:"isolation_mode"  json:"isolation_mode,omitempty"`
}
```

### 1.2 Runtime Sidecar Model

```go
// Runtime state lives in .tickets/.claims/<ticket-id>.json
// NOT in the ticket frontmatter
type RuntimeState struct {
    TicketID   string     `json:"ticket_id"`
    Claim      *Claim     `json:"claim,omitempty"`
    Lease      *Lease     `json:"lease,omitempty"`
    Heartbeat  *Heartbeat `json:"heartbeat,omitempty"`
    Phase      string     `json:"phase,omitempty"`      // "implementing", "verifying", "reviewing", "repairing"
    Attempt    int        `json:"attempt,omitempty"`
    Artifacts  []string   `json:"artifacts,omitempty"`  // refs to review/repair/closeout artifacts
}

type Claim struct {
    ClaimedBy    string    `json:"claimed_by"`
    ClaimBackend string    `json:"claim_backend"`
    ClaimedAt    time.Time `json:"claimed_at"`
}

type Lease struct {
    LeaseID    string    `json:"lease_id"`
    ExpiresAt   time.Time `json:"expires_at"`
}

type Heartbeat struct {
    LastBeat time.Time `json:"last_beat"`
}
```

### 1.3 Status Model

```go
type Status string

const (
    StatusOpen       Status = "open"        // tk-compatible
    StatusReady      Status = "ready"        // tk-compatible (synonym for open)
    StatusInProgress Status = "in_progress"  // tk-compatible
    StatusBlocked    Status = "blocked"       // tk-compatible (open but blocked by deps)
    StatusClosed     Status = "closed"       // tk-compatible
)

// Extended status maps to the fabrikk model.
// Note: StatusBlocked ("blocked") appears only in the tk-native set above.
// For operator-initiated blocking, use StatusHeld ("held").
const (
    StatusPending       Status = "pending"        // maps to tk: open
    StatusClaimed       Status = "claimed"        // maps to tk: in_progress
    StatusImplementing  Status = "implementing"    // maps to tk: in_progress
    StatusVerifying     Status = "verifying"       // maps to tk: in_progress
    StatusUnderReview  Status = "under_review"    // maps to tk: in_progress
    StatusRepairPending Status = "repair_pending" // maps to tk: open
    StatusHeld          Status = "held"           // maps to tk: blocked (operator-initiated blocking)
    StatusDone         Status = "done"            // maps to tk: closed
    StatusFailed        Status = "failed"         // maps to tk: closed
)
```

> **Design note on StatusBlocked collision:** The old extended set had `StatusBlocked = "blocked"` which collided with the tk-native `StatusBlocked`. These had different semantics: tk's "blocked" means "open but blocked by dependencies", while the extended "blocked" meant "explicitly blocked by operator decision." The collision is resolved by removing `StatusBlocked` from the extended set and adding `StatusHeld = "held"` for operator-initiated blocking. This eliminates the string collision while preserving both semantics.

---

## 2. What Code Lifts From Where

| Capability | Source | What to Lift | What to Change |
|---|---|---|---|
| Markdown frontmatter parse/encode | `fabrikk/internal/ticket/format.go` | `splitFrontmatterBody`, `MarshalTicket`, `UnmarshalTicket`, `UpdateFrontmatter`, `renderFrontmatterBody`, `TaskToFrontmatter`, `FrontmatterToTask` | Use `ticket.Ticket` instead of `state.Task`; implement custom `MarshalYAML()` returning `*yaml.Node` for deterministic field ordering and present-map-aware emission; remove inline claim preservation logic (D6); handle `TitleDerived` to avoid duplicating `# Title` heading |
| YAML value parsing | `verk/tkmd/store.go` (lines 391–795) | `decodeFrontMatter`, `assignField`, `parseScalar`, `parseInlineList`, `formatString`, `formatYAMLValue`, `splitKeyValue`, `isPlainString` | Both repos have hand-rolled YAML parsers. Consolidate into one using `gopkg.in/yaml.v3` (fabrikk's approach) since it's more robust. |
| Status mapping | `fabrikk/internal/ticket/format.go` | `StatusToTicket`, `StatusFromTicket`, `extendedToTicketStatus`, `validExtendedStatuses` | Extend with verk's 5-status model; keep dual mapping (extended ↔ tk-native). Replace `StatusBlocked` collision with `StatusHeld = "held"` |
| Store CRUD | `fabrikk/internal/ticket/store.go` | `Store`, `CreateRun`, `ReadTasks`, `WriteTasks`, `ReadTask`, `WriteTask`, `UpdateStatus`, `AddNote`, `AddDep`, `RemoveDep`, `Link`, `Unlink` | Adapt to use `ticket.Ticket`; remove `state.Task` dependency; add `PartialReadError` for partial reads; add `validateTicketWritable` from verk |
| ID generation & resolution | `fabrikk/internal/ticket/` (id.go) | `GenerateID`, `ResolveID`, `ValidateID` | Move as-is; `GenerateID` retries up to 3 times on collision |
| Atomic writes & file locking | `fabrikk/internal/ticket/store.go` | `atomicWrite`, `withLock`, `withLocks` | Move as-is; use `flock` |
| Dependency graph | Both repos have `deps.go` | fabrikk: `ReadyFilter`, `BlockedFilter`, `DetectCycles`; verk: `ListReadyChildren`, `depsClosed`, `DetectEpicCycle` | Pure functions only in `ticket/graph/`: `ReadyFilter` (sorts by priority then ID), `BlockedFilter` (dependency- and parent-aware blocking), `DetectCycles`, `FilterChildren`, `FilterReadyChildren` (parent-scoped readiness). I/O-bound `ListAllChildren`, `ListReadyChildren`, `HasChildren`, `DetectEpicCycle` move to `ticket/store/` |
| Claim sidecar | `verk/tkmd/store.go` + `fabrikk/internal/ticket/claim.go` | `claimRecord`, `claimAllowsReady`, `validateClaimIdentifier`, `ClaimTask`, `ReleaseClaim`, `RenewClaim`, `ReclaimExpired`, `ReadClaimsForRun`, `DefaultLeaseDuration` | Lift claim logic from verk's JSON sidecar approach + fabrikk's claim lifecycle; add `validateClaimEligibility` (only pending/repair_pending); `Release` takes `newStatus` and `reason`; same-owner re-claim is idempotent |
| Owned path validation | `verk/tkmd/store.go` | `validateOwnedPath` | Move as-is |
| Title extraction from body | `verk/tkmd/store.go` | `extractHeadingTitle` | Move as-is |

### 2.1 Fields New to epos (Not in Either Source)

The following fields on the canonical `Ticket` struct do not exist in either fabrikk's `Frontmatter`/`state.Task` or verk's `Ticket`. They are intentional additions for epos:

| Field | Rationale |
|-------|-----------|
| `SourceRefs []string` | References to external sources (GitHub issues, design docs). Not in fabrikk or verk. |
| `ReviewerGuidance string` | Instructions for human reviewers. Not in fabrikk or verk. |
| `Links []string` | **Exists in fabrikk Frontmatter** (format.go line 24) but was missing from the canonical Ticket struct. Now added. Operated on by `Link`/`Unlink` Store methods. D7 excludes `links` from MVP dependency semantics, but the field is preserved for data round-trip. |

---

## 3. Migration Path

### Phase 0: Bootstrap `epos` repo

Set up the Go module, directory structure, CI, and initial scaffold. No code from other repos yet — just the skeleton.

### Phase 1: Canonical schema + markdown package

Define the `Ticket`, `RuntimeState`, `Status` types. Implement markdown frontmatter marshal/unmarshal with tk compatibility. Implement validation.

### Phase 2: Store + graph + runtime

Implement `FileStore` (CRUD, atomic writes, file locking). Implement dependency graph (ready/blocked/cycles). Implement runtime sidecar (claims/leases). Add comprehensive tests.

### Phase 3: CLI MVP

Implement `epos` CLI with: `new`, `edit`, `show`, `validate`, `lint`, `ready`, `blocked`, `claim`, `release`, `close`, `reopen`, `export`, and `tui`. Machine-readable JSON output for non-interactive commands; `tui` is interactive and rejects `--json`.

#### TUI command contract

`epos tui [parent]` opens the Bubble Tea ticket operator UI. The optional `parent` limits the view to children of that ticket. `--owner` sets the owner used by claim/release actions; when omitted, owner resolution checks `EPOS_OWNER`, then `USER`, then `USERNAME`. `--refresh` accepts a Go duration, defaults to `5s`, and `0` disables polling.

The TUI presents a grouped two-pane view: ticket groups (`ready`, `blocked`, `claimed`, `open`, `closed`, `all`) on the left and selected-ticket detail on the right. Key actions: `j`/`k` or arrows move selection, `tab`/`shift+tab` switch groups, `/` searches, `r` refreshes, `enter` opens detail, `n` adds a note, `c`/`u` claim/release, `x`/`o` close/reopen, `esc` cancels the active mode, and `q`/`ctrl+c` quits. The TUI has no JSON output mode.

### Phase 4: `fabrikk` integration

Replace `fabrikk/internal/ticket` with `epos/ticket`. Add adapter layer during transition. Ensure fabrikk emits canonical tickets directly.

### Phase 5: `verk` integration

Replace `verk/internal/adapters/ticketstore/tkmd` with `epos/ticket`. Keep verk's runtime policy engine. Move claims/leases to shared runtime model.

### Phase 6: Skill + `vakt` read integration

Wrap CLI with an agent skill. Make ticket context available to vakt read-only.

---

## 4. Epic & Task Breakdown

### Epic E1: Bootstrap `epos` repo

| ID | Task | Details | Exit Criteria |
|----|------|---------|---------------|
| E1-T1 | Initialize Go module | `go mod init github.com/php-workx/epos`, create directory structure per §1 | `go build ./...` succeeds |
| E1-T2 | Add dependencies | `gopkg.in/yaml.v3`, `github.com/gofrs/flock`, `cobra` for CLI | `go mod tidy` succeeds |
| E1-T3 | Add CI | GitHub Actions: `go build`, `go test`, `go vet`, `gofumpt` | CI green on push |
| E1-T4 | Add `.gitignore` | `.tickets/*.lock` for flock lock files | `.gitignore` exists with lock pattern |
| E1-T5 | Add README | Purpose, architecture, usage, migration guide | Reviewed |

### Epic E2: Canonical schema + markdown package

| ID | Task | Details | Exit Criteria |
|----|------|---------|---------------|
| E2-T1 | Define core types | `Ticket` (with exported `Present map[string]bool` and `TitleDerived bool`), `NewTicket()` constructor with `TicketOption` pattern, `Status` (with `StatusHeld` replacing extended `StatusBlocked`), `TaskScope`, `ImplementationDetail`, `ValidationCheck`, `LearningRef`, `FileChange` in `ticket/tickets.go` | Types compile, JSON tags match ADR, NewTicket sets Present correctly |
| E2-T2 | Define runtime types | `RuntimeState`, `Claim`, `Lease`, `Heartbeat` in `ticket/runtime.go` | Types compile |
| E2-T3 | Define custom error types | `TicketNotFoundError`, `AmbiguousIDError`, `IDCollisionError`, `CorruptYAMLError`, `CycleDetectedError`, `AlreadyClaimedError`, etc. in `ticket/errors.go` — custom structs with structured fields, not sentinel `errors.New` | Errors compile, implement `error` interface, CLI uses type assertions for exit codes |
| E2-T4 | Implement frontmatter marshal | `MarshalTicket(Ticket) ([]byte, error)` in `ticket/markdown/frontmatter.go` — implements custom `MarshalYAML()` returning `*yaml.Node` with: canonical field ordering, sorted Extra keys, present-map-aware emission, TitleDerived suppression | Round-trip test passes, deterministic output test passes |
| E2-T5 | Implement frontmatter unmarshal | `UnmarshalTicket(data []byte) (*Ticket, error)` in `ticket/markdown/frontmatter.go` — populates `Present` map, sets `TitleDerived` | Round-trip test passes, Present map populated correctly |
| E2-T6 | Implement status mapping | `StatusToTK()`, `StatusFromTK()`, extended status bidirectional mapping in `ticket/markdown/compat.go` | All 8 extended statuses + StatusHeld map correctly to 3 tk statuses and back |
| E2-T7 | Implement unknown field preservation | `Extra map[string]any` with `yaml:",inline"` pass-through | Write ticket with extra fields, read back, fields preserved |
| E2-T8 | Implement full markdown round-trip | `MarshalTicket(Ticket) ([]byte, error)`, `UnmarshalTicket(data []byte) (*Ticket, error)` — includes body, scope sections, etc. | Full round-trip: Ticket → markdown → Ticket = identical, re-marshal produces identical bytes |
| E2-T9 | Implement body section rendering | Scope, Acceptance Criteria, Validation, Implementation Detail, Learning Context, Notes | Sections render correctly |
| E2-T10 | Implement UpdateFrontmatter | Read-modify-write that preserves body and unknown frontmatter fields | Existing body and unknown fields survive update |
| E2-T11 | tk CLI compatibility tests | Write Go-generated ticket, verify `tk show` reads it; write with `tk create`, verify Go reads it | Cross-CLI compatibility confirmed |
| E2-T12 | Implement validation | `Validate(ticket Ticket) []ValidationError` — required fields, status values, type field validation against `validTicketTypes`, owned path validation, dep cycle detection | Invalid tickets rejected, valid tickets pass |

### Epic E3: Store + graph + runtime

| ID | Task | Details | Exit Criteria |
|----|------|---------|---------------|
| E3-T1 | Implement FileStore CRUD | `NewStore(dir)`, `CreateRun`, `ReadTasks`, `WriteTasks`, `ReadTask`, `WriteTask`, `UpdateStatus` in `ticket/store/` | CRUD tests pass |
| E3-T2 | Implement ID generation & resolution | `GenerateID(dir)`, `ResolveID(dir, partial)`, `ValidateID(id)` in `ticket/store/id.go` | Collision detection, partial matching, ambiguity detection work |
| E3-T3 | Implement atomic writes | `atomicWrite(path, data)` with temp file + fsync + rename + dir fsync in `ticket/store/atomic.go` | Crash-recovery test passes |
| E3-T4 | Implement file locking | `withLock(path, fn)`, `withLocks(paths, fn)` using `flock` | Concurrent write test passes |
| E3-T5 | Implement AddNote | Append timestamped note to `## Notes` section | Notes accumulate correctly |
| E3-T6 | Implement AddDep / RemoveDep | Directional dependency management, with existence validation | Dep operations work |
| E3-T7 | Implement Link / Unlink | Bidirectional link management | Links work symmetrically |
| E3-T8 | Implement dependency graph (pure only) | `ReadyFilter`, `BlockedFilter` with unresolved-dependency and parent-blocked behavior, `DetectCycles`, `FilterChildren`, `FilterReadyChildren` with parent scoping in `ticket/graph/` — pure functions only, no I/O | All graph tests pass |
| E3-T9 | Implement child discovery (I/O-bound) | `ListAllChildren`, `ListReadyChildren`, `HasChildren` in `ticket/store/store.go` — store wrappers around graph child filters since they require filesystem I/O | Parent-child hierarchy works |
| E3-T10 | Implement runtime sidecar | `ReadRuntimeState`, `WriteRuntimeState` — JSON files in `.tickets/.claims/` | Claim read/write works |
| E3-T11 | Implement claim/release | `Claim(ticketID, ownerID, backend, lease)` with eligibility validation (only pending/repair_pending, same-owner re-claim idempotent), `Release(ticketID, ownerID, newStatus, reason)` — sidecar-based, no frontmatter mutation | Claims work, frontmatter untouched, ineligible status rejected, same-owner re-claim extends lease |
| E3-T11b | Implement claim lifecycle | `ReclaimExpired(runID)`, `ReadClaimsForRun(runID)`, `DefaultLeaseDuration = 15*time.Minute`, `validateClaimEligibility` | Expired claims reclaimed, run claims readable, default lease enforced |
| E3-T12 | Implement orphan cleanup | `removeOrphanedTasks` — from fabrikk's WriteTasks | Old tasks cleaned up on recompile |
| E3-T13 | Implement test utilities | `NewTestStore`, `NewTestTicket`, `NewTestTicketWithStatus` in `ticket/testutil/` | Helpers compile and work in test files |
| E3-T14 | Comprehensive tests | CRUD, concurrent writes, partial ID, cycle detection, ready/blocked, sidecar claims, custom error type assertions | All tests pass |

### Epic E4: CLI MVP

| ID | Task | Details | Exit Criteria |
|----|------|---------|---------------|
| E4-T1 | CLI scaffold | `cobra`-based CLI in `cmd/epos/`, root command, --json flag | `epos --help` works |
| E4-T2 | `epos new` | Create a new ticket with required fields | Ticket file created with correct frontmatter |
| E4-T3 | `epos show` | Display a ticket by ID (partial matching) | Ticket displayed, JSON output available |
| E4-T4 | `epos validate` / `epos lint` | Validate ticket schema, check for issues | Errors reported with deterministic exit codes |
| E4-T5 | `epos ready` / `epos blocked` | List ready/blocked tickets with dependency analysis | Correct lists, JSON output |
| E4-T6 | `epos claim` / `epos release` | Claim/release tickets via sidecar | Sidecar files created/removed, frontmatter untouched |
| E4-T7 | `epos close` / `epos reopen` | Status transitions | Status changes with optional reason |
| E4-T8 | `epos export` | Export tickets as JSON for agent consumption | Deterministic JSON output |
| E4-T9 | Deterministic exit codes | 0 = success, 1 = general error, 2 = validation error, 3 = not found, 4 = ambiguous ID | Exit codes match spec |
| E4-T10 | Integration tests | Full CLI workflows tested end-to-end | All commands work together |

### Epic E5: `fabrikk` integration

| ID | Task | Details | Exit Criteria |
|----|------|---------|---------------|
| E5-T1 | Add `epos` as a dependency | `go get github.com/php-workx/epos` in fabrikk | Dependency resolves |
| E5-T2 | Create adapter in fabrikk | Thin adapter that implements `state.TaskStore` using `epos/ticket` | Adapter compiles |
| E5-T3 | Wire adapter into fabrikk CLI | Replace `ticket.NewStore` with `epos` adapter | All fabrikk tests pass |
| E5-T4 | Validate round-trip | fabrikk compile → epos ticket → epos show = correct | No data loss |
| E5-T5 | Remove `fabrikk/internal/ticket` | Delete old package, update all import paths | No references to `internal/ticket` remain |
| E5-T6 | Remove inline claim fields from fabrikk | Stop writing `claimed_by`, `claim_backend`, etc. to frontmatter; use epos sidecar instead | Claim data lives in sidecars only |

### Epic E6: `verk` integration

| ID | Task | Details | Exit Criteria |
|----|------|---------|---------------|
| E6-T1 | Add `epos` as a dependency | `go get github.com/php-workx/epos` in verk | Dependency resolves |
| E6-T2 | Create adapter in verk | Thin adapter that bridges verk's ticket interface to `epos/ticket` | Adapter compiles |
| E6-T3 | Wire adapter into verk engine | Replace `tkmd` store with epos adapter | All verk tests pass |
| E6-T4 | Migrate claim logic to epos sidecar | Move verk's `.claims/` JSON logic to epos `ticket/runtime` | Claims work through epos |
| E6-T5 | Remove `verk/internal/adapters/ticketstore/tkmd` | Delete old package, update all import paths | No references to `tkmd` remain |
| E6-T6 | Validate verk intake | verk consumes canonical tickets directly, no lossy translation | verk ready/blocked/claim works |

### Epic E7: Skill + vakt read integration

| ID | Task | Details | Exit Criteria |
|----|------|---------|---------------|
| E7-T1 | Define skill contract | Agent skill wraps CLI, enforces "no freehand ticket editing" | Skill spec documented |
| E7-T2 | Implement skill | Skill uses `epos` CLI for all ticket mutations | Skill creates/updates tickets via CLI only |
| E7-T3 | vakt read-only integration | vakt reads tickets via `epos` for review context | vakt can query ticket relationships and scope |

---

## 5. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| `tk` CLI changes YAML format | Go parser breaks | Pin to known format, compatibility tests, `Extra` catch-all field |
| Concurrent agent writes | Data loss | Per-file flock locking (proven in fabrikk) |
| Two-YAML-parser reconciliation | Merge conflicts | Use `yaml.v3` exclusively (fabrikk's approach); drop verk's hand-rolled parser |
| Inline claim fields in fabrikk frontmatter | Migration confusion | Phase E5-T6 explicitly removes inline claim writes; Phase 1 keeps claims/leases sidecar-only in epos runtime |
| `fabrikk` `state.Task` type coupling | Breaking changes in fabrikk | Epos defines its own `Ticket` type; fabrikk adapter maps between `state.Task` and `ticket.Ticket` during E5 |
| `verk` 5-status vs `fabrikk` 9-status | Status mapping conflicts | epos supports both: 5 tk-native statuses + 8 extended statuses (StatusHeld replaces extended StatusBlocked) with bidirectional mapping |
| Non-deterministic YAML output | Noisy diffs in version control | Custom `MarshalYAML()` returning `*yaml.Node` with sorted Extra keys and present-map-aware emission |
| Unexported `present`/`titleDerived` fields | Programmatic ticket creation breaks round-trip | Exported as `Present` and `TitleDerived` with `NewTicket()` constructor and `TicketOption` pattern |
| I/O in graph package | Untestable, circular dependencies | `graph/` contains only pure functions on `[]Ticket`; I/O-bound child discovery lives in `store/` |

---

## 6. Verification Checklist

- [ ] `go build ./...` — all packages compile
- [ ] `go test ./...` — all tests pass
- [ ] `go test ./ticket/... -v -run TestNewTicket` — constructor tests pass
- [ ] `go test ./ticket/... -v -run TestStatusConstants` — no status string collisions
- [ ] `go test ./ticket/... -v -run TestCustomErrorTypeAssertions` — error types work for CLI exit codes
- [ ] `ticket/graph/` has no I/O imports (no `os`, `io/ioutil`, `filepath`)
- [ ] Custom error types produce correct CLI exit codes via type assertions
- [ ] `NewTicket()` constructor correctly populates `Present` map
- [ ] `MarshalYAML()` produces deterministic output (sorted Extra keys)
- [ ] `Present`-map-aware emission: absent fields not written to frontmatter
- [ ] `TitleDerived` suppresses `title:` in frontmatter when title came from body
- [ ] `MarshalTicket` does not duplicate `# Title` heading when `TitleDerived == true`
- [ ] Inline claim field preservation from fabrikk is NOT ported (claims in sidecars only)
- [ ] `StatusHeld = "held"` maps to tk "blocked" and back
- [ ] No `StatusBlocked` in extended status set (only in tk-native set)
- [ ] `Links []string` field exists on Ticket struct and Link/Unlink operate on it
- [ ] `Claim` rejects ineligible statuses (not pending/repair_pending)
- [ ] `Claim` allows same-owner idempotent re-claim
- [ ] `Release` takes `newStatus` and `reason` parameters
- [ ] `ReclaimExpired` and `ReadClaimsForRun` exist in runtime package
- [ ] `DefaultLeaseDuration = 15 * time.Minute` constant defined
- [ ] `epos new` creates a ticket that `tk show` can read
- [ ] `tk create` creates a ticket that `epos show` can read
- [ ] Full round-trip: Ticket → markdown → Ticket = identical (re-marshal produces identical bytes)
- [ ] Claims live in `.tickets/.claims/`, not in frontmatter
- [ ] `epos ready` and `epos blocked` work without repo-specific adapters
- [ ] `epos claim` / `epos release` do not rewrite the canonical ticket file
- [ ] fabrikk emits canonical tickets that verk can consume directly
- [ ] No duplicate schema definitions across repos
