# Changelog

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
