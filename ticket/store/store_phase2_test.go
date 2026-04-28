package store_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/store"
	"github.com/php-workx/epos/ticket/testutil"
)

// ─── CRUD round-trip ─────────────────────────────────────────────────────────

func TestCRUD(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicket("round trip")

	if err := s.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Title != tk.Title {
		t.Errorf("Read title = %q, want %q", got.Title, tk.Title)
	}

	got.Title = "renamed"
	if err := s.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, err := s.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read after Update: %v", err)
	}
	if after.Title != "renamed" {
		t.Errorf("after Update title = %q, want %q", after.Title, "renamed")
	}

	if err := s.Delete(tk.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Read(tk.ID); !errors.As(err, new(*ticket.TicketNotFoundError)) {
		t.Errorf("Read after Delete: expected TicketNotFoundError, got %v", err)
	}
}

// ─── Concurrency / atomicity ─────────────────────────────────────────────────

func TestConcurrentWrites(t *testing.T) {
	s := testutil.NewTestStore(t)

	const n = 10
	var wg sync.WaitGroup
	errs := make(chan error, n)
	tickets := make([]*ticket.Ticket, n)
	for i := range tickets {
		tickets[i] = testutil.NewTestTicket(fmt.Sprintf("concurrent-%d", i))
	}

	for i := range tickets {
		wg.Add(1)
		go func(tk *ticket.Ticket) {
			defer wg.Done()
			if err := s.Create(tk); err != nil {
				errs <- fmt.Errorf("Create %s: %w", tk.ID, err)
			}
		}(tickets[i])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Create: %v", err)
	}

	all, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != n {
		t.Fatalf("List: got %d tickets, want %d", len(all), n)
	}
	for _, tk := range tickets {
		got, rerr := s.Read(tk.ID)
		if rerr != nil {
			t.Errorf("Read %s: %v", tk.ID, rerr)
			continue
		}
		if got.Title != tk.Title {
			t.Errorf("Read %s: title = %q, want %q", tk.ID, got.Title, tk.Title)
		}
	}
}

func TestAtomicWrite(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.MustCreateTestTicket(t, s, "atom")

	// Drop a stale temp file in the tickets directory to simulate a prior
	// aborted write. The target file must remain untouched and Update must
	// continue to operate atomically.
	td := testutil.TicketsDir(s.Dir)
	stale, err := os.CreateTemp(td, "."+tk.ID+".md.tmp-*")
	if err != nil {
		t.Fatalf("create stale temp: %v", err)
	}
	if _, err := stale.WriteString("garbage partial content"); err != nil {
		t.Fatalf("write stale temp: %v", err)
	}
	_ = stale.Close()

	target := filepath.Join(td, tk.ID+".md")
	infoBefore, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if infoBefore.Size() == 0 {
		t.Fatalf("target zero-byte before Update")
	}

	tk.Title = "atomic-updated"
	if err := s.Update(tk); err != nil {
		t.Fatalf("Update: %v", err)
	}

	infoAfter, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target after Update: %v", err)
	}
	if infoAfter.Size() == 0 {
		t.Fatalf("target zero-byte after Update")
	}
	got, err := s.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Title != "atomic-updated" {
		t.Errorf("title = %q, want %q", got.Title, "atomic-updated")
	}
}

// ─── AddNote ─────────────────────────────────────────────────────────────────

func TestAddNote(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.MustCreateTestTicket(t, s, "note me")

	if err := s.AddNote(tk.ID, "first note"); err != nil {
		t.Fatalf("AddNote first: %v", err)
	}
	if err := s.AddNote(tk.ID, "second note"); err != nil {
		t.Fatalf("AddNote second: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(testutil.TicketsDir(s.Dir), tk.ID+".md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(body)
	if !contains(content, "## Notes") {
		t.Errorf("missing ## Notes section: %s", content)
	}
	if !contains(content, "first note") || !contains(content, "second note") {
		t.Errorf("notes not accumulated: %s", content)
	}
}

// ─── Deps / cycle detection ──────────────────────────────────────────────────

func TestDeps(t *testing.T) {
	s := testutil.NewTestStore(t)
	a := testutil.MustCreateTestTicket(t, s, "a")
	b := testutil.MustCreateTestTicket(t, s, "b")

	if err := s.AddDep(a.ID, b.ID); err != nil {
		t.Fatalf("AddDep: %v", err)
	}
	got, _ := s.Read(a.ID)
	if len(got.Deps) != 1 || got.Deps[0] != b.ID {
		t.Errorf("AddDep: Deps = %v, want [%s]", got.Deps, b.ID)
	}

	// Idempotent.
	if err := s.AddDep(a.ID, b.ID); err != nil {
		t.Fatalf("AddDep idempotent: %v", err)
	}
	got, _ = s.Read(a.ID)
	if len(got.Deps) != 1 {
		t.Errorf("AddDep idempotent: Deps = %v, want length 1", got.Deps)
	}

	if err := s.RemoveDep(a.ID, b.ID); err != nil {
		t.Fatalf("RemoveDep: %v", err)
	}
	got, _ = s.Read(a.ID)
	if len(got.Deps) != 0 {
		t.Errorf("RemoveDep: Deps = %v, want []", got.Deps)
	}

	// Removing absent dep is a no-op.
	if err := s.RemoveDep(a.ID, b.ID); err != nil {
		t.Errorf("RemoveDep absent: expected nil, got %v", err)
	}
}

func TestAddDepCycleDetection(t *testing.T) {
	s := testutil.NewTestStore(t)
	a := testutil.MustCreateTestTicket(t, s, "a")
	b := testutil.MustCreateTestTicket(t, s, "b")
	c := testutil.MustCreateTestTicket(t, s, "c")

	if err := s.AddDep(a.ID, b.ID); err != nil {
		t.Fatalf("a→b: %v", err)
	}
	if err := s.AddDep(b.ID, c.ID); err != nil {
		t.Fatalf("b→c: %v", err)
	}
	// c → a would close the a → b → c → a cycle.
	err := s.AddDep(c.ID, a.ID)
	if !errors.As(err, new(*ticket.CycleDetectedError)) {
		t.Fatalf("expected *CycleDetectedError, got %T: %v", err, err)
	}

	// Self-cycle.
	err = s.AddDep(a.ID, a.ID)
	if !errors.As(err, new(*ticket.CycleDetectedError)) {
		t.Errorf("self-cycle: expected *CycleDetectedError, got %T: %v", err, err)
	}
}

// ─── Links ───────────────────────────────────────────────────────────────────

func TestLinks(t *testing.T) {
	t.Run("symmetric", testLinkSymmetric)
	t.Run("unlink", testUnlinkSymmetric)
}

func TestLinkSymmetric(t *testing.T) { testLinkSymmetric(t) }

func testLinkSymmetric(t *testing.T) {
	s := testutil.NewTestStore(t)
	a := testutil.MustCreateTestTicket(t, s, "link-a")
	b := testutil.MustCreateTestTicket(t, s, "link-b")

	if err := s.Link(a.ID, b.ID); err != nil {
		t.Fatalf("Link: %v", err)
	}
	ga, _ := s.Read(a.ID)
	gb, _ := s.Read(b.ID)
	if !sliceContains(ga.Links, b.ID) {
		t.Errorf("A.Links = %v, want contains %s", ga.Links, b.ID)
	}
	if !sliceContains(gb.Links, a.ID) {
		t.Errorf("B.Links = %v, want contains %s", gb.Links, a.ID)
	}

	// Idempotent — re-linking does not duplicate.
	if err := s.Link(a.ID, b.ID); err != nil {
		t.Fatalf("Link idempotent: %v", err)
	}
	ga, _ = s.Read(a.ID)
	if countOccurrences(ga.Links, b.ID) != 1 {
		t.Errorf("Link not idempotent: %v", ga.Links)
	}
}

func TestUnlinkSymmetric(t *testing.T) { testUnlinkSymmetric(t) }

func testUnlinkSymmetric(t *testing.T) {
	s := testutil.NewTestStore(t)
	a := testutil.MustCreateTestTicket(t, s, "unlink-a")
	b := testutil.MustCreateTestTicket(t, s, "unlink-b")

	if err := s.Link(a.ID, b.ID); err != nil {
		t.Fatalf("Link: %v", err)
	}
	if err := s.Unlink(a.ID, b.ID); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	ga, _ := s.Read(a.ID)
	gb, _ := s.Read(b.ID)
	if sliceContains(ga.Links, b.ID) {
		t.Errorf("A.Links still references B: %v", ga.Links)
	}
	if sliceContains(gb.Links, a.ID) {
		t.Errorf("B.Links still references A: %v", gb.Links)
	}
}

// ─── Children helpers ────────────────────────────────────────────────────────

func TestListAllChildren(t *testing.T) {
	s := testutil.NewTestStore(t)
	parent := testutil.MustCreateTestTicket(t, s, "parent")
	child1 := testutil.NewTestTicket("child1")
	child1.Parent = parent.ID
	child1.Present["parent"] = true
	testutil.MustCreateTicket(t, s, child1)
	child2 := testutil.NewTestTicket("child2")
	child2.Parent = parent.ID
	child2.Present["parent"] = true
	testutil.MustCreateTicket(t, s, child2)
	other := testutil.MustCreateTestTicket(t, s, "unrelated")

	kids, err := s.ListAllChildren(parent.ID)
	if err != nil {
		t.Fatalf("ListAllChildren: %v", err)
	}
	if len(kids) != 2 {
		t.Fatalf("got %d children, want 2", len(kids))
	}
	for _, k := range kids {
		if k.ID == other.ID {
			t.Errorf("non-child %s leaked into result", other.ID)
		}
	}
}

func TestFilterReadyChildren(t *testing.T) {
	s := testutil.NewTestStore(t)
	parent := testutil.MustCreateTestTicket(t, s, "parent2")

	open := testutil.NewTestTicket("open-child")
	open.Parent = parent.ID
	open.Present["parent"] = true
	testutil.MustCreateTicket(t, s, open)

	claimed := testutil.NewTestTicket("claimed-child")
	claimed.Parent = parent.ID
	claimed.Present["parent"] = true
	testutil.MustCreateTicket(t, s, claimed)

	blocked := testutil.NewTestTicket("blocked-child")
	blocked.Parent = parent.ID
	blocked.Present["parent"] = true
	blocked.Deps = []string{open.ID}
	blocked.Present["deps"] = true
	testutil.MustCreateTicket(t, s, blocked)

	isClaimed := func(id string) bool { return id == claimed.ID }
	ready, err := s.FilterReadyChildren(parent.ID, isClaimed)
	if err != nil {
		t.Fatalf("FilterReadyChildren: %v", err)
	}

	ids := make([]string, 0, len(ready))
	for _, r := range ready {
		ids = append(ids, r.ID)
	}
	if !sliceContains(ids, open.ID) {
		t.Errorf("open child %s missing: %v", open.ID, ids)
	}
	if sliceContains(ids, claimed.ID) {
		t.Errorf("claimed child %s leaked: %v", claimed.ID, ids)
	}
	if sliceContains(ids, blocked.ID) {
		t.Errorf("blocked child %s leaked (open dep should block ready): %v", blocked.ID, ids)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func countOccurrences(haystack []string, needle string) int {
	n := 0
	for _, s := range haystack {
		if s == needle {
			n++
		}
	}
	return n
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// _ ensures the store package import is used even if all references above are
// removed by future edits.
var _ = store.TicketsDir
