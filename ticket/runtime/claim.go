package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/php-workx/epos/ticket"
)

// The default lease duration is 15 minutes; re-exported from the ticket package
// so callers that import only the runtime subpackage can reference it directly.
const DefaultLeaseDuration = ticket.DefaultLeaseDuration

// resolveClaimsDir returns the absolute path of the .claims directory within
// the ticket store rooted at dir. dir is expected to be the repository root
// that contains the .tickets/ subdirectory.
func resolveClaimsDir(dir string) string {
	return filepath.Join(dir, ".tickets", ticket.ClaimsDir)
}

// sidecarPath returns the JSON sidecar path for a validated ticketID within dir.
func sidecarPath(dir, ticketID string) string {
	return filepath.Join(resolveClaimsDir(dir), ticketID+ticket.ClaimsSuffix)
}

// flockPath returns the advisory lock file path for a sidecar path p.
func flockPath(p string) string {
	return p + ".lock"
}

// ReadRuntimeState reads the runtime sidecar for ticketID.
// If no sidecar exists yet, a zero-value RuntimeState (with TicketID set) is returned.
func ReadRuntimeState(dir, ticketID string) (*ticket.RuntimeState, error) {
	if err := validateRuntimeTicketID(ticketID); err != nil {
		return nil, err
	}
	path := sidecarPath(dir, ticketID)
	data, err := os.ReadFile(path) //nolint:gosec // G304 G703: path uses a validated ticket ID
	if errors.Is(err, os.ErrNotExist) {
		return &ticket.RuntimeState{TicketID: ticketID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read runtime state %q: %w", ticketID, err)
	}
	var state ticket.RuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse runtime state %q: %w", ticketID, err)
	}
	return &state, nil
}

// WriteRuntimeState atomically writes the runtime sidecar for state.TicketID.
// It creates the claims directory if it does not exist, writes to a temp file,
// then renames the temp file into place.
func WriteRuntimeState(dir string, state *ticket.RuntimeState) error {
	if state == nil {
		return &ticket.ValidationError{Field: "state", Message: "must not be nil"}
	}
	if err := validateRuntimeTicketID(state.TicketID); err != nil {
		return err
	}
	claimsDir := resolveClaimsDir(dir)
	if err := os.MkdirAll(claimsDir, 0o750); err != nil {
		return fmt.Errorf("create claims dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runtime state %q: %w", state.TicketID, err)
	}
	path := sidecarPath(dir, state.TicketID)
	tmp, err := os.CreateTemp(claimsDir, state.TicketID+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp runtime state %q: %w", state.TicketID, err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpName)
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	if _, err = tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp runtime state %q: %w", state.TicketID, err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync temp runtime state %q: %w", state.TicketID, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp runtime state %q: %w", state.TicketID, err)
	}
	if err = os.Chmod(tmpName, 0o644); err != nil { //nolint:gosec // G302: sidecars are non-secret local runtime state
		return fmt.Errorf("chmod temp runtime state %q: %w", state.TicketID, err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit runtime state %q: %w", state.TicketID, err)
	}
	cleanup = func() {}
	claims, err := os.Open(claimsDir) //nolint:gosec // G304: claimsDir is derived from caller-provided store root
	if err != nil {
		return fmt.Errorf("open claims dir: %w", err)
	}
	defer func() { _ = claims.Close() }()
	if err = claims.Sync(); err != nil {
		return fmt.Errorf("fsync claims dir: %w", err)
	}
	return nil
}

func validateRuntimeTicketID(ticketID string) error {
	if err := ticket.ValidateID(ticketID); err != nil {
		var validation *ticket.ValidationError
		if errors.As(err, &validation) {
			return &ticket.ValidationError{Field: "ticket_id", Message: validation.Message}
		}
		return err
	}
	return nil
}

// validateClaimIdentifier returns a ValidationError if ticketID or ownerID is empty.
func validateClaimIdentifier(ticketID, ownerID string) error {
	if err := validateRuntimeTicketID(ticketID); err != nil {
		return err
	}
	if ownerID == "" {
		return &ticket.ValidationError{Field: "owner_id", Message: "must not be empty"}
	}
	return nil
}

// eligibilityCheck returns nil when status permits a new claim (empty, pending,
// or repair_pending) and an error for all other statuses.
func validateClaimEligibility(status ticket.Status) error {
	if status == "" || status == ticket.StatusPending || status == ticket.StatusRepairPending {
		return nil
	}
	return fmt.Errorf("ticket is not eligible for claiming: only pending or repair_pending tickets can be claimed, got %q", status)
}

// withExclusiveLock acquires an exclusive flock on the sidecar for ticketID,
// calls fn with the current state. If fn returns a non-nil state and nil error,
// withExclusiveLock writes the returned state before releasing the lock.
// The claims directory is created if it does not exist.
func withExclusiveLock(dir, ticketID string, fn func(*ticket.RuntimeState) (*ticket.RuntimeState, error)) error {
	if err := validateRuntimeTicketID(ticketID); err != nil {
		return err
	}
	claimsDir := resolveClaimsDir(dir)
	if err := os.MkdirAll(claimsDir, 0o750); err != nil {
		return fmt.Errorf("create claims dir: %w", err)
	}
	lp := flockPath(sidecarPath(dir, ticketID))
	fl := flock.New(lp)
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("acquire claim lock %q: %w", ticketID, err)
	}
	defer func() { _ = fl.Unlock() }()

	state, err := ReadRuntimeState(dir, ticketID)
	if err != nil {
		return err
	}
	newState, err := fn(state)
	if err != nil {
		return err
	}
	if newState != nil {
		return WriteRuntimeState(dir, newState)
	}
	return nil
}

// Claim acquires an exclusive lease on ticketID for ownerID within runID.
//
// Eligibility rules enforced inside the exclusive lock:
//   - Ticket sidecar status must be empty (treated as pending), pending, or repair_pending.
//   - If the ticket is already claimed by ownerID the call is idempotent and extends the lease.
//   - If the ticket is claimed by a different owner an *ticket.AlreadyClaimedError is returned.
func Claim(dir, ticketID, ownerID, runID string, duration time.Duration) error {
	if err := validateClaimIdentifier(ticketID, ownerID); err != nil {
		return err
	}
	return withExclusiveLock(dir, ticketID, func(state *ticket.RuntimeState) (*ticket.RuntimeState, error) {
		// Same-owner reclaim: idempotent — extend the lease without re-checking eligibility.
		if state.Claim != nil && state.Claim.ClaimedBy == ownerID {
			leaseID := fmt.Sprintf("%s-%d", ticketID, time.Now().UnixNano())
			if state.Lease != nil && state.Lease.LeaseID != "" {
				leaseID = state.Lease.LeaseID
			}
			state.Claim.ClaimBackend = runID
			state.Lease = &ticket.Lease{
				LeaseID:   leaseID,
				ExpiresAt: time.Now().Add(duration),
			}
			return state, nil
		}

		// Already claimed by a different owner.
		if state.Claim != nil {
			return nil, &ticket.AlreadyClaimedError{TicketID: ticketID, ClaimedBy: state.Claim.ClaimedBy}
		}

		// New claim: eligibility check (inlined; the standalone helper is defined above).
		if state.Status != "" && state.Status != ticket.StatusPending && state.Status != ticket.StatusRepairPending {
			return nil, fmt.Errorf("ticket %q is not eligible for claiming: only pending or repair_pending tickets can be claimed, got %q", ticketID, state.Status)
		}

		now := time.Now()
		state.Claim = &ticket.Claim{
			ClaimedBy:    ownerID,
			ClaimBackend: runID,
			ClaimedAt:    now,
		}
		state.Lease = &ticket.Lease{
			LeaseID:   fmt.Sprintf("%s-%d", ticketID, now.UnixNano()),
			ExpiresAt: now.Add(duration),
		}
		state.Heartbeat = &ticket.Heartbeat{LastBeat: now}
		return state, nil
	})
}

// Release removes the claim on ticketID held by ownerID.
// newStatus is written to the sidecar so that subsequent Claim calls can
// enforce eligibility (e.g. a closed ticket cannot be re-claimed).
// reason is accepted for caller documentation purposes but is not persisted in MVP.
func Release(dir, ticketID, ownerID string, newStatus ticket.Status, reason string) error {
	if err := validateClaimIdentifier(ticketID, ownerID); err != nil {
		return err
	}
	_ = reason // accepted for API forward-compatibility; not persisted in MVP
	return withExclusiveLock(dir, ticketID, func(state *ticket.RuntimeState) (*ticket.RuntimeState, error) {
		if state.Claim == nil {
			return nil, &ticket.NotClaimedError{TicketID: ticketID}
		}
		if state.Claim.ClaimedBy != ownerID {
			return nil, &ticket.NotClaimOwnerError{
				TicketID:  ticketID,
				ClaimedBy: state.Claim.ClaimedBy,
				Caller:    ownerID,
			}
		}
		state.Claim = nil
		state.Lease = nil
		state.Heartbeat = nil
		state.Status = newStatus
		return state, nil
	})
}

// Renew extends the lease on ticketID held by ownerID by extension duration from now.
func Renew(dir, ticketID, ownerID string, extension time.Duration) error {
	if err := validateClaimIdentifier(ticketID, ownerID); err != nil {
		return err
	}
	return withExclusiveLock(dir, ticketID, func(state *ticket.RuntimeState) (*ticket.RuntimeState, error) {
		if state.Claim == nil {
			return nil, &ticket.NotClaimedError{TicketID: ticketID}
		}
		if state.Claim.ClaimedBy != ownerID {
			return nil, &ticket.NotClaimOwnerError{
				TicketID:  ticketID,
				ClaimedBy: state.Claim.ClaimedBy,
				Caller:    ownerID,
			}
		}
		now := time.Now()
		if state.Lease == nil {
			state.Lease = &ticket.Lease{
				LeaseID:   fmt.Sprintf("%s-renewed-%d", ticketID, now.UnixNano()),
				ExpiresAt: now.Add(extension),
			}
		} else {
			state.Lease.ExpiresAt = now.Add(extension)
		}
		state.Heartbeat = &ticket.Heartbeat{LastBeat: now}
		return state, nil
	})
}

// ReclaimExpired transfers ownership of ticketID to ownerID/runID if the current
// lease has expired or is absent. Returns *ticket.AlreadyClaimedError if a
// different owner holds a lease that is still valid.
func ReclaimExpired(dir, ticketID, ownerID, runID string, duration time.Duration) error {
	if err := validateClaimIdentifier(ticketID, ownerID); err != nil {
		return err
	}
	return withExclusiveLock(dir, ticketID, func(state *ticket.RuntimeState) (*ticket.RuntimeState, error) {
		// Block reclaim if a different owner holds a valid (non-expired) lease.
		if state.Claim != nil && state.Claim.ClaimedBy != ownerID {
			if state.Lease != nil && time.Now().Before(state.Lease.ExpiresAt) {
				return nil, &ticket.AlreadyClaimedError{TicketID: ticketID, ClaimedBy: state.Claim.ClaimedBy}
			}
		}
		if err := validateClaimEligibility(state.Status); err != nil {
			return nil, err
		}
		now := time.Now()
		state.Claim = &ticket.Claim{
			ClaimedBy:    ownerID,
			ClaimBackend: runID,
			ClaimedAt:    now,
		}
		state.Lease = &ticket.Lease{
			LeaseID:   fmt.Sprintf("%s-%d", ticketID, now.UnixNano()),
			ExpiresAt: now.Add(duration),
		}
		state.Heartbeat = &ticket.Heartbeat{LastBeat: now}
		return state, nil
	})
}

// ReadClaimsForRun returns all RuntimeState sidecars for which the active claim's
// ClaimBackend equals runID. Returns nil (not an error) if the claims directory
// does not exist yet.
func ReadClaimsForRun(dir, runID string) ([]*ticket.RuntimeState, error) {
	claimsDir := resolveClaimsDir(dir)
	entries, err := os.ReadDir(claimsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read claims dir: %w", err)
	}
	var results []*ticket.RuntimeState
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ticket.ClaimsSuffix {
			continue
		}
		data, err := os.ReadFile(filepath.Join(claimsDir, entry.Name())) //nolint:gosec // G304: entry.Name() is a directory entry, cannot contain path separators
		if err != nil {
			return nil, fmt.Errorf("read claim file %q: %w", entry.Name(), err)
		}
		var state ticket.RuntimeState
		if err := json.Unmarshal(data, &state); err != nil {
			continue // skip corrupt sidecars
		}
		if state.Claim != nil && state.Claim.ClaimBackend == runID {
			results = append(results, &state)
		}
	}
	return results, nil
}

// ClaimAllowsReady reports whether the ticket may be transitioned to ready/pending
// by a scheduler associated with runID.
//
//   - Unclaimed ticket → true.
//   - Claim held by the same runID → true (idempotent for the same run).
//   - Claim held by a different runID with an expired or absent lease → true.
//   - Claim held by a different runID with a valid lease → false.
func ClaimAllowsReady(dir, ticketID, runID string) (bool, error) {
	state, err := ReadRuntimeState(dir, ticketID)
	if err != nil {
		return false, err
	}
	if state.Claim == nil {
		return true, nil
	}
	// Same run: always allow (the run already owns it).
	if state.Claim.ClaimBackend == runID {
		return true, nil
	}
	// Different run: allow only if the lease is absent or expired.
	if state.Lease != nil && time.Now().Before(state.Lease.ExpiresAt) {
		return false, nil
	}
	return true, nil
}
