package store

import (
	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
)

// claimedSnapshot fetches the ticket list and active claim set as two
// independent snapshots. See ReadyTickets for the non-atomic caveat.
func claimedSnapshot(s Store) ([]ticket.Ticket, func(string) bool, error) {
	tickets, err := s.List()
	if err != nil {
		return nil, nil, err
	}
	claimed, err := s.ActiveClaimSet()
	if err != nil {
		return nil, nil, err
	}
	return tickets, func(id string) bool { return claimed[id] }, nil
}

// ReadyTickets returns all tickets that are ready to be worked: no open
// blocking dependencies and no active claim. Callers that want to include
// claimed tickets should use s.List and graph.ReadyFilterUnclaimed directly.
//
// NOTE: List and ActiveClaimSet are called as separate snapshots with no
// spanning lock. A ticket returned here may be claimed by another agent before
// the caller acts on it. Always call Claim() and handle AlreadyClaimedError.
func ReadyTickets(s Store) ([]ticket.Ticket, error) {
	tickets, isClaimed, err := claimedSnapshot(s)
	if err != nil {
		return nil, err
	}
	return graph.ReadyFilterUnclaimed(tickets, isClaimed), nil
}

// ReadyChildren returns the ready, unclaimed children of parentID.
// See ReadyTickets for the non-atomic snapshot caveat.
func ReadyChildren(s Store, parentID string) ([]ticket.Ticket, error) {
	tickets, isClaimed, err := claimedSnapshot(s)
	if err != nil {
		return nil, err
	}
	return graph.FilterReadyChildren(tickets, parentID, isClaimed), nil
}
