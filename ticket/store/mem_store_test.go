package store_test

import (
	"errors"
	"sort"
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
