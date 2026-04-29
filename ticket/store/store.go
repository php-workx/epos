// Package store implements the FileStore for ticket CRUD operations and
// directory-level child discovery.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
	"github.com/php-workx/epos/ticket/markdown"
)

const (
	// TicketsDir is the subdirectory within the store root that holds ticket files.
	TicketsDir = ".tickets"

	// TicketExt is the file extension for ticket markdown files.
	TicketExt = ".md"
)

// FileStore provides CRUD operations for tickets stored as Markdown files
// in a directory on the local filesystem. The expected layout is:
//
//	<dir>/.tickets/<ticket-id>.md
//
// All operations are scoped to the directory provided at construction time.
type FileStore struct {
	// Dir is the repository root containing the .tickets subdirectory.
	Dir string
}

// NewFileStore creates a FileStore rooted at dir. The .tickets subdirectory
// is created if it does not exist.
func NewFileStore(dir string) (*FileStore, error) {
	td := filepath.Join(dir, TicketsDir)
	if err := os.MkdirAll(td, 0o750); err != nil {
		return nil, fmt.Errorf("create tickets dir: %w", err)
	}
	return &FileStore{Dir: dir}, nil
}

// ticketPath returns the file path for a validated ticket ID.
func (s *FileStore) ticketPath(id string) string {
	return filepath.Join(s.Dir, TicketsDir, id+TicketExt)
}

func (s *FileStore) graphLockPath() string {
	return filepath.Join(s.Dir, TicketsDir, ".graph")
}

// Create writes a new ticket file. It returns *ticket.IDCollisionError if the
// file already exists.
func (s *FileStore) Create(t *ticket.Ticket) error {
	if err := ticket.ValidateID(t.ID); err != nil {
		return err
	}
	if t.Present == nil {
		t.Present = make(map[string]bool)
	}
	path := s.ticketPath(t.ID)
	if _, err := os.Stat(path); err == nil {
		return &ticket.IDCollisionError{ID: t.ID}
	}
	// Stamp creation time if not already set.
	if t.Created == "" {
		t.Created = time.Now().UTC().Format(time.RFC3339)
	}
	t.Present["created"] = true
	// Ensure the ticket has a valid extended_status default.
	if t.ExtendedStatus == "" {
		t.ExtendedStatus = "open"
	}
	t.Present["extended_status"] = true
	if t.Status == "" {
		t.Status = ticket.StatusOpen
	}
	t.Present["status"] = true
	t.Present["id"] = true
	data, err := markdown.MarshalTicket(t)
	if err != nil {
		return fmt.Errorf("marshal ticket %q: %w", t.ID, err)
	}
	return withLock(path, func() error {
		// Re-check existence under the lock to close the race between Stat and write.
		if _, statErr := os.Stat(path); statErr == nil {
			return &ticket.IDCollisionError{ID: t.ID}
		}
		return atomicWrite(path, data)
	})
}

// Read loads a ticket by its full ID. It returns *ticket.TicketNotFoundError
// if no file matches.
func (s *FileStore) Read(id string) (*ticket.Ticket, error) {
	if err := ticket.ValidateID(id); err != nil {
		return nil, err
	}
	path := s.ticketPath(id)
	data, err := os.ReadFile(path) //nolint:gosec // G304: path uses a validated ticket ID
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ticket.TicketNotFoundError{ID: id}
		}
		return nil, fmt.Errorf("read ticket %q: %w", id, err)
	}
	t, err := markdown.UnmarshalTicket(data)
	if err != nil {
		return nil, &ticket.CorruptYAMLError{Path: path, Cause: err}
	}
	// Backfill ID from filename — tk-compatible files may omit the id field.
	if t.ID == "" {
		if t.Present == nil {
			t.Present = make(map[string]bool)
		}
		t.ID = id
		t.Present["id"] = true
	}
	return t, nil
}

// Update writes the ticket file, preserving any existing Markdown body content.
func (s *FileStore) Update(t *ticket.Ticket) error {
	if err := ticket.ValidateID(t.ID); err != nil {
		return err
	}
	if t.Present == nil {
		t.Present = make(map[string]bool)
	}
	path := s.ticketPath(t.ID)
	return withLock(path, func() error {
		existing, err := os.ReadFile(path) //nolint:gosec // G304: path uses a validated ticket ID
		if err != nil {
			if os.IsNotExist(err) {
				return &ticket.TicketNotFoundError{ID: t.ID}
			}
			return fmt.Errorf("read ticket %q: %w", t.ID, err)
		}
		t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		t.Present["updated_at"] = true
		data, err := markdown.UpdateFrontmatter(existing, t)
		if err != nil {
			return fmt.Errorf("update ticket %q: %w", t.ID, err)
		}
		return atomicWrite(path, data)
	})
}

// Delete removes the ticket file for the given ID.
func (s *FileStore) Delete(id string) error {
	if err := ticket.ValidateID(id); err != nil {
		return err
	}
	path := s.ticketPath(id)
	return withLock(path, func() error {
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return &ticket.TicketNotFoundError{ID: id}
			}
			return fmt.Errorf("delete ticket %q: %w", id, err)
		}
		return nil
	})
}

// List returns all tickets in the store, sorted by ID.
func (s *FileStore) List() ([]ticket.Ticket, error) {
	td := filepath.Join(s.Dir, TicketsDir)
	entries, err := os.ReadDir(td)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	var results []ticket.Ticket
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != TicketExt {
			continue
		}
		fileID := strings.TrimSuffix(entry.Name(), TicketExt)
		data, err := os.ReadFile(filepath.Join(td, entry.Name())) //nolint:gosec // G304: entry.Name() is a directory entry, cannot contain path separators
		if err != nil {
			continue
		}
		t, err := markdown.UnmarshalTicket(data)
		if err != nil {
			continue // skip corrupt files during list
		}
		// Ensure ID is populated from the filename (canonical source of truth)
		// even if the YAML frontmatter omits the id field.
		if t.ID == "" {
			t.ID = fileID
		}
		results = append(results, *t)
	}
	return results, nil
}

// ResolveID resolves a partial ticket ID to a full ID. If the partial matches
// exactly one ticket, it returns the full ID. If it matches none, it returns
// *ticket.TicketNotFoundError. If it matches more than one, it returns
// *ticket.AmbiguousIDError.
func (s *FileStore) ResolveID(partial string) (string, error) {
	td := filepath.Join(s.Dir, TicketsDir)
	entries, err := os.ReadDir(td)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &ticket.TicketNotFoundError{ID: partial}
		}
		return "", fmt.Errorf("resolve ID %q: %w", partial, err)
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != TicketExt {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), TicketExt)
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

// AddNote appends a timestamped note to the ticket's ## Notes Markdown section.
// id may be a full or partial ticket ID. The read-modify-write is performed
// under an exclusive lock and committed atomically.
func (s *FileStore) AddNote(id, text string) error {
	fullID, err := s.ResolveID(id)
	if err != nil {
		return err
	}
	path := s.ticketPath(fullID)
	return withLock(path, func() error {
		existing, rerr := os.ReadFile(path) //nolint:gosec // G304: path uses a resolved ticket filename ID
		if rerr != nil {
			if os.IsNotExist(rerr) {
				return &ticket.TicketNotFoundError{ID: fullID}
			}
			return fmt.Errorf("read ticket %q: %w", fullID, rerr)
		}
		t, uerr := markdown.UnmarshalTicket(existing)
		if uerr != nil {
			return &ticket.CorruptYAMLError{Path: path, Cause: uerr}
		}
		if t.ID == "" {
			t.ID = fullID
			t.Present["id"] = true
		}
		formattedNote := markdown.FormatNote(text)
		updated := markdown.UpdateBody(existing, func(body string) string {
			return markdown.AddFormattedNote(body, formattedNote)
		})
		t.Notes = append(t.Notes, formattedNote)
		t.Present["notes"] = true
		updated, uerr = markdown.UpdateFrontmatter(updated, t)
		if uerr != nil {
			return fmt.Errorf("update ticket %q notes: %w", fullID, uerr)
		}
		return atomicWrite(path, updated)
	})
}

// AddDep adds depID to the ticket's Deps list. Both id and depID may be
// partial. AddDep returns *ticket.CycleDetectedError if the addition would
// create a dependency cycle. If depID is already present the operation is a
// no-op.
func (s *FileStore) AddDep(id, depID string) error {
	fullID, err := s.ResolveID(id)
	if err != nil {
		return err
	}
	fullDep, err := s.ResolveID(depID)
	if err != nil {
		return err
	}
	if fullID == fullDep {
		return &ticket.CycleDetectedError{Cycle: []string{fullID, fullDep}}
	}
	path := s.ticketPath(fullID)
	return withLocks([]string{s.graphLockPath(), path}, func() error {
		t, rerr := s.Read(fullID)
		if rerr != nil {
			return rerr
		}
		for _, d := range t.Deps {
			if d == fullDep {
				return nil // already present
			}
		}
		t.Deps = append(t.Deps, fullDep)
		if t.Present == nil {
			t.Present = make(map[string]bool)
		}
		t.Present["deps"] = true

		all, lerr := s.List()
		if lerr != nil {
			return lerr
		}
		for i := range all {
			if all[i].ID == fullID {
				all[i].Deps = t.Deps
				break
			}
		}
		if cycles := graph.DetectCycles(all); len(cycles) > 0 {
			return &ticket.CycleDetectedError{Cycle: cycles[0]}
		}
		return s.writeFrontmatterUnderLock(path, t)
	})
}

// RemoveDep removes depID from the ticket's Deps list. id and depID may be
// partial. Removing an absent dep is a no-op.
func (s *FileStore) RemoveDep(id, depID string) error {
	fullID, err := s.ResolveID(id)
	if err != nil {
		return err
	}
	// depID need not exist as a ticket; we just remove the literal string match.
	// Try to resolve to canonical form when possible, but tolerate not-found.
	target := depID
	if resolved, rerr := s.ResolveID(depID); rerr == nil {
		target = resolved
	}
	path := s.ticketPath(fullID)
	return withLock(path, func() error {
		t, rerr := s.Read(fullID)
		if rerr != nil {
			return rerr
		}
		filtered := t.Deps[:0]
		removed := false
		for _, d := range t.Deps {
			if d == target {
				removed = true
				continue
			}
			filtered = append(filtered, d)
		}
		if !removed {
			return nil
		}
		t.Deps = filtered
		if t.Present == nil {
			t.Present = make(map[string]bool)
		}
		t.Present["deps"] = true
		return s.writeFrontmatterUnderLock(path, t)
	})
}

// Link creates a symmetric link between id and targetID: each ticket's Links
// slice gains the other's ID. Both files are updated atomically under a
// combined lock acquired in deterministic path order.
func (s *FileStore) Link(id, targetID string) error {
	return s.linkOp(id, targetID, true)
}

// Unlink removes the symmetric link between id and targetID. Removing an
// absent link is a no-op on the affected side.
func (s *FileStore) Unlink(id, targetID string) error {
	return s.linkOp(id, targetID, false)
}

func (s *FileStore) linkOp(id, targetID string, add bool) error {
	fullA, err := s.ResolveID(id)
	if err != nil {
		return err
	}
	fullB, err := s.ResolveID(targetID)
	if err != nil {
		return err
	}
	if fullA == fullB {
		return &ticket.ValidationError{Field: "links", Message: "cannot link a ticket to itself"}
	}
	pathA := s.ticketPath(fullA)
	pathB := s.ticketPath(fullB)
	return withLocks([]string{pathA, pathB}, func() error {
		ta, rerr := s.Read(fullA)
		if rerr != nil {
			return rerr
		}
		tb, rerr := s.Read(fullB)
		if rerr != nil {
			return rerr
		}
		if add {
			ta.Links = appendUnique(ta.Links, fullB)
			tb.Links = appendUnique(tb.Links, fullA)
		} else {
			ta.Links = removeFirst(ta.Links, fullB)
			tb.Links = removeFirst(tb.Links, fullA)
		}
		markPresent(ta, "links")
		markPresent(tb, "links")
		if werr := s.writeFrontmatterUnderLock(pathA, ta); werr != nil {
			return werr
		}
		return s.writeFrontmatterUnderLock(pathB, tb)
	})
}

// writeFrontmatterUnderLock rewrites the ticket file at path with t's frontmatter,
// preserving the existing Markdown body. The caller is responsible for holding
// the file lock.
func (s *FileStore) writeFrontmatterUnderLock(path string, t *ticket.Ticket) error {
	existing, err := os.ReadFile(path) //nolint:gosec // G304: path uses a resolved or validated ticket ID
	if err != nil {
		if os.IsNotExist(err) {
			return &ticket.TicketNotFoundError{ID: t.ID}
		}
		return fmt.Errorf("read ticket %q: %w", t.ID, err)
	}
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if t.Present == nil {
		t.Present = make(map[string]bool)
	}
	t.Present["updated_at"] = true
	data, err := markdown.UpdateFrontmatter(existing, t)
	if err != nil {
		return fmt.Errorf("update ticket %q: %w", t.ID, err)
	}
	return atomicWrite(path, data)
}

func appendUnique(slice []string, v string) []string {
	for _, s := range slice {
		if s == v {
			return slice
		}
	}
	return append(slice, v)
}

func removeFirst(slice []string, v string) []string {
	for i, s := range slice {
		if s == v {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func markPresent(t *ticket.Ticket, key string) {
	if t.Present == nil {
		t.Present = make(map[string]bool)
	}
	t.Present[key] = true
}

// ListAllChildren returns all tickets whose Parent field equals parentID.
func (s *FileStore) ListAllChildren(parentID string) ([]ticket.Ticket, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	return graph.FilterChildren(all, parentID), nil
}

// FilterReadyChildren returns children of parentID that are ready to work,
// excluding tickets whose IDs appear in claimed.
func (s *FileStore) FilterReadyChildren(parentID string, isClaimed func(string) bool) ([]ticket.Ticket, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	return graph.FilterReadyChildren(all, parentID, isClaimed), nil
}
