# AGENTS.md

`AGENTS.md` is the durable instruction file for this repo.

## What This Repo Is

`epos` is the planned shared ticket system for the wider toolchain. Right now
the repo is still in the plan-first stage, so the implementation contract lives
primarily in the checked-in plan document.

## Hard Rules

- Treat `IMPLEMENTATION_PLAN.md` as the current source of truth.
- Do not invent alternate package layouts, schema spines, or migration strategy
  without updating the plan.
- Preserve the Phase 1 compatibility goals described in the plan, especially
  tk-style markdown compatibility and sidecar runtime state for claims/leases.
- Keep repo instructions short until the codebase exists; otherwise the docs
  will rot immediately.

## Current Workflow

1. Start from `IMPLEMENTATION_PLAN.md`.
2. If a task changes the ticket model, CLI contract, or migration strategy,
   update the plan alongside the code.
3. Prefer incremental implementation that keeps the plan and repo aligned.

## References

- `IMPLEMENTATION_PLAN.md` — current implementation contract
