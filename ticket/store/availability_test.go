package store_test

import (
	"testing"
	"time"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/runtime"
	"github.com/php-workx/epos/ticket/store"
	"github.com/php-workx/epos/ticket/testutil"
)

func TestReadyTicketsExcludesClaimed(t *testing.T) {
	s := testutil.NewTestStore(t)
	ready := testutil.MustCreateTestTicket(t, s, "ready")
	claimed := testutil.MustCreateTestTicket(t, s, "claimed")

	if err := runtime.Claim(s.Dir, claimed.ID, "agent-1", "run-1", time.Hour); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	got, err := store.ReadyTickets(s)
	if err != nil {
		t.Fatalf("ReadyTickets: %v", err)
	}
	ids := ticketIDSet(got)
	if !ids[ready.ID] {
		t.Errorf("ReadyTickets: ready ticket %q missing from result", ready.ID)
	}
	if ids[claimed.ID] {
		t.Errorf("ReadyTickets: claimed ticket %q leaked into result", claimed.ID)
	}
}

func TestReadyChildrenExcludesClaimed(t *testing.T) {
	s := testutil.NewTestStore(t)
	parent := testutil.MustCreateTestTicket(t, s, "parent")

	childReady := ticket.NewTicket(ticket.WithTitle("child-ready"), ticket.WithType("task"), ticket.WithParent(parent.ID))
	childReady.ID = "child-ready-1"
	childReady.Present["id"] = true
	if err := s.Create(childReady); err != nil {
		t.Fatalf("Create childReady: %v", err)
	}

	childClaimed := ticket.NewTicket(ticket.WithTitle("child-claimed"), ticket.WithType("task"), ticket.WithParent(parent.ID))
	childClaimed.ID = "child-claimed-1"
	childClaimed.Present["id"] = true
	if err := s.Create(childClaimed); err != nil {
		t.Fatalf("Create childClaimed: %v", err)
	}

	if err := runtime.Claim(s.Dir, childClaimed.ID, "agent-1", "run-1", time.Hour); err != nil {
		t.Fatalf("Claim childClaimed: %v", err)
	}

	got, err := store.ReadyChildren(s, parent.ID)
	if err != nil {
		t.Fatalf("ReadyChildren: %v", err)
	}
	ids := ticketIDSet(got)
	if !ids[childReady.ID] {
		t.Errorf("ReadyChildren: ready child %q missing from result", childReady.ID)
	}
	if ids[childClaimed.ID] {
		t.Errorf("ReadyChildren: claimed child %q leaked into result", childClaimed.ID)
	}
}

func TestReadyChildrenOnlyReturnsChildren(t *testing.T) {
	s := testutil.NewTestStore(t)
	parent := testutil.MustCreateTestTicket(t, s, "parent")
	unrelated := testutil.MustCreateTestTicket(t, s, "unrelated")

	child := ticket.NewTicket(ticket.WithTitle("child"), ticket.WithType("task"), ticket.WithParent(parent.ID))
	child.ID = "child-of-parent-1"
	child.Present["id"] = true
	if err := s.Create(child); err != nil {
		t.Fatalf("Create child: %v", err)
	}

	got, err := store.ReadyChildren(s, parent.ID)
	if err != nil {
		t.Fatalf("ReadyChildren: %v", err)
	}
	ids := ticketIDSet(got)
	if !ids[child.ID] {
		t.Errorf("ReadyChildren: child %q missing from result", child.ID)
	}
	if ids[unrelated.ID] {
		t.Errorf("ReadyChildren: unrelated ticket %q leaked into result", unrelated.ID)
	}
}

func ticketIDSet(tickets []ticket.Ticket) map[string]bool {
	m := make(map[string]bool, len(tickets))
	for _, tk := range tickets {
		m[tk.ID] = true
	}
	return m
}
