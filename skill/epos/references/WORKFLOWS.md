# Workflows

Step-by-step workflows for common epos usage patterns.

## Contents

- [Session Start](#session-start)
- [Creating an Epic with Children](#epic-with-children)
- [Claim → Work → Close](#claim-work-close)
- [Dependency Management](#dependency-management)
- [Multi-Repo Work](#multi-repo)
- [Post-Compaction Recovery](#compaction)

## Session Start {#session-start}

When starting work in a repo that has `.tickets/`:

```bash
# 1. See what's ready to work
epos ready --json

# 2. If nothing ready, check what's blocked
epos blocked --json

# 3. Pick a ticket and inspect it
epos show <id> --json
```

Always prefer `--json` for programmatic consumption.

## Creating an Epic with Children {#epic-with-children}

```bash
# 1. Create the epic
epos new "API Overhaul" --type epic --priority 3 --body "Modernize the REST API with versioning and pagination"

# 2. Create child tickets (capture the epic ID from step 1)
epos new "Add API versioning middleware" --parent <epic-id> --priority 2 --type feature --body "..."
epos new "Implement cursor-based pagination" --parent <epic-id> --priority 2 --type feature --body "..."

# 3. Validate the structure
epos lint --json

# 4. Check what children are ready (none yet — all open, no deps)
epos ready <epic-id> --json
```

## Claim → Work → Close {#claim-work-close}

```bash
# 1. Claim a ticket before working on it
epos claim epo-xxx -o agent-1

# 2. Validate before execution
epos validate epo-xxx --json

# 3. Do the work...

# 4. Add a note about what was done
epos edit epo-xxx --note "Implemented token validation middleware with expiry check"

# 5. Close the ticket
epos close epo-xxx -r "Fixed token validation; added expiry check middleware"

# 6. Run lint to catch introduced cycles
epos lint --json
```

**Important:** Never skip `epos claim`. Without a claim, another agent may pick up the same ticket. Claim conflicts (exit code 3) are hard stops — do not force-claim.

## Dependency Management {#dependency-management}

### Adding dependencies

```bash
# Set deps when creating
epos new "Feature B" --deps epo-aaa,epo-bbb

# Or add via edit (replaces all deps — include existing ones)
epos edit epo-ccc --deps epo-aaa,epo-bbb,epo-new
```

### Removing a single dependency

```bash
# 1. Read current deps
deps=$(epos show epo-ccc --json | jq -r '.deps[]')

# 2. Reconstruct list minus the one to remove
epos edit epo-ccc --deps epo-aaa,epo-bbb  # exclude epo-new
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

After context compaction, ticket files persist:

```bash
# 1. Find tickets you claimed
epos export --json | jq '.[] | select(.extended_status == "claimed")'

# 2. Read the last notes on claimed tickets
epos show <id> --json | jq '.notes[-1]'

# 3. Check what's ready to continue
epos ready --json
```

Tickets are plain markdown files — they survive compaction, session boundaries, and machine changes.
