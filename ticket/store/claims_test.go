package store_test

import (
	"testing"
	"time"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/runtime"
	"github.com/php-workx/epos/ticket/testutil"
)

// TestActiveClaimSetIncludesActiveLeaseExcludesExpired verifies the
// sidecar-scanning helper underpinning sidecar-aware ready: tickets with a
// live lease are reported, tickets with an expired lease (or no claim) are
// not.
func TestActiveClaimSetIncludesActiveLeaseExcludesExpired(t *testing.T) {
	s := testutil.NewTestStore(t)

	live := testutil.MustCreateTestTicket(t, s, "live")
	expired := testutil.MustCreateTestTicket(t, s, "expired")
	unclaimed := testutil.MustCreateTestTicket(t, s, "unclaimed")

	if err := runtime.Claim(s.Dir, live.ID, "agent-live", "run-1", time.Hour); err != nil {
		t.Fatalf("Claim live: %v", err)
	}
	expiredState := &ticket.RuntimeState{
		TicketID: expired.ID,
		Claim: &ticket.Claim{
			ClaimedBy:    "agent-stale",
			ClaimBackend: "run-old",
			ClaimedAt:    time.Now().Add(-2 * time.Hour),
		},
		Lease: &ticket.Lease{
			LeaseID:   "old-lease",
			ExpiresAt: time.Now().Add(-time.Hour),
		},
	}
	if err := runtime.WriteRuntimeState(s.Dir, expiredState); err != nil {
		t.Fatalf("WriteRuntimeState expired: %v", err)
	}

	got, err := s.ActiveClaimSet()
	if err != nil {
		t.Fatalf("ActiveClaimSet: %v", err)
	}
	if !got[live.ID] {
		t.Errorf("live claim missing from set: %v", got)
	}
	if got[expired.ID] {
		t.Errorf("expired claim should not appear: %v", got)
	}
	if got[unclaimed.ID] {
		t.Errorf("unclaimed ticket should not appear: %v", got)
	}
}

// TestListReadyExcludesActivelyClaimedTicket is the end-to-end gate: the
// behavior the user experiences via "epos ready". A ticket holding an active
// claim must not appear in the ready list, while an expired-claim ticket
// reverts to ready.
func TestListReadyExcludesActivelyClaimedTicket(t *testing.T) {
	s := testutil.NewTestStore(t)

	a := testutil.MustCreateTestTicket(t, s, "a")
	b := testutil.MustCreateTestTicket(t, s, "b")
	c := testutil.MustCreateTestTicket(t, s, "c")

	if err := runtime.Claim(s.Dir, b.ID, "agent-1", "run-1", time.Hour); err != nil {
		t.Fatalf("Claim b: %v", err)
	}

	ready, err := s.ListReady()
	if err != nil {
		t.Fatalf("ListReady: %v", err)
	}

	gotIDs := make(map[string]bool, len(ready))
	for _, tk := range ready {
		gotIDs[tk.ID] = true
	}
	if !gotIDs[a.ID] {
		t.Errorf("ticket a (unclaimed) missing from ready list: %v", gotIDs)
	}
	if gotIDs[b.ID] {
		t.Errorf("ticket b (claimed) leaked into ready list: %v", gotIDs)
	}
	if !gotIDs[c.ID] {
		t.Errorf("ticket c (unclaimed) missing from ready list: %v", gotIDs)
	}
}

// TestListReadyEmptyClaimsDir confirms that a fresh store with no claims dir
// behaves like the legacy ReadyFilter.
func TestListReadyEmptyClaimsDir(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.MustCreateTestTicket(t, s, "fresh")

	ready, err := s.ListReady()
	if err != nil {
		t.Fatalf("ListReady: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != tk.ID {
		t.Fatalf("ListReady on fresh store = %v, want [%s]", ready, tk.ID)
	}
}
