package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/runtime"
	"github.com/php-workx/epos/ticket/testutil"
)

func TestStoreDataSourceSnapshotGroupsTicketsAndClaims(t *testing.T) {
	s := testutil.NewTestStore(t)

	done := testutil.NewTestTicketWithStatus("finished dependency", ticket.StatusClosed)
	done.ID = "epo-done"
	testutil.MustCreateTicket(t, s, done)

	ready := testutil.NewTestTicketWithStatus("ready task", ticket.StatusOpen)
	ready.ID = "epo-ready"
	ready.Priority = 4
	testutil.MustCreateTicket(t, s, ready)

	blocked := testutil.NewTestTicketWithStatus("blocked task", ticket.StatusOpen)
	blocked.ID = "epo-blocked"
	blocked.Deps = []string{"epo-missing"}
	blocked.Present["deps"] = true
	testutil.MustCreateTicket(t, s, blocked)

	claimed := testutil.NewTestTicketWithStatus("claimed task", ticket.StatusOpen)
	claimed.ID = "epo-claimed"
	testutil.MustCreateTicket(t, s, claimed)
	if err := runtime.Claim(s.Dir, claimed.ID, "agent-1", "test", time.Minute); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	ds := NewStoreDataSource(s)
	snap, err := ds.Snapshot("")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	assertGroupIDs(t, snap, GroupReady, []string{"epo-ready"})
	assertGroupIDs(t, snap, GroupBlocked, []string{"epo-blocked"})
	assertGroupIDs(t, snap, GroupClaimed, []string{"epo-claimed"})
	assertGroupIDs(t, snap, GroupClosed, []string{"epo-done"})

	if snap.Counts[GroupAll] != 4 {
		t.Fatalf("all count = %d, want 4", snap.Counts[GroupAll])
	}
	row := snap.RowByID("epo-claimed")
	if row == nil {
		t.Fatal("claimed row missing")
	}
	if row.ClaimOwner != "agent-1" {
		t.Fatalf("ClaimOwner = %q, want agent-1", row.ClaimOwner)
	}
	if !strings.Contains(row.RuntimeSummary, "agent-1") {
		t.Fatalf("RuntimeSummary = %q, want owner", row.RuntimeSummary)
	}
}

func TestStoreDataSourceSnapshotIgnoresInactiveClaims(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state *ticket.RuntimeState
	}{
		{
			name: "expired lease",
			state: &ticket.RuntimeState{
				TicketID: "epo-inactive",
				Claim: &ticket.Claim{
					ClaimedBy:    "agent-1",
					ClaimBackend: "run-1",
					ClaimedAt:    time.Now().Add(-2 * time.Hour),
				},
				Lease: &ticket.Lease{
					LeaseID:   "expired",
					ExpiresAt: time.Now().Add(-time.Hour),
				},
			},
		},
		{
			name: "absent lease",
			state: &ticket.RuntimeState{
				TicketID: "epo-inactive",
				Claim: &ticket.Claim{
					ClaimedBy:    "agent-1",
					ClaimBackend: "run-1",
					ClaimedAt:    time.Now().Add(-2 * time.Hour),
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testutil.NewTestStore(t)
			tk := testutil.NewTestTicketWithStatus("inactive claim", ticket.StatusOpen)
			tk.ID = "epo-inactive"
			testutil.MustCreateTicket(t, s, tk)
			if err := runtime.WriteRuntimeState(s.Dir, tc.state); err != nil {
				t.Fatalf("WriteRuntimeState: %v", err)
			}

			ds := NewStoreDataSource(s)
			snap, err := ds.Snapshot("")
			if err != nil {
				t.Fatalf("Snapshot: %v", err)
			}

			assertGroupIDs(t, snap, GroupReady, []string{"epo-inactive"})
			assertGroupIDs(t, snap, GroupClaimed, nil)
			row := snap.RowByID("epo-inactive")
			if row == nil {
				t.Fatal("inactive claim row missing")
			}
			if row.ReadinessGroup != GroupReady {
				t.Fatalf("ReadinessGroup = %q, want %q", row.ReadinessGroup, GroupReady)
			}
			if row.ClaimOwner != "" || row.RuntimeSummary != "" {
				t.Fatalf("inactive claim rendered as active: owner=%q summary=%q", row.ClaimOwner, row.RuntimeSummary)
			}
		})
	}
}

func TestStoreDataSourceClaimReclaimsExpiredClaim(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicketWithStatus("expired claim", ticket.StatusOpen)
	tk.ID = "epo-expired"
	testutil.MustCreateTicket(t, s, tk)
	if err := runtime.WriteRuntimeState(s.Dir, &ticket.RuntimeState{
		TicketID: tk.ID,
		Status:   ticket.StatusPending,
		Claim: &ticket.Claim{
			ClaimedBy:    "stale-agent",
			ClaimBackend: "old-run",
			ClaimedAt:    time.Now().Add(-2 * time.Hour),
		},
		Lease: &ticket.Lease{
			LeaseID:   "expired",
			ExpiresAt: time.Now().Add(-time.Hour),
		},
	}); err != nil {
		t.Fatalf("WriteRuntimeState: %v", err)
	}

	ds := NewStoreDataSource(s)
	if err := ds.Claim(tk.ID, "agent-2"); err != nil {
		t.Fatalf("Claim expired sidecar: %v", err)
	}

	state, err := runtime.ReadRuntimeState(s.Dir, tk.ID)
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	if state.Claim == nil || state.Claim.ClaimedBy != "agent-2" {
		t.Fatalf("Claim = %#v, want agent-2", state.Claim)
	}
}

func TestStoreDataSourceSnapshotFiltersByParent(t *testing.T) {
	s := testutil.NewTestStore(t)

	parent := testutil.NewTestTicketWithStatus("parent", ticket.StatusOpen)
	parent.ID = "epo-parent"
	testutil.MustCreateTicket(t, s, parent)

	child := testutil.NewTestTicketWithStatus("child", ticket.StatusOpen)
	child.ID = "epo-child"
	child.Parent = parent.ID
	child.Present["parent"] = true
	testutil.MustCreateTicket(t, s, child)

	other := testutil.NewTestTicketWithStatus("other", ticket.StatusOpen)
	other.ID = "epo-other"
	testutil.MustCreateTicket(t, s, other)

	ds := NewStoreDataSource(s)
	snap, err := ds.Snapshot(parent.ID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	assertGroupIDs(t, snap, GroupAll, []string{"epo-child"})
}

func TestStoreDataSourceSnapshotIncludesChildrenInRows(t *testing.T) {
	s := testutil.NewTestStore(t)

	parent := testutil.NewTestTicketWithStatus("parent", ticket.StatusOpen)
	parent.ID = "epo-parent"
	testutil.MustCreateTicket(t, s, parent)

	child := testutil.NewTestTicketWithStatus("child task", ticket.StatusOpen)
	child.ID = "epo-child"
	child.Parent = parent.ID
	child.Priority = 2
	child.Present["parent"] = true
	testutil.MustCreateTicket(t, s, child)

	ds := NewStoreDataSource(s)
	snap, err := ds.Snapshot("")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	row := snap.RowByID(parent.ID)
	if row == nil {
		t.Fatal("parent row missing")
	}
	if row.ChildCount != 1 {
		t.Fatalf("ChildCount = %d, want 1", row.ChildCount)
	}
	if len(row.ChildSummaries) != 1 || !strings.Contains(row.ChildSummaries[0], "epo-child") {
		t.Fatalf("ChildSummaries = %#v, want child summary", row.ChildSummaries)
	}
}

func TestFilterRowsMatchesIDTitleStatusTypeTagsAndAssignee(t *testing.T) {
	rows := []TicketRow{
		{
			ID:       "epo-one",
			Title:    "Render detail pane",
			Status:   ticket.StatusOpen,
			Type:     "feature",
			Tags:     []string{"ui", "operator"},
			Assignee: "riley",
		},
		{
			ID:       "epo-two",
			Title:    "Repair importer",
			Status:   ticket.StatusClosed,
			Type:     "bug",
			Tags:     []string{"backend"},
			Assignee: "casey",
		},
	}

	for _, tc := range []struct {
		name string
		q    string
		want string
	}{
		{name: "id", q: "one", want: "epo-one"},
		{name: "title", q: "detail", want: "epo-one"},
		{name: "status", q: "closed", want: "epo-two"},
		{name: "type", q: "bug", want: "epo-two"},
		{name: "tag", q: "operator", want: "epo-one"},
		{name: "assignee", q: "casey", want: "epo-two"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterRows(rows, tc.q)
			if len(got) != 1 || got[0].ID != tc.want {
				t.Fatalf("FilterRows(%q) = %#v, want only %s", tc.q, got, tc.want)
			}
		})
	}
}

func TestStoreDataSourceMutationsUseStoreAndRuntime(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicketWithStatus("mutable", ticket.StatusOpen)
	tk.ID = "epo-mutable"
	testutil.MustCreateTicket(t, s, tk)

	ds := NewStoreDataSource(s)
	if err := ds.AddNote(tk.ID, "operator note"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	updated, err := s.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(updated.Notes) != 1 || !strings.Contains(updated.Notes[0], "operator note") {
		t.Fatalf("Notes = %#v, want note", updated.Notes)
	}

	if err := ds.Claim(tk.ID, "agent-1"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	state, err := runtime.ReadRuntimeState(s.Dir, tk.ID)
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	if state.Claim == nil || state.Claim.ClaimedBy != "agent-1" {
		t.Fatalf("Claim = %#v, want agent-1", state.Claim)
	}

	if err := ds.Release(tk.ID, "agent-1"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	state, err = runtime.ReadRuntimeState(s.Dir, tk.ID)
	if err != nil {
		t.Fatalf("ReadRuntimeState after release: %v", err)
	}
	if state.Claim != nil || state.Status != ticket.StatusPending {
		t.Fatalf("state after release = %#v, want unclaimed pending", state)
	}

	if err := ds.Close(tk.ID, "done"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	closed, err := s.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read closed: %v", err)
	}
	if closed.Status != ticket.StatusClosed || closed.StatusReason != "done" {
		t.Fatalf("closed ticket = %#v", closed)
	}
	state, err = runtime.ReadRuntimeState(s.Dir, tk.ID)
	if err != nil {
		t.Fatalf("ReadRuntimeState after close: %v", err)
	}
	if state.Claim != nil || state.Lease != nil || state.Heartbeat != nil || state.Status != ticket.StatusClosed {
		t.Fatalf("state after close = %#v, want unclaimed closed", state)
	}

	if err := ds.Reopen(tk.ID, "retry"); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	reopened, err := s.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read reopened: %v", err)
	}
	if reopened.Status != ticket.StatusOpen || reopened.StatusReason != "retry" {
		t.Fatalf("reopened ticket = %#v", reopened)
	}
	state, err = runtime.ReadRuntimeState(s.Dir, tk.ID)
	if err != nil {
		t.Fatalf("ReadRuntimeState after reopen: %v", err)
	}
	if state.Claim != nil || state.Lease != nil || state.Heartbeat != nil || state.Status != ticket.StatusPending {
		t.Fatalf("state after reopen = %#v, want unclaimed pending", state)
	}
}

func TestStoreDataSourceCloseClearsClaimRuntime(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicketWithStatus("claimed close", ticket.StatusOpen)
	tk.ID = "epo-claimed-close"
	testutil.MustCreateTicket(t, s, tk)
	if err := runtime.Claim(s.Dir, tk.ID, "agent-1", "test", time.Minute); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	ds := NewStoreDataSource(s)
	if err := ds.Close(tk.ID, "done"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	state, err := runtime.ReadRuntimeState(s.Dir, tk.ID)
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	if state.Claim != nil || state.Lease != nil || state.Heartbeat != nil {
		t.Fatalf("runtime claim fields after close = claim:%#v lease:%#v heartbeat:%#v", state.Claim, state.Lease, state.Heartbeat)
	}
	if state.Status != ticket.StatusClosed {
		t.Fatalf("runtime status = %q, want %q", state.Status, ticket.StatusClosed)
	}

	snap, err := ds.Snapshot("")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	assertGroupIDs(t, snap, GroupClaimed, nil)
	assertGroupIDs(t, snap, GroupClosed, []string{"epo-claimed-close"})
	row := snap.RowByID(tk.ID)
	if row == nil {
		t.Fatal("closed row missing")
	}
	if row.ReadinessGroup != GroupClosed {
		t.Fatalf("ReadinessGroup = %q, want %q", row.ReadinessGroup, GroupClosed)
	}
}

func assertGroupIDs(t *testing.T, snap Snapshot, group Group, want []string) {
	t.Helper()
	gotRows := snap.Groups[group]
	got := make([]string, 0, len(gotRows))
	for _, row := range gotRows {
		got = append(got, row.ID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("%s IDs = %#v, want %#v", group, got, want)
	}
}
