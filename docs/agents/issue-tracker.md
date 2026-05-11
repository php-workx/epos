# Issue tracker: epos CLI

Issues for this repo are managed with the `epos` CLI — the repo is its own dogfood store. Tickets live in `.tickets/` at the repo root.

## Creating issues

```
epos new "<title>" [--type task|epic|feature|bug|chore|spike|doc] [--tags "..."] [--priority N]
```

Common flags: `--body`, `--intent`, `--ac`, `--deps`, `--parent`.

## Fetching a ticket

```
epos show <id>          # human-readable
epos show --json <id>   # machine-readable
epos export             # all tickets as JSON
```

## Updating a ticket

```
epos edit <id> [--tags "..."] [--priority N] [--assignee "..."]
```

## Closing / won't fix

```
epos close <id>
```

## Listing ready work

```
epos ready              # unblocked, unclaimed tickets
```

## Triage state

Applied as tags via `epos edit <id> --tags "..."`. See `triage-labels.md` for the label strings.
