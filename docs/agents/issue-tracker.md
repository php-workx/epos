# Issue tracker: epos CLI

Issues for this repo are managed with the `epos` CLI — the repo is its own dogfood store. Tickets live in `.tickets/` at the repo root.

## Creating issues

```bash
epos new "<title>" [--type task|epic|feature|bug|chore|spike|doc] [--tags "..."] [--priority N]
```

Common flags: `--body`, `--intent`, `--ac`, `--deps`, `--parent`.

## Fetching a ticket

```bash
epos show <id>          # human-readable
epos show --json <id>   # machine-readable
epos export             # all tickets as JSON
```

## Updating a ticket

```bash
epos edit <id> [--tags "..."] [--priority N] [--assignee "..."]
```

## Closing / won't fix

```bash
epos close <id>
```

## Listing ready work

```bash
epos ready              # unblocked, unclaimed tickets
```

## Triage state

Applied as tags via `epos edit <id> --tags "..."`. See `triage-labels.md` for the label strings.
