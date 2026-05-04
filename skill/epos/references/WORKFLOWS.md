# Workflows

Step-by-step workflows for common epos usage patterns.

## Contents

- [Session Start](#session-start)
- [Creating an Epic with Children](#epic-with-children)
- [Claim → Work → Close](#claim-work-close)
- [Unhappy Paths](#unhappy-paths)
- [Dependency Management](#dependency-management)
- [Multi-Repo Work](#multi-repo)
- [Post-Compaction Recovery](#compaction)

## Conventions

These workflows assume:

- `AGENT_ID` is set to a stable per-session owner string (e.g. `agent-impl-3f7a`)
- `--json | jq -r .id` captures new ticket IDs — never parse human output
- Exit codes: `0`=ok, `1`=generic (incl. already-claimed), `2`=validation, `3`=not-found, `4`=ambiguous-ID

## Session Start {#session-start}

```bash
# 0. Detect: is this an epos repo?
[ -d .tickets ] || { echo "no .tickets/ — not an epos repo"; exit 0; }

# 1. Set a stable owner for the session
AGENT_ID="agent-impl-$(date +%s | tail -c 5)"

# 2. See what's ready (sorted by priority desc, ID asc)
epos ready --json | jq -r '.[] | "\(.id)\t\(.priority)\t\(.title)"'

# 3. If nothing ready, see what's blocked and on what
epos blocked --json | jq -r '.[] | "\(.id)\tdeps=\(.deps|join(","))"'

# 4. Pick top, inspect, claim
id=<chosen-id>
epos show "$id" --json
epos claim "$id" -o "$AGENT_ID" || echo "already claimed — pick another"
```

`ready` does not consult sidecars, so a listed ticket may already be claimed by another agent. Always *attempt* the claim and skip on failure.

## Creating an Epic with Children {#epic-with-children}

```bash
# 1. Create the epic and capture its ID
epic=$(epos new "API Overhaul" --type epic --priority 3 \
  --body "Modernize the REST API with versioning and pagination" \
  --json | jq -r .id)

# 2. Create child tickets under the epic
v=$(epos new "Add API versioning middleware" --parent "$epic" --priority 2 \
  --type feature --body "..." --json | jq -r .id)
p=$(epos new "Cursor-based pagination" --parent "$epic" --priority 2 --deps "$v" \
  --type feature --body "..." --json | jq -r .id)

# 3. Validate the structure (cycles, missing parents, malformed deps)
epos lint --json | jq '.tickets'   # null = clean

# 4. Check ready children under the epic
epos ready "$epic" --json
```

## Claim → Work → Close {#claim-work-close}

```bash
id=epo-xxx

# 1. Claim before working — exits 1 if another agent holds it
epos claim "$id" -o "$AGENT_ID" || { echo "claim failed; pick another"; exit 0; }

# 2. Validate before execution
epos validate "$id" --json

# 3. Do the work...
#    For long work (>10 min), heartbeat the lease in a background loop:
#    while kill -0 $$ 2>/dev/null; do epos claim "$id" -o "$AGENT_ID"; sleep 300; done &

# 4. Append a note about what was done
epos edit "$id" --note "Implemented token validation middleware with expiry check"

# 5. Close (this also clears the claim sidecar)
epos close "$id" -r "Fixed token validation; added expiry check middleware"

# 6. Run lint to catch introduced cycles
epos lint --json | jq '.tickets'   # null = clean
```

**Never skip `epos claim`** — without it, another agent may pick up the same ticket. **Never force-claim** on a conflict; pick another ready ticket instead.

**Lease**: 15 minutes from the last `epos claim`. Re-claim with the same `-o` value to extend (idempotent heartbeat). There is no separate `renew` command.

## Unhappy Paths {#unhappy-paths}

### Claim conflict — another agent has it

```bash
epos claim "$id" -o "$AGENT_ID"
# stderr: ticket already claimed by agent-other
# exit:   1
```

**Do**: skip and pick the next ticket from `epos ready --json`. **Don't**: force-claim, edit the sidecar, or wait in a tight loop.

### Validation fails after claim

```bash
epos claim "$id" -o "$AGENT_ID"
epos validate "$id" --json | jq '.errors'
# [ { "field": "deps", "message": "ticket epo-yyy not found" } ]
```

If the fix is small (typo, missing field) and within scope, fix it with `epos edit` and re-validate. If the fix requires decisions you cannot make autonomously, release the ticket with a diagnostic note:

```bash
epos edit "$id" --note "Validation failed: dep epo-yyy missing. Releasing for triage."
epos release "$id" -o "$AGENT_ID"
```

### Hit a blocker mid-work

```bash
epos edit "$id" --note "Blocker: needs schema decision from owner. Tried X, Y."
epos release "$id" -o "$AGENT_ID"
# Status returns to pending — ticket reappears in epos ready
```

`release` resets status to `pending`, not `blocked`. If the ticket should be considered blocked by an open dep, add it via `epos edit --deps`.

### Lease expired mid-work

The 15-minute window passed without a re-claim. Sidecar may already have been claimed by another agent. Re-attempt:

```bash
epos claim "$id" -o "$AGENT_ID"
# If exits 1: another agent took it. Stop work, do not commit, pick a new ticket.
# If exits 0: lease re-acquired. Continue, set up a heartbeat loop this time.
```

## Dependency Management {#dependency-management}

### Adding dependencies

```bash
# Set deps when creating
epos new "Feature B" --deps epo-aaa,epo-bbb

# Or add via edit (replaces all deps — include existing ones)
epos edit epo-ccc --deps epo-aaa,epo-bbb,epo-new
```

### Removing a single dependency

`--deps` replaces the full list — there is no `--remove-dep`. Reconstruct:

```bash
# Read current deps, drop the one to remove, re-set as comma-separated list
remaining=$(epos show epo-ccc --json | jq -r '.deps - ["epo-new"] | join(",")')
epos edit epo-ccc --deps "$remaining"
```

### Checking the graph

```bash
# Find tickets ready to work (all deps resolved)
epos ready --json

# Find tickets blocked by unresolved deps
epos blocked --json

# Find cycles (should return null if clean)
epos lint --json
```

## Multi-Repo Work {#multi-repo}

```bash
# Point --dir at any repo's .tickets/
epos --dir ../other-project ready --json
epos -d /absolute/path/to/repo show epo-xxx --json

# Default is current directory
epos ready --json  # same as epos --dir . ready --json
```

## Post-Compaction Recovery {#compaction}

After context compaction, ticket files persist on disk. Reconstruct your working set:

```bash
# 0. Restore your AGENT_ID — must match the value used before compaction
AGENT_ID="agent-impl-3f7a"   # whatever was stable for this session

# 1. Find tickets you still hold by reading sidecars
ls .tickets/.claims/ 2>/dev/null | while read f; do
  id="${f%.json}"
  owner=$(jq -r '.claim.claimed_by // empty' ".tickets/.claims/$f")
  [ "$owner" = "$AGENT_ID" ] && echo "$id"
done

# 2. Read the last note on each held ticket
epos show "$id" --json | jq '.notes[-1]'

# 3. Heartbeat the lease before resuming (it may have expired during compaction)
epos claim "$id" -o "$AGENT_ID" || echo "lost — another agent took it"

# 4. If nothing held, pick fresh from ready
epos ready --json
```

Tickets are plain markdown files — they survive compaction, session boundaries, and machine changes. Sidecar leases do not — assume any work paused longer than 15 minutes has lost its claim.
