package graph_test

import (
	"testing"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
)

// mk builds a minimal Ticket for use in graph tests.
func mk(id string, status ticket.Status, priority int, deps []string, parent string) ticket.Ticket {
	return ticket.Ticket{
		ID:       id,
		Status:   status,
		Priority: priority,
		Deps:     deps,
		Parent:   parent,
	}
}

// ---- ReadyFilter -----------------------------------------------------------

// TestReadyNoDeps: a ticket with empty Deps and status pending appears in ReadyFilter.
func TestReadyNoDeps(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("t1", ticket.StatusPending, 0, nil, ""),
	}
	got := graph.ReadyFilter(tickets)
	if len(got) != 1 {
		t.Fatalf("ReadyFilter: expected 1 ticket, got %d", len(got))
	}
	if got[0].ID != "t1" {
		t.Errorf("ReadyFilter: expected t1, got %s", got[0].ID)
	}
}

// TestReadyRepairPending: a ticket with status repair_pending and no open deps is ready.
func TestReadyRepairPending(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("r1", ticket.StatusRepairPending, 0, nil, ""),
	}
	got := graph.ReadyFilter(tickets)
	if len(got) != 1 {
		t.Fatalf("ReadyFilter: expected 1 repair_pending ticket, got %d", len(got))
	}
}

// TestReadyAllDepsClosed: a ticket with Deps ["a","b"] where both are closed appears.
func TestReadyAllDepsClosed(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("main", ticket.StatusPending, 0, []string{"a", "b"}, ""),
		mk("a", ticket.StatusClosed, 0, nil, ""),
		mk("b", ticket.StatusDone, 0, nil, ""),
	}
	got := graph.ReadyFilter(tickets)
	if len(got) != 1 {
		t.Fatalf("ReadyFilter: expected 1 ticket, got %d (%v)", len(got), ids(got))
	}
	if got[0].ID != "main" {
		t.Errorf("ReadyFilter: expected main, got %s", got[0].ID)
	}
}

// TestReadyExcludesOpenDep: a ticket whose dep has status open (not closed) is NOT ready.
// The dep itself (open, no deps) IS ready; only the dependent child is excluded.
func TestReadyExcludesOpenDep(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("child", ticket.StatusPending, 0, []string{"dep"}, ""),
		mk("dep", ticket.StatusOpen, 0, nil, ""),
	}
	got := graph.ReadyFilter(tickets)
	for _, r := range got {
		if r.ID == "child" {
			t.Errorf("ReadyFilter: child with open dep should not appear in ready list")
		}
	}
}

// TestReadyExcludesAbsentDep: a ticket whose dep is not in the set is NOT ready.
func TestReadyExcludesAbsentDep(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("child", ticket.StatusPending, 0, []string{"ghost"}, ""),
	}
	got := graph.ReadyFilter(tickets)
	if len(got) != 0 {
		t.Fatalf("ReadyFilter: expected 0 tickets for unknown dep, got %d", len(got))
	}
}

// TestReadyExcludesNonPending: tickets in active/terminal statuses are not returned.
// open, pending, and repair_pending are ready; everything else is not.
func TestReadyExcludesNonPending(t *testing.T) {
	t.Parallel()
	nonReady := []ticket.Status{
		ticket.StatusInProgress,
		ticket.StatusClosed,
		ticket.StatusDone,
		ticket.StatusFailed,
		ticket.StatusClaimed,
		ticket.StatusHeld,
	}
	for _, s := range nonReady {
		s := s
		t.Run(string(s), func(t *testing.T) {
			t.Parallel()
			got := graph.ReadyFilter([]ticket.Ticket{mk("x", s, 0, nil, "")})
			if len(got) != 0 {
				t.Errorf("ReadyFilter: status %q should not appear, got %v", s, ids(got))
			}
		})
	}
}

// TestReadySortOrder: ReadyFilter sorts by priority descending then ID ascending.
func TestReadySortOrder(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("b", ticket.StatusPending, 10, nil, ""),
		mk("a", ticket.StatusPending, 10, nil, ""), // same priority as b, but ID a < b
		mk("c", ticket.StatusPending, 5, nil, ""),
		mk("d", ticket.StatusPending, 20, nil, ""),
	}
	got := graph.ReadyFilter(tickets)
	want := []string{"d", "a", "b", "c"}
	assertOrder(t, "ReadyFilter", got, want)
}

// TestReadyEmpty: empty input returns nil/empty.
func TestReadyEmpty(t *testing.T) {
	t.Parallel()
	got := graph.ReadyFilter(nil)
	if len(got) != 0 {
		t.Fatalf("ReadyFilter(nil): expected empty, got %d", len(got))
	}
}

// ---- BlockedFilter ---------------------------------------------------------

// TestBlockedOpenDep: ticket with Deps ["a"] where a is open appears in BlockedFilter.
func TestBlockedOpenDep(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("child", ticket.StatusPending, 0, []string{"a"}, ""),
		mk("a", ticket.StatusOpen, 0, nil, ""),
	}
	got := graph.BlockedFilter(tickets)
	if len(got) != 1 {
		t.Fatalf("BlockedFilter: expected 1 ticket, got %d (%v)", len(got), ids(got))
	}
	if got[0].ID != "child" {
		t.Errorf("BlockedFilter: expected child, got %s", got[0].ID)
	}
}

// TestBlockedAbsentDep: ticket whose dep is not in the set is blocked (conservative).
func TestBlockedAbsentDep(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("child", ticket.StatusPending, 0, []string{"ghost"}, ""),
	}
	got := graph.BlockedFilter(tickets)
	if len(got) != 1 {
		t.Fatalf("BlockedFilter: expected 1 ticket for unknown dep, got %d", len(got))
	}
}

// TestBlockedExcludesNoDeps: pending ticket with no deps is NOT blocked.
func TestBlockedExcludesNoDeps(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("t1", ticket.StatusPending, 0, nil, ""),
	}
	got := graph.BlockedFilter(tickets)
	if len(got) != 0 {
		t.Fatalf("BlockedFilter: expected 0 tickets, got %d", len(got))
	}
}

// TestBlockedExcludesAllDepsClosed: ticket with all-closed deps is NOT blocked.
func TestBlockedExcludesAllDepsClosed(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("child", ticket.StatusPending, 0, []string{"dep"}, ""),
		mk("dep", ticket.StatusClosed, 0, nil, ""),
	}
	got := graph.BlockedFilter(tickets)
	if len(got) != 0 {
		t.Fatalf("BlockedFilter: expected 0 tickets (dep is closed), got %d", len(got))
	}
}

// TestBlockedSortOrder: BlockedFilter sorts by priority descending then ID ascending.
func TestBlockedSortOrder(t *testing.T) {
	t.Parallel()
	openDep := mk("dep", ticket.StatusOpen, 0, nil, "")
	tickets := []ticket.Ticket{
		mk("b", ticket.StatusPending, 10, []string{"dep"}, ""),
		mk("a", ticket.StatusPending, 10, []string{"dep"}, ""),
		mk("c", ticket.StatusPending, 5, []string{"dep"}, ""),
		mk("d", ticket.StatusPending, 20, []string{"dep"}, ""),
		openDep,
	}
	got := graph.BlockedFilter(tickets)
	want := []string{"d", "a", "b", "c"}
	assertOrder(t, "BlockedFilter", got, want)
}

// ---- DetectCycles ----------------------------------------------------------

// TestCycleDetectionSimple: cycle A→B→A detected, returns [][]string{{"A","B"}}.
func TestCycleDetectionSimple(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("A", ticket.StatusPending, 0, []string{"B"}, ""),
		mk("B", ticket.StatusPending, 0, []string{"A"}, ""),
	}
	cycles := graph.DetectCycles(tickets)
	if len(cycles) != 1 {
		t.Fatalf("DetectCycles: expected 1 cycle, got %d: %v", len(cycles), cycles)
	}
	got := cycles[0]
	if len(got) != 2 {
		t.Fatalf("DetectCycles: expected cycle of length 2, got %d: %v", len(got), got)
	}
	// Cycle must contain both A and B.
	idSet := map[string]bool{got[0]: true, got[1]: true}
	if !idSet["A"] || !idSet["B"] {
		t.Errorf("DetectCycles: expected cycle [A B], got %v", got)
	}
	// Per acceptance criteria, expected representation is {A, B}.
	if got[0] != "A" || got[1] != "B" {
		t.Errorf("DetectCycles: expected [A B] (A first, lexicographic), got %v", got)
	}
}

// TestNoCycles: acyclic graph returns no cycles.
func TestNoCycles(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("A", ticket.StatusPending, 0, []string{"B"}, ""),
		mk("B", ticket.StatusPending, 0, []string{"C"}, ""),
		mk("C", ticket.StatusClosed, 0, nil, ""),
	}
	cycles := graph.DetectCycles(tickets)
	if len(cycles) != 0 {
		t.Fatalf("DetectCycles: expected 0 cycles in acyclic graph, got %d: %v", len(cycles), cycles)
	}
}

// TestSelfCycle: a ticket that depends on itself.
func TestSelfCycle(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("A", ticket.StatusPending, 0, []string{"A"}, ""),
	}
	cycles := graph.DetectCycles(tickets)
	if len(cycles) != 1 {
		t.Fatalf("DetectCycles: expected 1 self-cycle, got %d: %v", len(cycles), cycles)
	}
	if len(cycles[0]) != 1 || cycles[0][0] != "A" {
		t.Errorf("DetectCycles: expected self-cycle [A], got %v", cycles[0])
	}
}

// TestLongerCycle: A→B→C→A produces a 3-node cycle.
func TestLongerCycle(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("A", ticket.StatusPending, 0, []string{"B"}, ""),
		mk("B", ticket.StatusPending, 0, []string{"C"}, ""),
		mk("C", ticket.StatusPending, 0, []string{"A"}, ""),
	}
	cycles := graph.DetectCycles(tickets)
	if len(cycles) != 1 {
		t.Fatalf("DetectCycles: expected 1 cycle, got %d: %v", len(cycles), cycles)
	}
	if len(cycles[0]) != 3 {
		t.Errorf("DetectCycles: expected 3-node cycle, got %v", cycles[0])
	}
}

// TestCycleIgnoresAbsentDeps: deps not in the ticket set do not trigger a cycle.
func TestCycleIgnoresAbsentDeps(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("A", ticket.StatusPending, 0, []string{"ghost"}, ""),
	}
	cycles := graph.DetectCycles(tickets)
	if len(cycles) != 0 {
		t.Fatalf("DetectCycles: expected 0 cycles (dep absent), got %d: %v", len(cycles), cycles)
	}
}

// ---- FilterChildren --------------------------------------------------------

// TestFilterChildren: FilterChildren(all, "parent-1") returns only tickets where Parent == "parent-1".
func TestFilterChildren(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("c1", ticket.StatusPending, 0, nil, "parent-1"),
		mk("c2", ticket.StatusPending, 0, nil, "parent-1"),
		mk("c3", ticket.StatusPending, 0, nil, "parent-2"),
		mk("c4", ticket.StatusClosed, 0, nil, ""),
	}
	got := graph.FilterChildren(tickets, "parent-1")
	if len(got) != 2 {
		t.Fatalf("FilterChildren: expected 2, got %d (%v)", len(got), ids(got))
	}
	for _, ch := range got {
		if ch.Parent != "parent-1" {
			t.Errorf("FilterChildren: unexpected parent %q for ticket %q", ch.Parent, ch.ID)
		}
	}
}

// TestFilterChildrenEmpty: no children of an unknown parent.
func TestFilterChildrenEmpty(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("c1", ticket.StatusPending, 0, nil, "parent-1"),
	}
	got := graph.FilterChildren(tickets, "nobody")
	if len(got) != 0 {
		t.Fatalf("FilterChildren: expected 0, got %d", len(got))
	}
}

// TestFilterChildrenPreservesOrder: result preserves input order (no sort imposed).
func TestFilterChildrenPreservesOrder(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("z", ticket.StatusPending, 5, nil, "p"),
		mk("a", ticket.StatusPending, 1, nil, "p"),
		mk("m", ticket.StatusPending, 3, nil, "p"),
	}
	got := graph.FilterChildren(tickets, "p")
	want := []string{"z", "a", "m"} // input order preserved
	assertOrder(t, "FilterChildren", got, want)
}

// ---- FilterReadyChildren ---------------------------------------------------

// TestFilterReadyChildren: filters by parent, status pending/repair_pending, all deps closed, not claimed.
func TestFilterReadyChildren(t *testing.T) {
	t.Parallel()

	dep := mk("dep1", ticket.StatusClosed, 0, nil, "")
	openDep := mk("dep2", ticket.StatusOpen, 0, nil, "")

	c1 := mk("c1", ticket.StatusPending, 5, []string{"dep1"}, "parent-1") // ✓ ready
	c2 := mk("c2", ticket.StatusPending, 3, []string{"dep2"}, "parent-1") // ✗ dep open
	c3 := mk("c3", ticket.StatusPending, 7, nil, "parent-1")              // ✓ ready
	c4 := mk("c4", ticket.StatusPending, 1, nil, "parent-1")              // ✗ claimed
	c5 := mk("c5", ticket.StatusClaimed, 0, nil, "parent-1")              // ✗ wrong status
	c6 := mk("c6", ticket.StatusPending, 0, nil, "parent-2")              // ✗ wrong parent
	c7 := mk("c7", ticket.StatusRepairPending, 2, nil, "parent-1")        // ✓ repair_pending

	all := []ticket.Ticket{dep, openDep, c1, c2, c3, c4, c5, c6, c7}

	claimed := map[string]bool{"c4": true}
	isClaimed := func(id string) bool { return claimed[id] }

	got := graph.FilterReadyChildren(all, "parent-1", isClaimed)

	// Expected: c3 (p=7), c1 (p=5), c7 (p=2) — sorted by priority desc then ID asc.
	want := []string{"c3", "c1", "c7"}
	if len(got) != len(want) {
		t.Fatalf("FilterReadyChildren: expected %d tickets (%v), got %d (%v)",
			len(want), want, len(got), ids(got))
	}
	assertOrder(t, "FilterReadyChildren", got, want)
}

// TestFilterReadyChildrenNilIsClaimed: nil isClaimed means no claim filter.
func TestFilterReadyChildrenNilIsClaimed(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("c1", ticket.StatusPending, 0, nil, "p"),
		mk("c2", ticket.StatusPending, 0, nil, "p"),
	}
	got := graph.FilterReadyChildren(tickets, "p", nil)
	if len(got) != 2 {
		t.Fatalf("FilterReadyChildren(nil isClaimed): expected 2, got %d", len(got))
	}
}

// TestFilterReadyChildrenSortOrder: verifies priority desc + ID asc within same priority.
func TestFilterReadyChildrenSortOrder(t *testing.T) {
	t.Parallel()
	tickets := []ticket.Ticket{
		mk("b", ticket.StatusPending, 10, nil, "p"),
		mk("a", ticket.StatusPending, 10, nil, "p"),
		mk("d", ticket.StatusPending, 20, nil, "p"),
		mk("c", ticket.StatusPending, 5, nil, "p"),
	}
	got := graph.FilterReadyChildren(tickets, "p", nil)
	want := []string{"d", "a", "b", "c"}
	assertOrder(t, "FilterReadyChildren sort", got, want)
}

// ---- helpers ---------------------------------------------------------------

// ids extracts ticket IDs for readable failure messages.
func ids(tickets []ticket.Ticket) []string {
	out := make([]string, len(tickets))
	for i, t := range tickets {
		out[i] = t.ID
	}
	return out
}

// assertOrder checks that the IDs of got match want in order.
func assertOrder(t *testing.T, label string, got []ticket.Ticket, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: expected %v (len=%d), got %v (len=%d)",
			label, want, len(want), ids(got), len(got))
	}
	for i, w := range want {
		if got[i].ID != w {
			t.Errorf("%s: position %d: expected %q, got %q (full: %v)",
				label, i, w, got[i].ID, ids(got))
		}
	}
}
