// Package tui implements the interactive terminal UI for epos tickets.
package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
	ticketruntime "github.com/php-workx/epos/ticket/runtime"
	"github.com/php-workx/epos/ticket/store"
)

// Group names the ticket list groups shown by the TUI.
type Group string

const (
	GroupReady   Group = "ready"
	GroupBlocked Group = "blocked"
	GroupClaimed Group = "claimed"
	GroupOpen    Group = "open"
	GroupClosed  Group = "closed"
	GroupAll     Group = "all"
)

// Groups is the stable order of TUI groups.
var Groups = []Group{GroupReady, GroupBlocked, GroupClaimed, GroupOpen, GroupClosed, GroupAll}

// DataSource keeps the Bubble Tea model decoupled from ticket storage.
type DataSource interface {
	Snapshot(parent string) (Snapshot, error)
	AddNote(id, text string) error
	Claim(id, owner string) error
	Release(id, owner string) error
	Close(id, reason string) error
	Reopen(id, reason string) error
}

// StoreDataSource implements DataSource using epos-native store and runtime APIs.
type StoreDataSource struct {
	store *store.FileStore
}

// NewStoreDataSource returns a DataSource backed by s.
func NewStoreDataSource(s *store.FileStore) *StoreDataSource {
	return &StoreDataSource{store: s}
}

// TicketRow is the list/detail view model for a ticket.
type TicketRow struct {
	ID             string
	Title          string
	Status         ticket.Status
	Type           string
	Priority       int
	Parent         string
	Deps           []string
	Tags           []string
	Assignee       string
	ReadinessGroup Group
	ClaimOwner     string
	DepCount       int
	ChildCount     int
	ChildSummaries []string
	RuntimeSummary string
	Ticket         ticket.Ticket
	Runtime        *ticket.RuntimeState
}

// Snapshot is a complete, grouped TUI view of the store at a point in time.
type Snapshot struct {
	Groups  map[Group][]TicketRow
	Counts  map[Group]int
	Rows    []TicketRow
	Tickets map[string]ticket.Ticket
	Runtime map[string]*ticket.RuntimeState
}

// RowByID returns the first row with id across all rows.
func (s Snapshot) RowByID(id string) *TicketRow {
	for i := range s.Rows {
		if s.Rows[i].ID == id {
			return &s.Rows[i]
		}
	}
	return nil
}

// Snapshot loads, groups, and annotates tickets for the TUI.
func (s *StoreDataSource) Snapshot(parent string) (Snapshot, error) {
	all, err := s.store.List()
	if err != nil {
		return Snapshot{}, err
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Priority != all[j].Priority {
			return all[i].Priority > all[j].Priority
		}
		return all[i].ID < all[j].ID
	})

	readyIDs := idSet(graph.ReadyFilter(all))
	blockedIDs := idSet(graph.BlockedFilter(all))
	children := childSummaries(all)

	snap := newSnapshot()
	now := time.Now()
	for i := range all {
		tk := &all[i]
		if parent != "" && tk.Parent != parent {
			continue
		}
		state, rerr := ticketruntime.ReadRuntimeState(s.store.Dir, tk.ID)
		if rerr != nil {
			return Snapshot{}, fmt.Errorf("read runtime state %q: %w", tk.ID, rerr)
		}
		claimed := hasActiveClaim(state, now)
		row := rowFromTicket(tk, state, claimed, children[tk.ID])
		switch {
		case claimed:
			row.ReadinessGroup = GroupClaimed
		case readyIDs[tk.ID]:
			row.ReadinessGroup = GroupReady
		case blockedIDs[tk.ID]:
			row.ReadinessGroup = GroupBlocked
		case isClosedStatus(tk.Status):
			row.ReadinessGroup = GroupClosed
		default:
			row.ReadinessGroup = GroupOpen
		}

		snap.Rows = append(snap.Rows, row)
		snap.Tickets[tk.ID] = *tk
		snap.Runtime[tk.ID] = state
		snap.Groups[GroupAll] = append(snap.Groups[GroupAll], row)
		switch {
		case claimed:
			snap.Groups[GroupClaimed] = append(snap.Groups[GroupClaimed], row)
		case isClosedStatus(tk.Status):
			snap.Groups[GroupClosed] = append(snap.Groups[GroupClosed], row)
		default:
			snap.Groups[GroupOpen] = append(snap.Groups[GroupOpen], row)
		}
		if !claimed && readyIDs[tk.ID] {
			snap.Groups[GroupReady] = append(snap.Groups[GroupReady], row)
		}
		if !claimed && blockedIDs[tk.ID] {
			snap.Groups[GroupBlocked] = append(snap.Groups[GroupBlocked], row)
		}
	}

	for _, group := range Groups {
		snap.Counts[group] = len(snap.Groups[group])
	}
	return snap, nil
}

// AddNote appends text to the selected ticket's notes.
func (s *StoreDataSource) AddNote(id, text string) error {
	return s.store.AddNote(id, text)
}

// Claim claims the selected ticket for owner.
func (s *StoreDataSource) Claim(id, owner string) error {
	fullID, err := s.store.ResolveID(id)
	if err != nil {
		return err
	}
	return ticketruntime.ReclaimExpired(s.store.Dir, fullID, owner, "epos-tui", ticket.DefaultLeaseDuration)
}

// Release releases the selected ticket claim for owner.
func (s *StoreDataSource) Release(id, owner string) error {
	fullID, err := s.store.ResolveID(id)
	if err != nil {
		return err
	}
	return ticketruntime.Release(s.store.Dir, fullID, owner, ticket.StatusPending, "")
}

// Close transitions the selected ticket to closed.
func (s *StoreDataSource) Close(id, reason string) error {
	fullID, tk, err := s.resolve(id)
	if err != nil {
		return err
	}
	tk.Status = ticket.StatusClosed
	tk.Present["status"] = true
	if reason != "" {
		tk.StatusReason = reason
		tk.Present["status_reason"] = true
	}
	tk.ID = fullID
	if err := s.store.Update(tk); err != nil {
		return err
	}
	return s.reconcileRuntimeStatus(fullID, ticket.StatusClosed)
}

// Reopen transitions the selected ticket to open.
func (s *StoreDataSource) Reopen(id, reason string) error {
	fullID, tk, err := s.resolve(id)
	if err != nil {
		return err
	}
	tk.Status = ticket.StatusOpen
	tk.Present["status"] = true
	if reason != "" {
		tk.StatusReason = reason
		tk.Present["status_reason"] = true
	}
	tk.ID = fullID
	if err := s.store.Update(tk); err != nil {
		return err
	}
	return s.reconcileRuntimeStatus(fullID, ticket.StatusPending)
}

func (s *StoreDataSource) resolve(id string) (string, *ticket.Ticket, error) {
	fullID, err := s.store.ResolveID(id)
	if err != nil {
		return "", nil, err
	}
	tk, err := s.store.Read(fullID)
	if err != nil {
		return "", nil, err
	}
	if tk.Present == nil {
		tk.Present = make(map[string]bool)
	}
	return fullID, tk, nil
}

func (s *StoreDataSource) reconcileRuntimeStatus(id string, status ticket.Status) error {
	state, err := ticketruntime.ReadRuntimeState(s.store.Dir, id)
	if err != nil {
		return err
	}
	state.TicketID = id
	state.Status = status
	state.Claim = nil
	state.Lease = nil
	state.Heartbeat = nil
	return ticketruntime.WriteRuntimeState(s.store.Dir, state)
}

// FilterRows returns rows matching q across fields useful in the TUI.
func FilterRows(rows []TicketRow, q string) []TicketRow {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return rows
	}
	var out []TicketRow
	for i := range rows {
		row := &rows[i]
		haystack := strings.ToLower(strings.Join([]string{
			row.ID,
			row.Title,
			string(row.Status),
			row.Type,
			strings.Join(row.Tags, " "),
			row.Assignee,
		}, " "))
		if strings.Contains(haystack, q) {
			out = append(out, *row)
		}
	}
	return out
}

func newSnapshot() Snapshot {
	groups := make(map[Group][]TicketRow, len(Groups))
	counts := make(map[Group]int, len(Groups))
	for _, group := range Groups {
		groups[group] = nil
		counts[group] = 0
	}
	return Snapshot{
		Groups:  groups,
		Counts:  counts,
		Tickets: make(map[string]ticket.Ticket),
		Runtime: make(map[string]*ticket.RuntimeState),
	}
}

func rowFromTicket(tk *ticket.Ticket, state *ticket.RuntimeState, claimed bool, children []string) TicketRow {
	row := TicketRow{
		ID:             tk.ID,
		Title:          tk.Title,
		Status:         tk.Status,
		Type:           tk.Type,
		Priority:       tk.Priority,
		Parent:         tk.Parent,
		Deps:           append([]string(nil), tk.Deps...),
		Tags:           append([]string(nil), tk.Tags...),
		Assignee:       tk.Assignee,
		DepCount:       len(tk.Deps),
		ChildCount:     len(children),
		ChildSummaries: append([]string(nil), children...),
		Ticket:         *tk,
		Runtime:        state,
	}
	if claimed {
		row.ClaimOwner = state.Claim.ClaimedBy
		row.RuntimeSummary = "claimed by " + state.Claim.ClaimedBy
		row.RuntimeSummary += " until " + state.Lease.ExpiresAt.Local().Format(time.RFC3339)
	}
	return row
}

func childSummaries(tickets []ticket.Ticket) map[string][]string {
	out := make(map[string][]string)
	for i := range tickets {
		tk := &tickets[i]
		if tk.Parent == "" {
			continue
		}
		out[tk.Parent] = append(out[tk.Parent], fmt.Sprintf("%s  %s  p%d  %s", displayTicketID(tk.ID), tk.Status, tk.Priority, tk.Title))
	}
	return out
}

func displayTicketID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return "#" + id[len(id)-4:]
}

func hasActiveClaim(state *ticket.RuntimeState, now time.Time) bool {
	return state != nil &&
		state.Claim != nil &&
		state.Lease != nil &&
		!state.Lease.ExpiresAt.IsZero() &&
		now.Before(state.Lease.ExpiresAt)
}

func idSet(rows []ticket.Ticket) map[string]bool {
	set := make(map[string]bool, len(rows))
	for i := range rows {
		set[rows[i].ID] = true
	}
	return set
}

func isClosedStatus(status ticket.Status) bool {
	return status == ticket.StatusClosed || status == ticket.StatusDone || status == ticket.StatusFailed
}
