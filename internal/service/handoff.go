package service

import "sync"

type ZoneHandoffRegistry struct {
	mu        sync.Mutex
	byAccount map[uint64]string
}

func NewZoneHandoffRegistry() *ZoneHandoffRegistry {
	return &ZoneHandoffRegistry{byAccount: make(map[uint64]string)}
}

func (r *ZoneHandoffRegistry) Put(accountID uint64, zoneTicket string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byAccount[accountID] = zoneTicket
}

func (r *ZoneHandoffRegistry) Consume(accountID uint64) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ticket, ok := r.byAccount[accountID]
	if ok {
		delete(r.byAccount, accountID)
	}
	return ticket, ok
}
