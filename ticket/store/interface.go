package store

import "github.com/php-workx/epos/ticket"

// Store is the read/write interface for a ticket repository.
type Store interface {
	// CRUD
	Create(t *ticket.Ticket) error
	Read(id string) (*ticket.Ticket, error)
	Update(t *ticket.Ticket) error
	Delete(id string) error

	// Discovery
	List() ([]ticket.Ticket, error)
	ResolveID(partial string) (string, error)
	ListAllChildren(parentID string) ([]ticket.Ticket, error)
	FilterReadyChildren(parentID string, isClaimed func(string) bool) ([]ticket.Ticket, error)

	// Graph mutations
	AddNote(id, text string) error
	AddDep(id, depID string) error
	RemoveDep(id, depID string) error
	Link(id, targetID string) error
	Unlink(id, targetID string) error

	// Claims integration (see Epic 3 for seam deepening)
	ActiveClaimSet() (map[string]bool, error)
	ListReady() ([]ticket.Ticket, error)
}

// Compile-time assertion: FileStore must satisfy Store.
var _ Store = (*FileStore)(nil)
