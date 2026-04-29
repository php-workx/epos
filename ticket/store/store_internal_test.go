package store

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/php-workx/epos/ticket"
)

func TestWriteFrontmattersUnderLockRollsBackCommittedWrite(t *testing.T) {
	s, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	a := newInternalTestTicket("rollback-a")
	b := newInternalTestTicket("rollback-b")
	if err := s.Create(a); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	if err := s.Create(b); err != nil {
		t.Fatalf("Create B: %v", err)
	}
	pathA := s.ticketPath(a.ID)
	pathB := s.ticketPath(b.ID)

	originalA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatalf("ReadFile A: %v", err)
	}
	originalB, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatalf("ReadFile B: %v", err)
	}

	a.Links = append(a.Links, b.ID)
	b.Links = append(b.Links, a.ID)
	markLinksPresent(a)
	markLinksPresent(b)

	realAtomicWrite := atomicWriteFile
	atomicWriteFile = func(path string, data []byte) error {
		if path == pathB {
			return errors.New("injected write failure")
		}
		return realAtomicWrite(path, data)
	}
	t.Cleanup(func() {
		atomicWriteFile = realAtomicWrite
	})

	err = s.writeFrontmattersUnderLock([]frontmatterWrite{
		{path: pathA, ticket: a},
		{path: pathB, ticket: b},
	})
	if err == nil {
		t.Fatal("writeFrontmattersUnderLock: expected injected failure")
	}

	afterA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatalf("ReadFile A after rollback: %v", err)
	}
	afterB, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatalf("ReadFile B after rollback: %v", err)
	}
	if !bytes.Equal(afterA, originalA) {
		t.Fatalf("ticket A was not restored after partial commit:\n%s", afterA)
	}
	if !bytes.Equal(afterB, originalB) {
		t.Fatalf("ticket B changed unexpectedly:\n%s", afterB)
	}
}

func newInternalTestTicket(title string) *ticket.Ticket {
	t := ticket.NewTicket(
		ticket.WithTitle(title),
		ticket.WithType("task"),
	)
	t.ID = GenerateID("epo")
	t.Present["id"] = true
	return t
}
