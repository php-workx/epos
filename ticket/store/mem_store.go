package store

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
	"github.com/php-workx/epos/ticket/markdown"
)

// MemStore is an in-memory implementation of Store. It is safe for concurrent
// use and is intended for unit tests and ephemeral scenarios that do not need
// filesystem persistence.
type MemStore struct {
	mu      sync.Mutex
	tickets map[string]*ticket.Ticket
}

// NewMemStore returns an empty, ready-to-use MemStore.
func NewMemStore() *MemStore {
	return &MemStore{tickets: make(map[string]*ticket.Ticket)}
}

// cloneTicket returns a deep copy of t, duplicating the Present and Extra maps
// so that mutations to the returned value do not affect the stored copy.
func cloneTicket(t *ticket.Ticket) *ticket.Ticket {
	cp := *t
	if t.Present != nil {
		cp.Present = make(map[string]bool, len(t.Present))
		for k, v := range t.Present {
			cp.Present[k] = v
		}
	}
	if t.Extra != nil {
		cp.Extra = make(map[string]any, len(t.Extra))
		for k, v := range t.Extra {
			cp.Extra[k] = v
		}
	}
	return &cp
}

// Create stores a new ticket. It returns *ticket.IDCollisionError if the ID
// already exists.
func (m *MemStore) Create(t *ticket.Ticket) error {
	if t == nil {
		return &ticket.ValidationError{Field: "ticket", Message: "must not be nil"}
	}
	if err := ticket.ValidateID(t.ID); err != nil {
		return err
	}
	if t.Present == nil {
		t.Present = make(map[string]bool)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.tickets[t.ID]; exists {
		return &ticket.IDCollisionError{ID: t.ID}
	}
	cp := cloneTicket(t)
	if cp.Created == "" {
		cp.Created = time.Now().UTC().Format(time.RFC3339)
	}
	cp.Present["created"] = true
	if cp.ExtendedStatus == "" {
		cp.ExtendedStatus = "open"
	}
	cp.Present["extended_status"] = true
	if cp.Status == "" {
		cp.Status = ticket.StatusOpen
	}
	cp.Present["status"] = true
	cp.Present["id"] = true
	m.tickets[cp.ID] = cp
	return nil
}

// Read returns a copy of the ticket with the given full ID, or
// *ticket.TicketNotFoundError if absent.
func (m *MemStore) Read(id string) (*ticket.Ticket, error) {
	if err := ticket.ValidateID(id); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tickets[id]
	if !ok {
		return nil, &ticket.TicketNotFoundError{ID: id}
	}
	return cloneTicket(t), nil
}

// Update replaces the stored ticket. It returns *ticket.TicketNotFoundError if
// the ticket does not exist.
func (m *MemStore) Update(t *ticket.Ticket) error {
	if t == nil {
		return &ticket.ValidationError{Field: "ticket", Message: "must not be nil"}
	}
	if err := ticket.ValidateID(t.ID); err != nil {
		return err
	}
	if t.Present == nil {
		t.Present = make(map[string]bool)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tickets[t.ID]; !ok {
		return &ticket.TicketNotFoundError{ID: t.ID}
	}
	cp := cloneTicket(t)
	cp.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	cp.Present["updated_at"] = true
	m.tickets[cp.ID] = cp
	return nil
}

// Delete removes the ticket with the given ID. Returns *ticket.TicketNotFoundError
// if absent.
func (m *MemStore) Delete(id string) error {
	if err := ticket.ValidateID(id); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tickets[id]; !ok {
		return &ticket.TicketNotFoundError{ID: id}
	}
	delete(m.tickets, id)
	return nil
}

// List returns all tickets sorted by ID.
func (m *MemStore) List() ([]ticket.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]ticket.Ticket, 0, len(m.tickets))
	for _, t := range m.tickets {
		result = append(result, *cloneTicket(t))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// ResolveID resolves a partial ticket ID to a full ID using substring matching.
// Exact match is tried first. Returns *ticket.TicketNotFoundError for zero
// matches and *ticket.AmbiguousIDError for more than one match.
func (m *MemStore) ResolveID(partial string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Exact match wins immediately.
	if _, ok := m.tickets[partial]; ok {
		return partial, nil
	}
	var matches []string
	for id := range m.tickets {
		if strings.Contains(id, partial) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 0:
		return "", &ticket.TicketNotFoundError{ID: partial}
	case 1:
		return matches[0], nil
	default:
		return "", &ticket.AmbiguousIDError{Partial: partial, Matches: matches}
	}
}

// AddNote appends a timestamped note to the ticket identified by id (full or
// partial).
func (m *MemStore) AddNote(id, text string) error {
	fullID, err := m.ResolveID(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tickets[fullID]
	if !ok {
		return &ticket.TicketNotFoundError{ID: fullID}
	}
	cp := cloneTicket(t)
	formattedNote := markdown.FormatNote(text)
	cp.Notes = append(cp.Notes, formattedNote)
	if cp.Present == nil {
		cp.Present = make(map[string]bool)
	}
	cp.Present["notes"] = true
	m.tickets[fullID] = cp
	return nil
}

// reachable reports whether target is reachable from src via Deps edges.
// The caller must hold mu.
func (m *MemStore) reachable(src, target string) bool {
	visited := make(map[string]bool)
	queue := []string{src}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == target {
			return true
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		if t, ok := m.tickets[cur]; ok {
			queue = append(queue, t.Deps...)
		}
	}
	return false
}

// AddDep adds depID to the ticket's Deps list. Returns *ticket.CycleDetectedError
// if the addition would create a dependency cycle. If depID is already present
// the operation is a no-op.
func (m *MemStore) AddDep(id, depID string) error {
	fullID, err := m.ResolveID(id)
	if err != nil {
		return err
	}
	fullDep, err := m.ResolveID(depID)
	if err != nil {
		return err
	}
	if fullID == fullDep {
		return &ticket.CycleDetectedError{Cycle: []string{fullID, fullDep}}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tickets[fullID]
	if !ok {
		return &ticket.TicketNotFoundError{ID: fullID}
	}
	for _, d := range t.Deps {
		if d == fullDep {
			return nil // already present
		}
	}
	// Check whether adding fullID → fullDep would create a cycle.
	// A cycle exists if fullDep can already reach fullID (the reverse path).
	if m.reachable(fullDep, fullID) {
		return &ticket.CycleDetectedError{Cycle: []string{fullID, fullDep}}
	}
	cp := cloneTicket(t)
	cp.Deps = append(cp.Deps, fullDep)
	if cp.Present == nil {
		cp.Present = make(map[string]bool)
	}
	cp.Present["deps"] = true
	m.tickets[fullID] = cp
	return nil
}

// RemoveDep removes depID from the ticket's Deps list. Removing an absent dep
// is a no-op.
func (m *MemStore) RemoveDep(id, depID string) error {
	fullID, err := m.ResolveID(id)
	if err != nil {
		return err
	}
	// Tolerate depID not existing as a ticket.
	target := depID
	if resolved, rerr := m.ResolveID(depID); rerr == nil {
		target = resolved
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tickets[fullID]
	if !ok {
		return &ticket.TicketNotFoundError{ID: fullID}
	}
	cp := cloneTicket(t)
	filtered := cp.Deps[:0]
	removed := false
	for _, d := range cp.Deps {
		if d == target {
			removed = true
			continue
		}
		filtered = append(filtered, d)
	}
	if !removed {
		return nil
	}
	cp.Deps = filtered
	if cp.Present == nil {
		cp.Present = make(map[string]bool)
	}
	cp.Present["deps"] = true
	m.tickets[fullID] = cp
	return nil
}

// addLink appends targetID to the Links slice of the ticket identified by id.
// It is a no-op if targetID is already present. The caller must NOT hold mu.
func (m *MemStore) addLink(id, targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tickets[id]
	if !ok {
		return &ticket.TicketNotFoundError{ID: id}
	}
	cp := cloneTicket(t)
	cp.Links = appendUnique(cp.Links, targetID)
	if cp.Present == nil {
		cp.Present = make(map[string]bool)
	}
	cp.Present["links"] = true
	m.tickets[id] = cp
	return nil
}

// removeLink removes targetID from the Links slice of the ticket identified by id.
// The caller must NOT hold mu.
func (m *MemStore) removeLink(id, targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tickets[id]
	if !ok {
		return &ticket.TicketNotFoundError{ID: id}
	}
	cp := cloneTicket(t)
	cp.Links = removeFirst(cp.Links, targetID)
	if cp.Present == nil {
		cp.Present = make(map[string]bool)
	}
	cp.Present["links"] = true
	m.tickets[id] = cp
	return nil
}

// Link creates a symmetric link between id and targetID.
func (m *MemStore) Link(id, targetID string) error {
	fullA, err := m.ResolveID(id)
	if err != nil {
		return err
	}
	fullB, err := m.ResolveID(targetID)
	if err != nil {
		return err
	}
	if fullA == fullB {
		return &ticket.ValidationError{Field: "links", Message: "cannot link a ticket to itself"}
	}
	if err := m.addLink(fullA, fullB); err != nil {
		return err
	}
	return m.addLink(fullB, fullA)
}

// Unlink removes the symmetric link between id and targetID.
func (m *MemStore) Unlink(id, targetID string) error {
	fullA, err := m.ResolveID(id)
	if err != nil {
		return err
	}
	fullB, err := m.ResolveID(targetID)
	if err != nil {
		return err
	}
	if err := m.removeLink(fullA, fullB); err != nil {
		return err
	}
	return m.removeLink(fullB, fullA)
}

// ListAllChildren returns all tickets whose Parent field equals parentID.
func (m *MemStore) ListAllChildren(parentID string) ([]ticket.Ticket, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}
	return graph.FilterChildren(all, parentID), nil
}

// FilterReadyChildren returns children of parentID that are ready and unclaimed.
func (m *MemStore) FilterReadyChildren(parentID string, isClaimed func(string) bool) ([]ticket.Ticket, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}
	return graph.FilterReadyChildren(all, parentID, isClaimed), nil
}

// ActiveClaimSet returns an empty map — MemStore has no sidecar files.
func (m *MemStore) ActiveClaimSet() (map[string]bool, error) {
	return map[string]bool{}, nil
}

// ListReady returns tickets that are ready to be worked, excluding claimed ones.
func (m *MemStore) ListReady() ([]ticket.Ticket, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}
	claimed, err := m.ActiveClaimSet()
	if err != nil {
		return nil, err
	}
	isClaimed := func(id string) bool { return claimed[id] }
	return graph.ReadyFilterUnclaimed(all, isClaimed), nil
}

// Compile-time assertion: MemStore must satisfy Store.
var _ Store = (*MemStore)(nil)
