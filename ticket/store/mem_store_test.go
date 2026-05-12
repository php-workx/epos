package store_test

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/store"
	"github.com/php-workx/epos/ticket/testutil"
)

// ─── Tracer bullet ───────────────────────────────────────────────────────────

func TestMemStoreCreate(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("hello world")
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read after Create: %v", err)
	}
	if got.Title != tk.Title {
		t.Errorf("Title: got %q, want %q", got.Title, tk.Title)
	}
	if got.ID != tk.ID {
		t.Errorf("ID: got %q, want %q", got.ID, tk.ID)
	}
}

func TestMemStoreCreateConflict(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("conflict")
	if err := m.Create(tk); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	err := m.Create(tk)
	if err == nil {
		t.Fatal("expected IDCollisionError, got nil")
	}
	if _, ok := err.(*ticket.IDCollisionError); !ok {
		t.Errorf("expected *IDCollisionError, got %T: %v", err, err)
	}
}

func TestMemStoreRead(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("isolation")
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	// Mutate the returned copy — the stored version must remain unchanged.
	got.Title = "mutated"
	got2, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if got2.Title == "mutated" {
		t.Error("Read returned same pointer as internal store — isolation violated")
	}
}

func TestMemStoreReadNotFound(t *testing.T) {
	m := store.NewMemStore()
	_, err := m.Read("epo-does-not-exist")
	if err == nil {
		t.Fatal("expected TicketNotFoundError, got nil")
	}
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("expected *TicketNotFoundError, got %T: %v", err, err)
	}
}

func TestMemStoreUpdate(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("update me")
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	tk.Title = "updated title"
	tk.Present["title"] = true
	if err := m.Update(tk); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read after Update: %v", err)
	}
	if got.Title != "updated title" {
		t.Errorf("Title after Update: got %q, want %q", got.Title, "updated title")
	}
}

func TestMemStoreUpdateNotFound(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("ghost")
	err := m.Update(tk)
	if err == nil {
		t.Fatal("expected TicketNotFoundError, got nil")
	}
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("expected *TicketNotFoundError, got %T: %v", err, err)
	}
}

func TestMemStoreDelete(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("delete me")
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.Delete(tk.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := m.Read(tk.ID)
	if err == nil {
		t.Fatal("Read after Delete: expected TicketNotFoundError, got nil")
	}
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("Read after Delete: expected *TicketNotFoundError, got %T", err)
	}
}

func TestMemStoreDeleteNotFound(t *testing.T) {
	m := store.NewMemStore()
	err := m.Delete("epo-ghost-xxxx")
	if err == nil {
		t.Fatal("expected TicketNotFoundError, got nil")
	}
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("expected *TicketNotFoundError, got %T: %v", err, err)
	}
}

func TestMemStoreList(t *testing.T) {
	m := store.NewMemStore()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if err := m.Create(testutil.NewTestTicket(title)); err != nil {
			t.Fatalf("Create %s: %v", title, err)
		}
	}
	all, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("List: got %d tickets, want 3", len(all))
	}
	// Verify sorted by ID.
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i].ID < all[j].ID }) {
		t.Errorf("List result is not sorted by ID: %v", func() []string {
			ids := make([]string, len(all))
			for i, t := range all {
				ids[i] = t.ID
			}
			return ids
		}())
	}
}

func TestMemStoreResolveIDExact(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("resolve exact")
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := m.ResolveID(tk.ID)
	if err != nil {
		t.Fatalf("ResolveID: %v", err)
	}
	if got != tk.ID {
		t.Errorf("ResolveID: got %q, want %q", got, tk.ID)
	}
}

func TestMemStoreResolveIDPartial(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("partial match")
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Use a 4-char suffix that appears somewhere in the ID.
	partial := tk.ID[len(tk.ID)-4:]
	got, err := m.ResolveID(partial)
	if err != nil {
		t.Fatalf("ResolveID partial %q: %v", partial, err)
	}
	if got != tk.ID {
		t.Errorf("ResolveID partial: got %q, want %q", got, tk.ID)
	}
}

func TestMemStoreResolveIDNotFound(t *testing.T) {
	m := store.NewMemStore()
	_, err := m.ResolveID("zzz-nope")
	if err == nil {
		t.Fatal("expected TicketNotFoundError, got nil")
	}
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("expected *TicketNotFoundError, got %T: %v", err, err)
	}
}

func TestMemStoreResolveIDAmbiguous(t *testing.T) {
	m := store.NewMemStore()
	tk1 := testutil.NewTestTicket("first")
	tk2 := testutil.NewTestTicket("second")
	if err := m.Create(tk1); err != nil {
		t.Fatalf("Create tk1: %v", err)
	}
	if err := m.Create(tk2); err != nil {
		t.Fatalf("Create tk2: %v", err)
	}
	// "epo-" is a substring of all epo-prefixed IDs.
	_, err := m.ResolveID("epo-")
	if err == nil {
		t.Fatal("expected AmbiguousIDError, got nil")
	}
	if _, ok := err.(*ticket.AmbiguousIDError); !ok {
		t.Errorf("expected *AmbiguousIDError, got %T: %v", err, err)
	}
}

func TestMemStoreAddDep(t *testing.T) {
	m := store.NewMemStore()
	a := testutil.NewTestTicket("a")
	b := testutil.NewTestTicket("b")
	if err := m.Create(a); err != nil {
		t.Fatalf("Create a: %v", err)
	}
	if err := m.Create(b); err != nil {
		t.Fatalf("Create b: %v", err)
	}
	if err := m.AddDep(a.ID, b.ID); err != nil {
		t.Fatalf("AddDep: %v", err)
	}
	got, err := m.Read(a.ID)
	if err != nil {
		t.Fatalf("Read a: %v", err)
	}
	if len(got.Deps) != 1 || got.Deps[0] != b.ID {
		t.Errorf("Deps = %v, want [%s]", got.Deps, b.ID)
	}
	// Idempotent.
	if err := m.AddDep(a.ID, b.ID); err != nil {
		t.Fatalf("AddDep idempotent: %v", err)
	}
	got, _ = m.Read(a.ID)
	if len(got.Deps) != 1 {
		t.Errorf("AddDep idempotent: Deps = %v, want length 1", got.Deps)
	}
}

func TestMemStoreAddDepCycleDetected(t *testing.T) {
	m := store.NewMemStore()
	a := testutil.NewTestTicket("a")
	b := testutil.NewTestTicket("b")
	c := testutil.NewTestTicket("c")
	for _, tk := range []*ticket.Ticket{a, b, c} {
		if err := m.Create(tk); err != nil {
			t.Fatalf("Create %s: %v", tk.ID, err)
		}
	}
	if err := m.AddDep(a.ID, b.ID); err != nil {
		t.Fatalf("a→b: %v", err)
	}
	if err := m.AddDep(b.ID, c.ID); err != nil {
		t.Fatalf("b→c: %v", err)
	}
	// c → a would close the a → b → c → a cycle.
	err := m.AddDep(c.ID, a.ID)
	if !errors.As(err, new(*ticket.CycleDetectedError)) {
		t.Fatalf("expected *CycleDetectedError, got %T: %v", err, err)
	}
	// Self-cycle.
	err = m.AddDep(a.ID, a.ID)
	if !errors.As(err, new(*ticket.CycleDetectedError)) {
		t.Errorf("self-cycle: expected *CycleDetectedError, got %T: %v", err, err)
	}
}

func TestMemStoreListReady(t *testing.T) {
	m := store.NewMemStore()
	open := testutil.NewTestTicket("open")
	blocker := testutil.NewTestTicket("blocker")
	blocked := testutil.NewTestTicket("blocked")

	if err := m.Create(open); err != nil {
		t.Fatalf("Create open: %v", err)
	}
	if err := m.Create(blocker); err != nil {
		t.Fatalf("Create blocker: %v", err)
	}
	blocked.Deps = []string{blocker.ID}
	blocked.Present["deps"] = true
	if err := m.Create(blocked); err != nil {
		t.Fatalf("Create blocked: %v", err)
	}

	ready, err := m.ListReady()
	if err != nil {
		t.Fatalf("ListReady: %v", err)
	}

	ids := make(map[string]bool, len(ready))
	for _, tk := range ready {
		ids[tk.ID] = true
	}
	if !ids[open.ID] {
		t.Errorf("open ticket missing from ready: %v", ids)
	}
	if !ids[blocker.ID] {
		t.Errorf("blocker (no open deps) missing from ready: %v", ids)
	}
	if ids[blocked.ID] {
		t.Errorf("ticket with open dep should not be in ready: %v", ids)
	}
}

// ─── AddNote ─────────────────────────────────────────────────────────────────

func TestMemStoreAddNote(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("note me")
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.AddNote(tk.ID, "first note"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	got, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read after AddNote: %v", err)
	}
	if len(got.Notes) != 1 {
		t.Fatalf("Notes: got %d entries, want 1", len(got.Notes))
	}
	if !strings.Contains(got.Notes[0], "first note") {
		t.Errorf("Note content: got %q, want to contain %q", got.Notes[0], "first note")
	}
}

func TestMemStoreAddNoteNotFound(t *testing.T) {
	m := store.NewMemStore()
	err := m.AddNote("epo-ghost-xxxx", "note")
	if err == nil {
		t.Fatal("expected TicketNotFoundError, got nil")
	}
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("expected *TicketNotFoundError, got %T: %v", err, err)
	}
}

// ─── RemoveDep ───────────────────────────────────────────────────────────────

func TestMemStoreRemoveDep(t *testing.T) {
	m := store.NewMemStore()
	a := testutil.NewTestTicket("a")
	b := testutil.NewTestTicket("b")
	for _, tk := range []*ticket.Ticket{a, b} {
		if err := m.Create(tk); err != nil {
			t.Fatalf("Create %s: %v", tk.ID, err)
		}
	}
	if err := m.AddDep(a.ID, b.ID); err != nil {
		t.Fatalf("AddDep: %v", err)
	}
	if err := m.RemoveDep(a.ID, b.ID); err != nil {
		t.Fatalf("RemoveDep: %v", err)
	}
	got, err := m.Read(a.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got.Deps) != 0 {
		t.Errorf("Deps after RemoveDep: got %v, want empty", got.Deps)
	}
	// Removing absent dep is a no-op, not an error.
	if err := m.RemoveDep(a.ID, b.ID); err != nil {
		t.Errorf("RemoveDep idempotent: got %v, want nil", err)
	}
}

// ─── Link / Unlink ───────────────────────────────────────────────────────────

func TestMemStoreLink(t *testing.T) {
	m := store.NewMemStore()
	a := testutil.NewTestTicket("a")
	b := testutil.NewTestTicket("b")
	for _, tk := range []*ticket.Ticket{a, b} {
		if err := m.Create(tk); err != nil {
			t.Fatalf("Create %s: %v", tk.ID, err)
		}
	}
	if err := m.Link(a.ID, b.ID); err != nil {
		t.Fatalf("Link: %v", err)
	}
	gotA, err := m.Read(a.ID)
	if err != nil {
		t.Fatalf("Read a: %v", err)
	}
	gotB, err := m.Read(b.ID)
	if err != nil {
		t.Fatalf("Read b: %v", err)
	}
	if len(gotA.Links) != 1 || gotA.Links[0] != b.ID {
		t.Errorf("a.Links = %v, want [%s]", gotA.Links, b.ID)
	}
	if len(gotB.Links) != 1 || gotB.Links[0] != a.ID {
		t.Errorf("b.Links = %v, want [%s]", gotB.Links, a.ID)
	}
	// Link is idempotent.
	if err := m.Link(a.ID, b.ID); err != nil {
		t.Errorf("Link idempotent: got %v, want nil", err)
	}
	gotA, _ = m.Read(a.ID)
	if len(gotA.Links) != 1 {
		t.Errorf("Link idempotent: a.Links = %v, want length 1", gotA.Links)
	}
}

func TestMemStoreLinkSelf(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("self")
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	err := m.Link(tk.ID, tk.ID)
	if err == nil {
		t.Fatal("expected ValidationError for self-link, got nil")
	}
	if _, ok := err.(*ticket.ValidationError); !ok {
		t.Errorf("expected *ValidationError, got %T: %v", err, err)
	}
}

func TestMemStoreUnlink(t *testing.T) {
	m := store.NewMemStore()
	a := testutil.NewTestTicket("a")
	b := testutil.NewTestTicket("b")
	for _, tk := range []*ticket.Ticket{a, b} {
		if err := m.Create(tk); err != nil {
			t.Fatalf("Create %s: %v", tk.ID, err)
		}
	}
	if err := m.Link(a.ID, b.ID); err != nil {
		t.Fatalf("Link: %v", err)
	}
	if err := m.Unlink(a.ID, b.ID); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	gotA, _ := m.Read(a.ID)
	gotB, _ := m.Read(b.ID)
	if len(gotA.Links) != 0 {
		t.Errorf("a.Links after Unlink: got %v, want empty", gotA.Links)
	}
	if len(gotB.Links) != 0 {
		t.Errorf("b.Links after Unlink: got %v, want empty", gotB.Links)
	}
}

// ─── ListAllChildren / FilterReadyChildren ────────────────────────────────────

func TestMemStoreListAllChildren(t *testing.T) {
	m := store.NewMemStore()
	parent := testutil.NewTestTicket("parent")
	child1 := testutil.NewTestTicket("child1")
	child1.Parent = parent.ID
	child1.Present["parent"] = true
	child2 := testutil.NewTestTicket("child2")
	child2.Parent = parent.ID
	child2.Present["parent"] = true
	other := testutil.NewTestTicket("other")

	for _, tk := range []*ticket.Ticket{parent, child1, child2, other} {
		if err := m.Create(tk); err != nil {
			t.Fatalf("Create %s: %v", tk.ID, err)
		}
	}

	children, err := m.ListAllChildren(parent.ID)
	if err != nil {
		t.Fatalf("ListAllChildren: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("ListAllChildren: got %d children, want 2", len(children))
	}
	ids := map[string]bool{children[0].ID: true, children[1].ID: true}
	if !ids[child1.ID] || !ids[child2.ID] {
		t.Errorf("ListAllChildren: got %v, want [%s %s]", ids, child1.ID, child2.ID)
	}
	// Unrelated parent returns empty slice.
	none, err := m.ListAllChildren("epo-no-such-parent")
	if err != nil {
		t.Fatalf("ListAllChildren nonexistent: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("ListAllChildren nonexistent: got %v, want empty", none)
	}
}

func TestMemStoreFilterReadyChildren(t *testing.T) {
	m := store.NewMemStore()
	parent := testutil.NewTestTicket("parent")
	blocker := testutil.NewTestTicket("blocker")
	open := testutil.NewTestTicket("open")
	open.Parent = parent.ID
	open.Present["parent"] = true
	blocked := testutil.NewTestTicket("blocked")
	blocked.Parent = parent.ID
	blocked.Present["parent"] = true
	blocked.Deps = []string{blocker.ID}
	blocked.Present["deps"] = true

	for _, tk := range []*ticket.Ticket{parent, blocker, open, blocked} {
		if err := m.Create(tk); err != nil {
			t.Fatalf("Create %s: %v", tk.ID, err)
		}
	}

	ready, err := m.FilterReadyChildren(parent.ID, func(string) bool { return false })
	if err != nil {
		t.Fatalf("FilterReadyChildren: %v", err)
	}
	ids := make(map[string]bool, len(ready))
	for _, tk := range ready {
		ids[tk.ID] = true
	}
	if !ids[open.ID] {
		t.Errorf("open child missing from ready: %v", ids)
	}
	if ids[blocked.ID] {
		t.Errorf("blocked child should not be in ready: %v", ids)
	}
}

// ─── ActiveClaimSet ───────────────────────────────────────────────────────────

func TestMemStoreActiveClaimSet(t *testing.T) {
	m := store.NewMemStore()
	claims, err := m.ActiveClaimSet()
	if err != nil {
		t.Fatalf("ActiveClaimSet: %v", err)
	}
	if len(claims) != 0 {
		t.Errorf("ActiveClaimSet: got %v, want empty map", claims)
	}
}

// ─── Isolation ───────────────────────────────────────────────────────────────

func TestMemStoreReadIsolatesSlices(t *testing.T) {
	m := store.NewMemStore()
	a := testutil.NewTestTicket("a")
	b := testutil.NewTestTicket("b")
	for _, tk := range []*ticket.Ticket{a, b} {
		if err := m.Create(tk); err != nil {
			t.Fatalf("Create %s: %v", tk.ID, err)
		}
	}
	if err := m.AddDep(a.ID, b.ID); err != nil {
		t.Fatalf("AddDep: %v", err)
	}

	// Mutate Deps on the returned copy — stored ticket must be unaffected.
	got, err := m.Read(a.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got.Deps[0] = "tampered"

	got2, err := m.Read(a.ID)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if got2.Deps[0] == "tampered" {
		t.Error("Read returned Deps slice sharing backing array with internal store — isolation violated")
	}
}

func TestMemStoreReadIsolatesScope(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("scoped")
	tk.Scope = ticket.TaskScope{OwnedPaths: []string{"pkg/foo"}}
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got.Scope.OwnedPaths[0] = "tampered"
	got2, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if len(got2.Scope.OwnedPaths) > 0 && got2.Scope.OwnedPaths[0] == "tampered" {
		t.Error("Scope.OwnedPaths shares backing array with internal store — isolation violated")
	}
}

func TestMemStoreReadIsolatesImplementationDetailFiles(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("detailed")
	tk.ImplementationDetail = ticket.ImplementationDetail{
		Files: []ticket.FileChange{{Path: "original.go", Change: "add"}},
	}
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got.ImplementationDetail.Files[0].Path = "tampered.go"
	got2, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if len(got2.ImplementationDetail.Files) > 0 && got2.ImplementationDetail.Files[0].Path == "tampered.go" {
		t.Error("ImplementationDetail.Files shares backing array with internal store — isolation violated")
	}
}

func TestMemStoreReadIsolatesLearningContext(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("learning")
	tk.LearningContext = []ticket.LearningRef{{ID: "ref-001", Title: "original"}}
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got.LearningContext[0].Title = "tampered"
	got2, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if len(got2.LearningContext) > 0 && got2.LearningContext[0].Title == "tampered" {
		t.Error("LearningContext shares backing array with internal store — isolation violated")
	}
}

func TestMemStoreReadIsolatesValidationChecks(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("validated")
	tk.ValidationChecks = []ticket.ValidationCheck{{Command: "go test ./...", Expected: "ok"}}
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got.ValidationChecks[0].Command = "tampered"
	got2, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if len(got2.ValidationChecks) > 0 && got2.ValidationChecks[0].Command == "tampered" {
		t.Error("ValidationChecks shares backing array with internal store — isolation violated")
	}
}

func TestMemStoreReadIsolatesExtraNestedValues(t *testing.T) {
	m := store.NewMemStore()
	tk := testutil.NewTestTicket("extra-nested")
	tk.Extra = map[string]any{
		"nested_list": []any{"a", "b", "c"},
		"nested_map":  map[string]any{"x": "original"},
		"scalar":      "flat",
	}
	if err := m.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	// Mutate nested slice value on the returned copy.
	got.Extra["nested_list"].([]any)[0] = "mutated"
	// Mutate nested map value on the returned copy.
	got.Extra["nested_map"].(map[string]any)["x"] = "mutated"

	got2, err := m.Read(tk.ID)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if list, ok := got2.Extra["nested_list"].([]any); ok && list[0] == "mutated" {
		t.Error("Extra nested_list shares backing with internal store — isolation violated")
	}
	if m2, ok := got2.Extra["nested_map"].(map[string]any); ok && m2["x"] == "mutated" {
		t.Error("Extra nested_map shares backing with internal store — isolation violated")
	}
	if got2.Extra["scalar"] != "flat" {
		t.Errorf("Extra scalar: got %v, want %q", got2.Extra["scalar"], "flat")
	}
}
