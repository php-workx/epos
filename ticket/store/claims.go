package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
	"github.com/php-workx/epos/ticket/runtime"
)

// ActiveClaimSet returns the set of ticket IDs whose runtime sidecar reports
// an active (non-expired) claim. Sidecars without claims, with released
// claims, or with leases that have already expired are excluded.
//
// A nil error is returned when the claims directory does not yet exist.
func (s *FileStore) ActiveClaimSet() (map[string]bool, error) {
	claimsDir := filepath.Join(s.Dir, TicketsDir, ticket.ClaimsDir)
	entries, err := os.ReadDir(claimsDir)
	if errors.Is(err, os.ErrNotExist) {
		// Empty store — return an empty set so callers can use the result
		// without nil checks. nilnil lint forbids returning (nil, nil).
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read claims dir: %w", err)
	}
	now := time.Now()
	out := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ticket.ClaimsSuffix {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ticket.ClaimsSuffix)
		if id == "" {
			continue
		}
		state, rerr := runtime.ReadRuntimeState(s.Dir, id)
		if rerr != nil {
			// Skip unreadable sidecars rather than failing the whole listing —
			// a corrupt or partially-written file should not hide every other
			// claim from the caller.
			continue
		}
		if state.Claim == nil {
			continue
		}
		if state.Lease != nil && !state.Lease.ExpiresAt.IsZero() && now.After(state.Lease.ExpiresAt) {
			continue
		}
		out[id] = true
	}
	return out, nil
}

// ListReady returns the set of tickets that are ready to be worked, with
// active claims filtered out. This is the sidecar-aware equivalent of
// graph.ReadyFilter and is the function the "epos ready" command uses.
func (s *FileStore) ListReady() ([]ticket.Ticket, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	claimed, err := s.ActiveClaimSet()
	if err != nil {
		return nil, err
	}
	return graph.ReadyFilterUnclaimed(all, func(id string) bool { return claimed[id] }), nil
}
