# Changelog

## 0.3.0 - 2026-05-13

### Features
- Add a `TicketPatch` type for partial ticket updates: decode a JSON object and apply only the fields that were present, leaving everything else untouched.
- Add predictable exit codes for every error condition: 2 for validation failures, 3 for ticket not found, 4 for ambiguous ID prefix, 5 for dependency cycles, 6 for claim conflicts. All other errors exit 1.
- Add library-level helpers for querying ready (unblocked, unclaimed) tickets — useful for callers building agent or automation integrations.

### Fixes
- Fix error output: errors now go exclusively to stderr; usage text is no longer printed on runtime failures. Stdout stays clean for piping.
- Fix tag deduplication: whitespace variants like `"foo"` and `" foo"` are now correctly detected as duplicates; tags are stored with whitespace stripped.
- Fix title validation: titles containing only whitespace are now rejected.
- Fix ticket store: returned ticket slices are now deep-copied so callers cannot accidentally mutate stored data.

## 0.2.5 - 2026-05-08

### Fixes
- Fix shell installer cleanup under `set -u`.
- Remove the stale Homebrew cask when publishing the Formula.

## 0.2.4 - 2026-05-08

### Fixes
- Switch Homebrew distribution from cask to Formula.
- Add macOS/Linux and Windows install scripts backed by GitHub Release archives.
- Rename release archives with lower-case platform names for Mise and installer compatibility.

### Documentation
- Document Homebrew Formula, install script, PowerShell, Mise, and Go install paths.

## 0.2.3 - 2026-05-08

### Fixes
- Check out the requested tag when manually rerunning release workflows.

## 0.2.2 - 2026-05-08

### Fixes
- Support safe reruns of release workflows for existing tags.

## 0.2.1 - 2026-05-08

### Fixes
- Add release packaging for GitHub artifacts and Homebrew cask installation.
- Wire CLI version metadata into release builds.

### Documentation
- Document Homebrew and Go install commands.

## 0.2.0 - 2026-05-08

### Features
- Add caller-supplied lease ID support for runtime claims.
- Harden runtime sidecar path containment for symlinked ticket roots.
- Make `epos ready` sidecar-aware so active claims are filtered by default, with `--include-claimed` for debugging.

### Documentation
- Document sidecar-aware `epos ready` filtering in the README.
