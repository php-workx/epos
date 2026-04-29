package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/php-workx/epos/ticket"
)

// TestClaimAndRead verifies that Claim writes a sidecar and ReadRuntimeState
// returns the expected claim owner.
func TestClaimAndRead(t *testing.T) {
	dir := t.TempDir()
	if err := Claim(dir, "abc-1234", "agent-1", "local", DefaultLeaseDuration); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	state, err := ReadRuntimeState(dir, "abc-1234")
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	if state.Claim == nil {
		t.Fatal("expected Claim to be set after Claim()")
	}
	if state.Claim.ClaimedBy != "agent-1" {
		t.Errorf("ClaimedBy = %q, want %q", state.Claim.ClaimedBy, "agent-1")
	}
	if state.Lease == nil {
		t.Fatal("expected Lease to be set after Claim()")
	}
	if state.Lease.ExpiresAt.IsZero() {
		t.Error("Lease.ExpiresAt must not be zero")
	}
}

func TestRuntimeStateRejectsInvalidTicketIDs(t *testing.T) {
	dir := t.TempDir()
	invalidIDs := []string{"../abc-1234", "abc/1234", "abc\\1234"}

	for _, id := range invalidIDs {
		t.Run("Read "+id, func(t *testing.T) {
			_, err := ReadRuntimeState(dir, id)
			requireValidationError(t, err)
		})

		t.Run("Write "+id, func(t *testing.T) {
			err := WriteRuntimeState(dir, &ticket.RuntimeState{TicketID: id})
			requireValidationError(t, err)
		})

		t.Run("Claim "+id, func(t *testing.T) {
			err := Claim(dir, id, "agent-1", "local", DefaultLeaseDuration)
			requireValidationError(t, err)
		})
	}
}

func TestWriteRuntimeStateRejectsNilState(t *testing.T) {
	err := WriteRuntimeState(t.TempDir(), nil)
	requireValidationError(t, err)
}

func TestReadRuntimeStateBackfillsEmptyTicketID(t *testing.T) {
	dir := t.TempDir()
	if err := writeRawRuntimeState(dir, "abc-empty", ticket.RuntimeState{Status: ticket.StatusPending}); err != nil {
		t.Fatalf("writeRawRuntimeState: %v", err)
	}

	state, err := ReadRuntimeState(dir, "abc-empty")
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	if state.TicketID != "abc-empty" {
		t.Fatalf("TicketID = %q, want canonical id", state.TicketID)
	}
}

func TestReadRuntimeStateRejectsMismatchedTicketID(t *testing.T) {
	dir := t.TempDir()
	if err := writeRawRuntimeState(dir, "abc-canonical", ticket.RuntimeState{TicketID: "abc-other"}); err != nil {
		t.Fatalf("writeRawRuntimeState: %v", err)
	}

	_, err := ReadRuntimeState(dir, "abc-canonical")
	if err == nil || !strings.Contains(err.Error(), "mismatched ticket_id") {
		t.Fatalf("ReadRuntimeState error = %v, want mismatched ticket_id", err)
	}
}

func TestWriteRuntimeStateConcurrentWriters(t *testing.T) {
	dir := t.TempDir()

	const writers = 20
	start := make(chan struct{})
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs <- WriteRuntimeState(dir, &ticket.RuntimeState{
				TicketID: "abc-race",
				Claim: &ticket.Claim{
					ClaimedBy:    fmt.Sprintf("agent-%d", idx),
					ClaimBackend: "run",
					ClaimedAt:    time.Now(),
				},
			})
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("WriteRuntimeState concurrent writer failed: %v", err)
		}
	}

	state, err := ReadRuntimeState(dir, "abc-race")
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	if state.TicketID != "abc-race" || state.Claim == nil {
		t.Fatalf("unexpected final state: %+v", state)
	}

	entries, err := os.ReadDir(resolveClaimsDir(dir))
	if err != nil {
		t.Fatalf("ReadDir claims: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Fatalf("leftover temp runtime sidecar: %s", entry.Name())
		}
	}
}

func writeRawRuntimeState(dir, ticketID string, state ticket.RuntimeState) error {
	claimsDir := resolveClaimsDir(dir)
	if err := os.MkdirAll(claimsDir, 0o750); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(sidecarPath(dir, ticketID), data, 0o644)
}

// TestClaimConflict verifies that when two goroutines race to claim the same
// ticket, exactly one succeeds and the other receives *ticket.AlreadyClaimedError.
func TestClaimConflict(t *testing.T) {
	dir := t.TempDir()

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ownerID := fmt.Sprintf("agent-%d", idx+1)
			errs[idx] = Claim(dir, "abc-1234", ownerID, "local", DefaultLeaseDuration)
		}(i)
	}
	wg.Wait()

	nilCount := 0
	for _, err := range errs {
		if err == nil {
			nilCount++
			continue
		}
		var claimErr *ticket.AlreadyClaimedError
		if !errors.As(err, &claimErr) {
			t.Errorf("expected *ticket.AlreadyClaimedError, got %T: %v", err, err)
		}
	}
	if nilCount != 1 {
		t.Errorf("expected exactly 1 successful claim, got %d (errors: %v, %v)", nilCount, errs[0], errs[1])
	}
}

// TestClaimEligibility verifies that Claim enforces ticket status eligibility.
func TestClaimEligibility(t *testing.T) {
	t.Run("pending is eligible (no sidecar)", func(t *testing.T) {
		dir := t.TempDir()
		if err := Claim(dir, "abc-pend", "agent-1", "local", DefaultLeaseDuration); err != nil {
			t.Fatalf("expected pending ticket (no sidecar) to be claimable, got: %v", err)
		}
	})

	t.Run("repair_pending is eligible", func(t *testing.T) {
		dir := t.TempDir()
		pre := &ticket.RuntimeState{TicketID: "abc-rep", Status: ticket.StatusRepairPending}
		if err := WriteRuntimeState(dir, pre); err != nil {
			t.Fatal(err)
		}
		if err := Claim(dir, "abc-rep", "agent-1", "local", DefaultLeaseDuration); err != nil {
			t.Fatalf("expected repair_pending ticket to be claimable, got: %v", err)
		}
	})

	t.Run("closed is not eligible", func(t *testing.T) {
		dir := t.TempDir()
		pre := &ticket.RuntimeState{TicketID: "abc-close", Status: ticket.StatusClosed}
		if err := WriteRuntimeState(dir, pre); err != nil {
			t.Fatal(err)
		}
		err := Claim(dir, "abc-close", "agent-1", "local", DefaultLeaseDuration)
		if err == nil {
			t.Fatal("expected error claiming closed ticket, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "not eligible") && !strings.Contains(msg, "only pending or repair_pending") {
			t.Errorf("error %q must contain 'not eligible' or 'only pending or repair_pending'", msg)
		}
	})

	t.Run("done is not eligible", func(t *testing.T) {
		dir := t.TempDir()
		pre := &ticket.RuntimeState{TicketID: "abc-done", Status: ticket.StatusDone}
		if err := WriteRuntimeState(dir, pre); err != nil {
			t.Fatal(err)
		}
		err := Claim(dir, "abc-done", "agent-1", "local", DefaultLeaseDuration)
		if err == nil {
			t.Fatal("expected error claiming done ticket, got nil")
		}
	})
}

// TestSameOwnerReclaim verifies that claiming a ticket a second time with the
// same ownerID is idempotent and extends the lease.
func TestSameOwnerReclaim(t *testing.T) {
	dir := t.TempDir()

	if err := Claim(dir, "abc-1234", "agent-1", "local", DefaultLeaseDuration); err != nil {
		t.Fatalf("first Claim: %v", err)
	}

	first, err := ReadRuntimeState(dir, "abc-1234")
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	origExpiry := first.Lease.ExpiresAt

	// Ensure time advances before the second claim.
	time.Sleep(10 * time.Millisecond)

	// Second claim by same owner with a longer duration: must succeed.
	if err := Claim(dir, "abc-1234", "agent-1", "local", 30*time.Minute); err != nil {
		t.Fatalf("second Claim (same owner): %v", err)
	}

	second, err := ReadRuntimeState(dir, "abc-1234")
	if err != nil {
		t.Fatalf("ReadRuntimeState after reclaim: %v", err)
	}
	if second.Claim == nil {
		t.Fatal("expected Claim to still be set after same-owner reclaim")
	}
	if second.Claim.ClaimedBy != "agent-1" {
		t.Errorf("ClaimedBy = %q, want %q", second.Claim.ClaimedBy, "agent-1")
	}
	if !second.Lease.ExpiresAt.After(origExpiry) {
		t.Errorf("expected extended expiry: before=%v, after=%v", origExpiry, second.Lease.ExpiresAt)
	}
}

// TestRelease verifies that Release clears the claim and records the new status.
func TestRelease(t *testing.T) {
	dir := t.TempDir()

	if err := Claim(dir, "abc-1234", "agent-1", "local", DefaultLeaseDuration); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	if err := Release(dir, "abc-1234", "agent-1", ticket.StatusPending, "done"); err != nil {
		t.Fatalf("Release: %v", err)
	}

	state, err := ReadRuntimeState(dir, "abc-1234")
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	if state.Claim != nil {
		t.Errorf("expected Claim=nil after release, got %+v", state.Claim)
	}
	if state.Lease != nil {
		t.Errorf("expected Lease=nil after release, got %+v", state.Lease)
	}
	if state.Status != ticket.StatusPending {
		t.Errorf("expected Status=%q after release, got %q", ticket.StatusPending, state.Status)
	}
}

// TestRenew verifies that Renew extends the lease expiry.
func TestRenew(t *testing.T) {
	dir := t.TempDir()

	if err := Claim(dir, "abc-1234", "agent-1", "local", DefaultLeaseDuration); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	before, err := ReadRuntimeState(dir, "abc-1234")
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	origExpiry := before.Lease.ExpiresAt

	time.Sleep(10 * time.Millisecond)

	if err := Renew(dir, "abc-1234", "agent-1", 30*time.Minute); err != nil {
		t.Fatalf("Renew: %v", err)
	}

	after, err := ReadRuntimeState(dir, "abc-1234")
	if err != nil {
		t.Fatalf("ReadRuntimeState after Renew: %v", err)
	}
	if after.Lease == nil {
		t.Fatal("expected Lease to be set after Renew")
	}
	if !after.Lease.ExpiresAt.After(origExpiry) {
		t.Errorf("expected extended expiry after Renew: before=%v, after=%v", origExpiry, after.Lease.ExpiresAt)
	}
}

// TestClaimAllowsReady verifies the ready-transition eligibility rules.
func TestClaimAllowsReady(t *testing.T) {
	t.Run("unclaimed ticket returns true", func(t *testing.T) {
		dir := t.TempDir()
		ok, err := ClaimAllowsReady(dir, "abc-1234", "run-1")
		if err != nil {
			t.Fatalf("ClaimAllowsReady: %v", err)
		}
		if !ok {
			t.Error("expected true for unclaimed ticket")
		}
	})

	t.Run("ticket claimed by different run returns false", func(t *testing.T) {
		dir := t.TempDir()
		if err := Claim(dir, "abc-1234", "agent-1", "run-1", DefaultLeaseDuration); err != nil {
			t.Fatalf("Claim: %v", err)
		}
		ok, err := ClaimAllowsReady(dir, "abc-1234", "run-2") // different run
		if err != nil {
			t.Fatalf("ClaimAllowsReady: %v", err)
		}
		if ok {
			t.Error("expected false for ticket claimed by different run with valid lease")
		}
	})

	t.Run("ticket claimed by same run returns true", func(t *testing.T) {
		dir := t.TempDir()
		if err := Claim(dir, "abc-1234", "agent-1", "run-1", DefaultLeaseDuration); err != nil {
			t.Fatalf("Claim: %v", err)
		}
		ok, err := ClaimAllowsReady(dir, "abc-1234", "run-1") // same run
		if err != nil {
			t.Fatalf("ClaimAllowsReady: %v", err)
		}
		if !ok {
			t.Error("expected true for ticket claimed by the same run")
		}
	})
}

// TestExpiredLeaseAllowsReady verifies that a ticket with an expired lease allows
// a ready transition.
func TestExpiredLeaseAllowsReady(t *testing.T) {
	dir := t.TempDir()

	expired := &ticket.RuntimeState{
		TicketID: "abc-exp",
		Claim: &ticket.Claim{
			ClaimedBy:    "agent-1",
			ClaimBackend: "run-1",
			ClaimedAt:    time.Now().Add(-2 * time.Hour),
		},
		Lease: &ticket.Lease{
			LeaseID:   "expired-lease",
			ExpiresAt: time.Now().Add(-1 * time.Hour), // expired one hour ago
		},
	}
	if err := WriteRuntimeState(dir, expired); err != nil {
		t.Fatalf("WriteRuntimeState: %v", err)
	}

	ok, err := ClaimAllowsReady(dir, "abc-exp", "run-2") // different run
	if err != nil {
		t.Fatalf("ClaimAllowsReady: %v", err)
	}
	if !ok {
		t.Error("expected true for ticket with expired lease")
	}
}

// TestReadClaimsForRun verifies that ReadClaimsForRun returns only sidecars
// whose ClaimBackend matches the requested runID.
func TestReadClaimsForRun(t *testing.T) {
	dir := t.TempDir()

	if err := Claim(dir, "ticket-a", "agent-1", "run-x", DefaultLeaseDuration); err != nil {
		t.Fatalf("Claim ticket-a: %v", err)
	}
	if err := Claim(dir, "ticket-b", "agent-2", "run-y", DefaultLeaseDuration); err != nil {
		t.Fatalf("Claim ticket-b: %v", err)
	}
	if err := Claim(dir, "ticket-c", "agent-3", "run-x", DefaultLeaseDuration); err != nil {
		t.Fatalf("Claim ticket-c: %v", err)
	}

	results, err := ReadClaimsForRun(dir, "run-x")
	if err != nil {
		t.Fatalf("ReadClaimsForRun: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 claims for run-x, got %d", len(results))
	}
	for _, s := range results {
		if s.Claim.ClaimBackend != "run-x" {
			t.Errorf("unexpected ClaimBackend %q in results for run-x", s.Claim.ClaimBackend)
		}
	}
}

// TestReadClaimsForRunEmpty verifies that an empty or nonexistent claims directory
// returns nil without error.
func TestReadClaimsForRunEmpty(t *testing.T) {
	dir := t.TempDir()
	results, err := ReadClaimsForRun(dir, "run-x")
	if err != nil {
		t.Fatalf("ReadClaimsForRun on empty dir: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil for empty dir, got %v", results)
	}
}

// TestReclaimExpired verifies that an expired claim can be overridden.
func TestReclaimExpired(t *testing.T) {
	dir := t.TempDir()

	// Set up an expired claim.
	expired := &ticket.RuntimeState{
		TicketID: "abc-exp",
		Claim: &ticket.Claim{
			ClaimedBy:    "agent-old",
			ClaimBackend: "run-old",
			ClaimedAt:    time.Now().Add(-2 * time.Hour),
		},
		Lease: &ticket.Lease{
			LeaseID:   "old-lease",
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		},
	}
	if err := WriteRuntimeState(dir, expired); err != nil {
		t.Fatalf("WriteRuntimeState: %v", err)
	}

	if err := ReclaimExpired(dir, "abc-exp", "agent-new", "run-new", DefaultLeaseDuration); err != nil {
		t.Fatalf("ReclaimExpired: %v", err)
	}

	state, err := ReadRuntimeState(dir, "abc-exp")
	if err != nil {
		t.Fatalf("ReadRuntimeState: %v", err)
	}
	if state.Claim == nil {
		t.Fatal("expected Claim to be set after ReclaimExpired")
	}
	if state.Claim.ClaimedBy != "agent-new" {
		t.Errorf("ClaimedBy = %q, want %q", state.Claim.ClaimedBy, "agent-new")
	}
}

// TestReclaimExpiredBlockedOnValidLease verifies that ReclaimExpired returns
// AlreadyClaimedError when the existing lease is still valid.
func TestReclaimExpiredBlockedOnValidLease(t *testing.T) {
	dir := t.TempDir()

	if err := Claim(dir, "abc-1234", "agent-1", "run-1", DefaultLeaseDuration); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	err := ReclaimExpired(dir, "abc-1234", "agent-2", "run-2", DefaultLeaseDuration)
	if err == nil {
		t.Fatal("expected AlreadyClaimedError when lease is still valid")
	}
	var claimErr *ticket.AlreadyClaimedError
	if !errors.As(err, &claimErr) {
		t.Errorf("expected *ticket.AlreadyClaimedError, got %T: %v", err, err)
	}
}

func TestReclaimExpiredRejectsIneligibleStatus(t *testing.T) {
	dir := t.TempDir()

	expiredClosed := &ticket.RuntimeState{
		TicketID: "abc-closed",
		Status:   ticket.StatusClosed,
		Claim: &ticket.Claim{
			ClaimedBy:    "agent-old",
			ClaimBackend: "run-old",
			ClaimedAt:    time.Now().Add(-2 * time.Hour),
		},
		Lease: &ticket.Lease{
			LeaseID:   "old-lease",
			ExpiresAt: time.Now().Add(-time.Hour),
		},
	}
	if err := WriteRuntimeState(dir, expiredClosed); err != nil {
		t.Fatalf("WriteRuntimeState: %v", err)
	}

	err := ReclaimExpired(dir, "abc-closed", "agent-new", "run-new", DefaultLeaseDuration)
	if err == nil {
		t.Fatal("expected ineligible status error, got nil")
	}
	if !strings.Contains(err.Error(), "not eligible") {
		t.Fatalf("expected not eligible error, got: %v", err)
	}
	state, readErr := ReadRuntimeState(dir, "abc-closed")
	if readErr != nil {
		t.Fatalf("ReadRuntimeState: %v", readErr)
	}
	if state.Claim == nil || state.Claim.ClaimedBy != "agent-old" {
		t.Fatalf("claim changed despite rejected reclaim: %+v", state.Claim)
	}
}

// TestValidateClaimEligibility directly exercises the eligibility helper.
func TestValidateClaimEligibility(t *testing.T) {
	eligible := []ticket.Status{
		"", // empty → treated as pending
		ticket.StatusPending,
		ticket.StatusRepairPending,
	}
	for _, s := range eligible {
		if err := validateClaimEligibility(s); err != nil {
			t.Errorf("validateClaimEligibility(%q) expected nil, got: %v", s, err)
		}
	}

	ineligible := []ticket.Status{
		ticket.StatusClosed,
		ticket.StatusDone,
		ticket.StatusFailed,
		ticket.StatusClaimed,
		ticket.StatusImplementing,
		ticket.StatusUnderReview,
		ticket.StatusHeld,
		ticket.StatusBlocked,
	}
	for _, s := range ineligible {
		err := validateClaimEligibility(s)
		if err == nil {
			t.Errorf("validateClaimEligibility(%q) expected error, got nil", s)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, "not eligible") && !strings.Contains(msg, "only pending or repair_pending") {
			t.Errorf("validateClaimEligibility(%q) error %q must mention 'not eligible' or 'only pending or repair_pending'", s, msg)
		}
	}
}

// TestValidateClaimIdentifier exercises the identifier validation helper.
func TestValidateClaimIdentifier(t *testing.T) {
	if err := validateClaimIdentifier("abc-1234", "agent-1"); err != nil {
		t.Errorf("expected no error for valid identifiers, got: %v", err)
	}
	if err := validateClaimIdentifier("", "agent-1"); err == nil {
		t.Error("expected error for empty ticketID")
	}
	if err := validateClaimIdentifier("../abc-1234", "agent-1"); err == nil {
		t.Error("expected error for invalid ticketID")
	}
	if err := validateClaimIdentifier("abc-1234", ""); err == nil {
		t.Error("expected error for empty ownerID")
	}
}

// TestReleaseNotOwner verifies that Release returns NotClaimOwnerError when the
// caller does not own the claim.
func TestReleaseNotOwner(t *testing.T) {
	dir := t.TempDir()

	if err := Claim(dir, "abc-1234", "agent-1", "local", DefaultLeaseDuration); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	err := Release(dir, "abc-1234", "agent-2", ticket.StatusPending, "")
	if err == nil {
		t.Fatal("expected NotClaimOwnerError, got nil")
	}
	var notOwner *ticket.NotClaimOwnerError
	if !errors.As(err, &notOwner) {
		t.Errorf("expected *ticket.NotClaimOwnerError, got %T: %v", err, err)
	}
}

// TestReleaseNotClaimed verifies that Release returns NotClaimedError when the
// ticket has no active claim.
func TestReleaseNotClaimed(t *testing.T) {
	dir := t.TempDir()
	err := Release(dir, "abc-1234", "agent-1", ticket.StatusPending, "")
	if err == nil {
		t.Fatal("expected NotClaimedError, got nil")
	}
	var notClaimed *ticket.NotClaimedError
	if !errors.As(err, &notClaimed) {
		t.Errorf("expected *ticket.NotClaimedError, got %T: %v", err, err)
	}
}

func requireValidationError(t *testing.T, err error) {
	t.Helper()
	var validation *ticket.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected *ticket.ValidationError, got %T: %v", err, err)
	}
}
