# Changelog

## 0.2.0 - 2026-05-08

### Features
- Add caller-supplied lease ID support for runtime claims.
- Harden runtime sidecar path containment for symlinked ticket roots.
- Make `epos ready` sidecar-aware so active claims are filtered by default, with `--include-claimed` for debugging.

### Documentation
- Document sidecar-aware `epos ready` filtering in the README.
