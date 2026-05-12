package store

import (
	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
)

// ReadyTickets returns all tickets that are ready to be worked: no open
// blocking dependencies and no active claim. Callers that want to include
// claimed tickets should use s.List and graph.ReadyFilterUnclaimed directly.
func ReadyTickets(s Store) ([]ticket.Ticket, error) {
	tickets, err := s.List()
	if err != nil {
		return nil, err
	}
	claimed, err := s.ActiveClaimSet()
	if err != nil {
		return nil, err
	}
	isClaimed := func(id string) bool { return claimed[id] }
	return graph.ReadyFilterUnclaimed(tickets, isClaimed), nil
}

// ReadyChildren returns the ready, unclaimed children of parentID.
func ReadyChildren(s Store, parentID string) ([]ticket.Ticket, error) {
	tickets, err := s.List()
	if err != nil {
		return nil, err
	}
	claimed, err := s.ActiveClaimSet()
	if err != nil {
		return nil, err
	}
	isClaimed := func(id string) bool { return claimed[id] }
	return graph.FilterReadyChildren(tickets, parentID, isClaimed), nil
}
