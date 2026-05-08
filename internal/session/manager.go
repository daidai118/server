package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrSessionNotFound = errors.New("session: active account session not found")
	ErrSessionReplaced = errors.New("session: account session was replaced by a newer login")
	ErrTicketNotFound  = errors.New("session: ticket not found")
	ErrTicketConsumed  = errors.New("session: ticket already consumed")
	ErrTicketExpired   = errors.New("session: ticket expired")
	ErrTicketKind      = errors.New("session: unexpected ticket kind")
)

type Phase string

const (
	PhaseLogin  Phase = "login"
	PhaseGMS    Phase = "gms"
	PhaseZone   Phase = "zone"
	PhaseClosed Phase = "closed"
)

type TicketKind string

const (
	TicketKindGMS  TicketKind = "gms"
	TicketKindZone TicketKind = "zone"
)

type Session struct {
	SessionID   string
	AccountID   int64
	CharacterID int64
	Generation  uint64
	Phase       Phase
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Ticket struct {
	ID          string
	Kind        TicketKind
	SessionID   string
	AccountID   int64
	CharacterID int64
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
}

type Manager struct {
	mu       sync.Mutex
	now      func() time.Time
	newID    func(size int) string
	sessions map[int64]*Session
	tickets  map[string]*Ticket
}

type Option func(*Manager)

func WithClock(now func() time.Time) Option {
	return func(m *Manager) { m.now = now }
}

func WithIDGenerator(gen func(size int) string) Option {
	return func(m *Manager) { m.newID = gen }
}

func NewManager(opts ...Option) *Manager {
	m := &Manager{
		now:      time.Now,
		newID:    randomHex,
		sessions: make(map[int64]*Session),
		tickets:  make(map[string]*Ticket),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *Manager) StartAccountLogin(accountID int64) Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	generation := uint64(1)
	if current, ok := m.sessions[accountID]; ok {
		generation = current.Generation + 1
		current.Phase = PhaseClosed
		current.UpdatedAt = now
	}

	session := &Session{
		SessionID:  m.newID(16),
		AccountID:  accountID,
		Generation: generation,
		Phase:      PhaseLogin,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	m.sessions[accountID] = session
	return cloneSession(session)
}

func (m *Manager) ActiveSession(accountID int64) (Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[accountID]
	if !ok || session.Phase == PhaseClosed {
		return Session{}, false
	}
	return cloneSession(session), true
}

func (m *Manager) IssueGMSTicket(accountID int64, ttl time.Duration) (Ticket, error) {
	return m.issueTicket(accountID, 0, TicketKindGMS, ttl)
}

func (m *Manager) IssueZoneTicket(accountID, characterID int64, ttl time.Duration) (Ticket, error) {
	return m.issueTicket(accountID, characterID, TicketKindZone, ttl)
}

func (m *Manager) issueTicket(accountID, characterID int64, kind TicketKind, ttl time.Duration) (Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[accountID]
	if !ok || session.Phase == PhaseClosed {
		return Ticket{}, ErrSessionNotFound
	}

	now := m.now()
	ticket := &Ticket{
		ID:          m.newID(24),
		Kind:        kind,
		SessionID:   session.SessionID,
		AccountID:   accountID,
		CharacterID: characterID,
		ExpiresAt:   now.Add(ttl),
	}
	m.tickets[ticket.ID] = ticket
	session.UpdatedAt = now
	return cloneTicket(ticket), nil
}

func (m *Manager) ConsumeGMSTicket(ticketID string) (Session, Ticket, error) {
	return m.consumeTicket(ticketID, TicketKindGMS)
}

func (m *Manager) ConsumeZoneTicket(ticketID string) (Session, Ticket, error) {
	return m.consumeTicket(ticketID, TicketKindZone)
}

func (m *Manager) consumeTicket(ticketID string, expected TicketKind) (Session, Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ticket, ok := m.tickets[ticketID]
	if !ok {
		return Session{}, Ticket{}, ErrTicketNotFound
	}
	if ticket.Kind != expected {
		return Session{}, Ticket{}, fmt.Errorf("%w: got %s want %s", ErrTicketKind, ticket.Kind, expected)
	}
	if ticket.ConsumedAt != nil {
		return Session{}, Ticket{}, ErrTicketConsumed
	}

	now := m.now()
	if now.After(ticket.ExpiresAt) {
		delete(m.tickets, ticketID)
		return Session{}, Ticket{}, ErrTicketExpired
	}

	session, ok := m.sessions[ticket.AccountID]
	if !ok || session.Phase == PhaseClosed {
		delete(m.tickets, ticketID)
		return Session{}, Ticket{}, ErrSessionNotFound
	}
	if session.SessionID != ticket.SessionID {
		delete(m.tickets, ticketID)
		return Session{}, Ticket{}, ErrSessionReplaced
	}

	ticketCopy := cloneTicket(ticket)
	ticketCopy.ConsumedAt = &now
	ticket.ConsumedAt = &now

	switch expected {
	case TicketKindGMS:
		session.Phase = PhaseGMS
	case TicketKindZone:
		session.Phase = PhaseZone
		session.CharacterID = ticket.CharacterID
	}
	session.UpdatedAt = now
	return cloneSession(session), ticketCopy, nil
}

func (m *Manager) CloseAccountSession(accountID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[accountID]
	if !ok {
		return
	}
	session.Phase = PhaseClosed
	session.UpdatedAt = m.now()
}

func cloneSession(src *Session) Session {
	if src == nil {
		return Session{}
	}
	return *src
}

func cloneTicket(src *Ticket) Ticket {
	if src == nil {
		return Ticket{}
	}
	out := *src
	if src.ConsumedAt != nil {
		consumedAt := *src.ConsumedAt
		out.ConsumedAt = &consumedAt
	}
	return out
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}
