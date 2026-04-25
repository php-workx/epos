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

// ticketPath returns the file path for the given ticket ID.
func (s *FileStore) ticketPath(id string) string {
	return filepath.Join(s.Dir, TicketsDir, id+TicketExt)
}

// Create writes a new ticket file. It returns *ticket.IDCollisionError if the
// file already exists.
func (s *FileStore) Create(t *ticket.Ticket) error {
	if t.ID == "" {
		return &ticket.ValidationError{Field: "id", Message: "required"}
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
	return os.WriteFile(path, data, 0o644)
}

// Read loads a ticket by its full ID. It returns *ticket.TicketNotFoundError
// if no file matches.
func (s *FileStore) Read(id string) (*ticket.Ticket, error) {
	path := s.ticketPath(id)
	data, err := os.ReadFile(path)
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
	return t, nil
}

// Update writes the ticket file, overwriting any existing content.
func (s *FileStore) Update(t *ticket.Ticket) error {
	if t.ID == "" {
		return &ticket.ValidationError{Field: "id", Message: "required"}
	}
	path := s.ticketPath(t.ID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &ticket.TicketNotFoundError{ID: t.ID}
	}
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	t.Present["updated_at"] = true
	data, err := markdown.MarshalTicket(t)
	if err != nil {
		return fmt.Errorf("marshal ticket %q: %w", t.ID, err)
	}
	return os.WriteFile(path, data, 0o644)
}

// Delete removes the ticket file for the given ID.
func (s *FileStore) Delete(id string) error {
	path := s.ticketPath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return &ticket.TicketNotFoundError{ID: id}
		}
		return fmt.Errorf("delete ticket %q: %w", id, err)
	}
	return nil
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
		data, err := os.ReadFile(filepath.Join(td, entry.Name()))
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
