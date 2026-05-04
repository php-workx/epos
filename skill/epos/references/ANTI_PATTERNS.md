# Anti-Patterns

Common mistakes that bypass epos invariants and cause data inconsistency. Avoid these.

---

## Critical

### 1. Direct file edits on `.tickets/`

**DON'T:** Use `apply_patch`, `sed`, `cat >`, or any tool that writes directly to `.tickets/*.md` files.

```bash
# WRONG — bypasses atomic writes, skips sidecar updates
sed -i '' 's/open/closed/' .tickets/epo-xxx.md
```

```python
# WRONG — direct file mutation
apply_patch("*** Update File: .tickets/epo-xxx.md\n@@ ...")
```

**DO:** Use `epos edit` for all changes.

```bash
# CORRECT — atomic write, sidecar-aware, validated
epos edit epo-xxx --body "Updated description"
epos close epo-xxx -r "Done"
```

**Why it breaks:** epos uses atomic writes (`ticket/store/atomic.go`) with file locking and maintains sidecar runtime state (`.tickets/.claims/`). Direct edits bypass both, causing silent data loss and claim conflicts.

### 2. Burying structured metadata in body text

**DON'T:** Put structured information in the `--body` field when a dedicated flag exists.

```bash
# WRONG — metadata is invisible to query tools
epos new "Fix auth" --body "Priority: high\nDepends on: epo-abc\nTags: backend, security"
```

**DO:** Use dedicated flags.

```bash
# CORRECT — queryable, validatable
epos new "Fix auth" --priority 3 --deps epo-abc --tags backend,security --body "The token..."
```

**Why it breaks:** `epos ready`, `epos blocked`, and `epos lint` query structured fields. Metadata buried in prose is invisible to dependency resolution, status queries, and validation.

### 3. Editing claim sidecar files directly

**DON'T:** Touch `.tickets/.claims/*.json` files.

```bash
# WRONG — claim state becomes inconsistent
echo '{"owner": "me"}' > .tickets/.claims/epo-xxx.json
```

**DO:** Use `epos claim` / `epos release`.

```bash
# CORRECT
epos claim epo-xxx -o agent-1
epos release epo-xxx -o agent-1
```

**Why it breaks:** Claim files have a specific schema with expiry timestamps. Manual edits create unexpirable claims or orphaned locks.

### 4. Skipping validation before execution

**DON'T:** Pick up a ticket and start implementing without validating.

```bash
# WRONG — ticket may have schema errors or missing fields
epos claim epo-xxx -o agent-1
# ... start coding immediately ...
```

**DO:** Validate after every mutation.

```bash
# CORRECT
epos claim epo-xxx -o agent-1
epos validate epo-xxx --json
# Check JSON payload for errors, fix if needed
# ... only then start coding ...
```

**Why it breaks:** Unvalidated tickets may have malformed deps, missing required fields, or invalid status transitions. This causes downstream failures in `epos ready`/`blocked` and confuses other agents.

### 5. Mixing runtime state into frontmatter

**DON'T:** Add claim-related fields to YAML frontmatter.

```yaml
# WRONG — claim info in frontmatter (old fabrikk pattern)
---
id: epo-xxx
claimed_by: agent-1
claim_expires: "2026-05-04T12:00:00Z"
---
```

**DO:** Claims live exclusively in `.tickets/.claims/<id>.json`. The CLI manages them.

**Why it breaks:** ADR 0004 decision D6: claims/leases are sidecar-only. Inline claim fields create conflicts between the canonical ticket and runtime state.

---

## Moderate

### 6. Using `--json` but checking exit codes for validation

**DON'T:** Rely on exit codes after JSON-mode validation.

```bash
# WRONG — both exit 0 even with errors
epos lint --json
if [ $? -ne 0 ]; then ...  # never triggers
```

**DO:** Inspect the JSON payload.

```bash
# CORRECT
result=$(epos lint --json)
echo "$result" | jq '.tickets'  # null = clean, array = errors
```

**Why it breaks:** In JSON mode, `epos lint` and `epos validate` always exit 0. Validation errors are in the payload, not the exit code.

### 7. Reconstructing tickets from memory instead of reading

**DON'T:** Assume ticket state from memory.

```text
# WRONG — state may have changed since you last looked
"I know epo-xxx is still open and has no deps"
```

**DO:** Always re-read before acting.

```bash
# CORRECT — authoritative read
epos show epo-xxx --json
```

**Why it breaks:** Other agents may have modified the ticket (added deps, closed it, claimed it). Stale assumptions cause duplicate work or claim conflicts.

### 8. Not updating tickets after compaction

**DON'T:** Resume work after context loss without checking ticket state.

**DO:** Run `epos ready --json` and `epos export --json` to reconstruct your working set.

Tickets are your persistent memory across compactions.

---

## Environment Constraints

### 9. Assuming `tui` is available agent-side

**DON'T:** Try to use `epos tui` in non-interactive contexts.

```bash
# WRONG — TUI requires a terminal
epos tui  # fails or hangs in agent mode
```

**DO:** Use CLI commands with `--json` for all agent interactions.

**Why:** `tui` is deliberately excluded from the skill command surface. It requires an interactive terminal.
