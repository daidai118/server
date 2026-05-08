package session

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func newTestManager(now *time.Time) *Manager {
	nextID := 0
	return NewManager(
		WithClock(func() time.Time { return *now }),
		WithIDGenerator(func(size int) string {
			nextID++
			return fmt.Sprintf("id-%02d", nextID)
		}),
	)
}

func TestTicketCanOnlyBeConsumedOnce(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	manager := newTestManager(&now)

	session := manager.StartAccountLogin(1001)
	if session.Generation != 1 {
		t.Fatalf("unexpected generation: %d", session.Generation)
	}

	ticket, err := manager.IssueGMSTicket(1001, 10*time.Second)
	if err != nil {
		t.Fatalf("IssueGMSTicket() error = %v", err)
	}

	active, consumed, err := manager.ConsumeGMSTicket(ticket.ID)
	if err != nil {
		t.Fatalf("ConsumeGMSTicket() error = %v", err)
	}
	if active.Phase != PhaseGMS {
		t.Fatalf("phase mismatch after GMS consume: got %s want %s", active.Phase, PhaseGMS)
	}
	if consumed.ConsumedAt == nil {
		t.Fatal("ticket should have consumption timestamp")
	}

	_, _, err = manager.ConsumeGMSTicket(ticket.ID)
	if !errors.Is(err, ErrTicketConsumed) {
		t.Fatalf("second consume error = %v, want %v", err, ErrTicketConsumed)
	}
}

func TestNewLoginInvalidatesOldTickets(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	manager := newTestManager(&now)

	first := manager.StartAccountLogin(42)
	firstTicket, err := manager.IssueGMSTicket(42, time.Minute)
	if err != nil {
		t.Fatalf("IssueGMSTicket(first) error = %v", err)
	}

	now = now.Add(2 * time.Second)
	second := manager.StartAccountLogin(42)
	if second.Generation != first.Generation+1 {
		t.Fatalf("generation mismatch: got %d want %d", second.Generation, first.Generation+1)
	}

	_, _, err = manager.ConsumeGMSTicket(firstTicket.ID)
	if !errors.Is(err, ErrSessionReplaced) {
		t.Fatalf("ConsumeGMSTicket(old) error = %v, want %v", err, ErrSessionReplaced)
	}
}

func TestExpiredTicketFails(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	manager := newTestManager(&now)

	manager.StartAccountLogin(7)
	ticket, err := manager.IssueGMSTicket(7, time.Second)
	if err != nil {
		t.Fatalf("IssueGMSTicket() error = %v", err)
	}

	now = now.Add(3 * time.Second)
	_, _, err = manager.ConsumeGMSTicket(ticket.ID)
	if !errors.Is(err, ErrTicketExpired) {
		t.Fatalf("ConsumeGMSTicket(expired) error = %v, want %v", err, ErrTicketExpired)
	}
}

func TestZoneTicketBindsCharacterID(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	manager := newTestManager(&now)

	manager.StartAccountLogin(55)
	gmsTicket, err := manager.IssueGMSTicket(55, time.Minute)
	if err != nil {
		t.Fatalf("IssueGMSTicket() error = %v", err)
	}
	if _, _, err := manager.ConsumeGMSTicket(gmsTicket.ID); err != nil {
		t.Fatalf("ConsumeGMSTicket() error = %v", err)
	}

	zoneTicket, err := manager.IssueZoneTicket(55, 9001, time.Minute)
	if err != nil {
		t.Fatalf("IssueZoneTicket() error = %v", err)
	}

	session, _, err := manager.ConsumeZoneTicket(zoneTicket.ID)
	if err != nil {
		t.Fatalf("ConsumeZoneTicket() error = %v", err)
	}
	if session.Phase != PhaseZone {
		t.Fatalf("phase mismatch: got %s want %s", session.Phase, PhaseZone)
	}
	if session.CharacterID != 9001 {
		t.Fatalf("character mismatch: got %d want %d", session.CharacterID, 9001)
	}
}
