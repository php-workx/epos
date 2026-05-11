# Store interface omits Dir() getter

`FileStore` has a public `Dir string` field that the TUI and runtime layers use to locate sidecar files. When we extracted the `Store` interface, we chose not to include a `Dir() string` method on it.

`Dir` is a filesystem concept — an implementation detail of `FileStore`. Including it on `Store` would force every adapter (including `MemStore` and any future non-filesystem backend) to carry a concept that is meaningless outside the file-based implementation. The TUI genuinely requires a filesystem-backed store for sidecar operations; that dependency is better expressed explicitly by keeping `NewStoreDataSource` typed as `*FileStore` than by pretending it is a general `Store` constraint.

## Considered Options

**Include `Dir() string` on `Store`** — would let `cmd/epos/tui.go` use `storeFromFlag()` uniformly. Rejected because it leaks a filesystem concept into an interface that should be storage-agnostic, and because `MemStore.Dir()` would return a meaningless empty string, making the interface a lie for in-memory adapters.

## Consequences

`cmd/epos/tui.go` constructs its own `*FileStore` directly (bypassing `storeFromFlag`) to satisfy `NewStoreDataSource(*FileStore)`. This is the one command that cannot be driven by an arbitrary `Store` — that constraint is now visible at the call site rather than hidden behind the interface.
