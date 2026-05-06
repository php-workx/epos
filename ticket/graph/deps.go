package graph

import (
	"sort"
	"strings"

	"github.com/php-workx/epos/ticket"
)

// closedStatuses are the terminal ticket statuses under which a dependency is
// considered fulfilled (the ticket is done and will not change again).
var closedStatuses = map[ticket.Status]bool{
	ticket.StatusClosed: true,
	ticket.StatusDone:   true,
	ticket.StatusFailed: true,
}

// readyStatuses are the statuses under which a ticket is eligible to be picked up
// but has not yet been claimed or started.
var readyStatuses = map[ticket.Status]bool{
	ticket.StatusOpen:          true,
	ticket.StatusPending:       true,
	ticket.StatusRepairPending: true,
}

// indexByID builds a lookup map from ticket ID to Ticket value.
func indexByID(all []ticket.Ticket) map[string]ticket.Ticket {
	m := make(map[string]ticket.Ticket, len(all))
	for i := range all {
		m[all[i].ID] = all[i]
	}
	return m
}

// allDepsClosed reports whether every dep ID in deps is present in byID with a
// closed status (closed, done, or failed).  Deps absent from byID are treated
// as not closed (conservative assumption: unknown → not done).
func allDepsClosed(deps []string, byID map[string]ticket.Ticket) bool {
	for _, dep := range deps {
		t, ok := byID[dep]
		if !ok || !closedStatuses[t.Status] {
			return false
		}
	}
	return true
}

func hasOpenDeps(t *ticket.Ticket, byID map[string]ticket.Ticket) bool {
	return len(t.Deps) > 0 && !allDepsClosed(t.Deps, byID)
}

func parentBlocksWork(t *ticket.Ticket, byID map[string]ticket.Ticket) bool {
	seen := map[string]bool{}
	for parentID := t.Parent; parentID != ""; {
		if seen[parentID] {
			return true
		}
		seen[parentID] = true
		parent, ok := byID[parentID]
		if !ok {
			return false
		}
		if hasOpenDeps(&parent, byID) {
			return true
		}
		parentID = parent.Parent
	}
	return false
}

// sortByPriorityThenID sorts a ticket slice by priority descending, then ID
// ascending.  The sort is applied in-place.
func sortByPriorityThenID(tickets []ticket.Ticket) {
	sort.Slice(tickets, func(i, j int) bool {
		if tickets[i].Priority != tickets[j].Priority {
			return tickets[i].Priority > tickets[j].Priority
		}
		return tickets[i].ID < tickets[j].ID
	})
}

// ReadyFilter returns tickets from all that are ready to be worked:
//   - Status is open, pending, or repair_pending.
//   - All dep IDs resolve to closed tickets (status closed, done, or failed).
//     Deps absent from all are treated as not closed.
//   - No parent ticket in all is blocked by open dependencies.
//
// The result is sorted by priority descending, then ID ascending.
//
// ReadyFilter is sidecar-blind: tickets with an active claim still appear here.
// Use ReadyFilterUnclaimed when callers need to honour claim state.
func ReadyFilter(all []ticket.Ticket) []ticket.Ticket {
	return ReadyFilterUnclaimed(all, nil)
}

// ReadyFilterUnclaimed returns ready tickets with an additional claim filter.
// When isClaimed returns true for a ticket ID, that ticket is excluded from
// the result. Pass nil to disable claim filtering (equivalent to ReadyFilter).
//
// This is the function the CLI's "epos ready" command should call so that
// concurrently claimed tickets do not surface to other agents.
func ReadyFilterUnclaimed(all []ticket.Ticket, isClaimed func(string) bool) []ticket.Ticket {
	byID := indexByID(all)
	var result []ticket.Ticket
	for i := range all {
		if !readyStatuses[all[i].Status] {
			continue
		}
		if !allDepsClosed(all[i].Deps, byID) {
			continue
		}
		if parentBlocksWork(&all[i], byID) {
			continue
		}
		if isClaimed != nil && isClaimed(all[i].ID) {
			continue
		}
		result = append(result, all[i])
	}
	sortByPriorityThenID(result)
	return result
}

// BlockedFilter returns tickets from all that are blocked:
//   - Status is open, pending, or repair_pending.
//   - Has at least one dep that is not in a closed status (closed, done, or failed), or
//   - Has a parent ticket in all that is blocked by open dependencies.
//
// The result is sorted by priority descending, then ID ascending.
func BlockedFilter(all []ticket.Ticket) []ticket.Ticket {
	byID := indexByID(all)
	var result []ticket.Ticket
	for i := range all {
		if !readyStatuses[all[i].Status] {
			continue
		}
		if !hasOpenDeps(&all[i], byID) && !parentBlocksWork(&all[i], byID) {
			continue
		}
		result = append(result, all[i])
	}
	sortByPriorityThenID(result)
	return result
}

// DetectCycles returns any dependency cycles present in all as slices of ticket
// IDs.  Each returned slice contains the IDs forming one cycle in dependency
// order starting from the lexicographically smallest ID (for determinism).
// DetectCycles uses depth-first search; each unique cycle is reported once.
// Deps that reference ticket IDs absent from all are ignored.
func DetectCycles(all []ticket.Ticket) [][]string {
	byID := indexByID(all)

	const (
		white = 0 // unvisited
		gray  = 1 // on the current DFS stack
		black = 2 // fully explored
	)

	color := make(map[string]int, len(all))
	parent := make(map[string]string, len(all))
	seen := make(map[string]bool)
	var cycles [][]string

	var dfs func(id string)
	dfs = func(id string) {
		color[id] = gray
		t, ok := byID[id]
		if !ok {
			color[id] = black
			return
		}
		for _, dep := range t.Deps {
			if _, inGraph := byID[dep]; !inGraph {
				continue // dep not in the ticket set; skip
			}
			switch color[dep] {
			case white:
				parent[dep] = id
				dfs(dep)
			case gray:
				// Back edge: dep is an ancestor on the current DFS stack.
				cycle := traceCycle(parent, dep, id)
				key := cycleKey(cycle)
				if !seen[key] {
					seen[key] = true
					cycles = append(cycles, cycle)
				}
			}
			// black: dep already fully explored, no cycle through this edge.
		}
		color[id] = black
	}

	for i := range all {
		if color[all[i].ID] == white {
			dfs(all[i].ID)
		}
	}
	return cycles
}

// traceCycle reconstructs a cycle from the DFS parent map.
// start is the gray (ancestor) node that was re-encountered; end is the node
// at which the back edge was detected.  Returns the cycle in dependency order:
// [start, ..., end].
func traceCycle(parent map[string]string, start, end string) []string {
	var path []string
	for cur := end; cur != start; {
		path = append(path, cur)
		next, ok := parent[cur]
		if !ok {
			break // broken chain (should not happen in a correct DFS)
		}
		cur = next
	}
	path = append(path, start)
	// Reverse to get start→...→end order.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// cycleKey returns a canonical deduplication key for a cycle by rotating it to
// begin with the lexicographically smallest ID, then joining with commas.
func cycleKey(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	minIdx := 0
	for i, id := range cycle {
		if id < cycle[minIdx] {
			minIdx = i
		}
	}
	rotated := make([]string, len(cycle))
	n := len(cycle)
	for i, id := range cycle {
		rotated[(i-minIdx+n)%n] = id
	}
	return strings.Join(rotated, ",")
}

// FilterChildren returns tickets from all whose Parent field equals parentID.
// The result preserves the input order.
func FilterChildren(all []ticket.Ticket, parentID string) []ticket.Ticket {
	var result []ticket.Ticket
	for i := range all {
		if all[i].Parent == parentID {
			result = append(result, all[i])
		}
	}
	return result
}

// FilterReadyChildren returns tickets from all that satisfy all of:
//   - t.Parent == parentID
//   - Status is open, pending, or repair_pending
//   - All deps are closed (closed, done, or failed); absent deps treated as not closed
//   - No parent ticket in all is blocked by open dependencies
//   - isClaimed(t.ID) returns false
//
// Pass nil for isClaimed to skip claim filtering.
// The result is sorted by priority descending, then ID ascending.
func FilterReadyChildren(all []ticket.Ticket, parentID string, isClaimed func(string) bool) []ticket.Ticket {
	byID := indexByID(all)
	var result []ticket.Ticket
	for i := range all {
		if all[i].Parent != parentID {
			continue
		}
		if !readyStatuses[all[i].Status] {
			continue
		}
		if !allDepsClosed(all[i].Deps, byID) {
			continue
		}
		if parentBlocksWork(&all[i], byID) {
			continue
		}
		if isClaimed != nil && isClaimed(all[i].ID) {
			continue
		}
		result = append(result, all[i])
	}
	sortByPriorityThenID(result)
	return result
}
