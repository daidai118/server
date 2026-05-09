package world

import (
	"errors"
	"sync"

	"laghaim-go/internal/service"
)

var ErrPlayerNotFound = errors.New("world: player not found")

type Player struct {
	AccountID   uint64
	CharacterID uint64
	Name        string
	Race        uint8
	Sex         uint8
	Hair        uint8
	MapID       uint32
	ZoneID      uint32
	PosX        float64
	PosY        float64
	PosZ        float64
	Direction   float64
}

type Runtime struct {
	mu      sync.Mutex
	players map[uint64]Player
}

func NewRuntime() *Runtime {
	return &Runtime{players: make(map[uint64]Player)}
}

func (r *Runtime) Join(spawn service.OnlineSpawnResult) (Player, []Player) {
	r.mu.Lock()
	defer r.mu.Unlock()

	player := Player{
		AccountID:   spawn.AccountID,
		CharacterID: spawn.CharacterID,
		Name:        spawn.Name,
		Race:        spawn.Race,
		Sex:         spawn.Sex,
		Hair:        spawn.Hair,
		MapID:       spawn.MapID,
		ZoneID:      spawn.ZoneID,
		PosX:        spawn.PosX,
		PosY:        spawn.PosY,
		PosZ:        spawn.PosZ,
		Direction:   spawn.Direction,
	}

	visible := make([]Player, 0)
	for _, existing := range r.players {
		if existing.MapID == player.MapID && existing.ZoneID == player.ZoneID {
			visible = append(visible, existing)
		}
	}
	r.players[player.CharacterID] = player
	return player, visible
}

func (r *Runtime) Leave(characterID uint64) (Player, []Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	player, ok := r.players[characterID]
	if !ok {
		return Player{}, nil, ErrPlayerNotFound
	}
	delete(r.players, characterID)

	visible := make([]Player, 0)
	for _, existing := range r.players {
		if existing.MapID == player.MapID && existing.ZoneID == player.ZoneID {
			visible = append(visible, existing)
		}
	}
	return player, visible, nil
}

func (r *Runtime) Move(characterID uint64, posX, posY, posZ, direction float64) (Player, []Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	player, ok := r.players[characterID]
	if !ok {
		return Player{}, nil, ErrPlayerNotFound
	}
	player.PosX = posX
	player.PosY = posY
	player.PosZ = posZ
	player.Direction = direction
	r.players[characterID] = player

	visible := make([]Player, 0)
	for _, existing := range r.players {
		if existing.CharacterID == characterID {
			continue
		}
		if existing.MapID == player.MapID && existing.ZoneID == player.ZoneID {
			visible = append(visible, existing)
		}
	}
	return player, visible, nil
}

func (r *Runtime) Snapshot(mapID, zoneID uint32) []Player {
	r.mu.Lock()
	defer r.mu.Unlock()

	players := make([]Player, 0)
	for _, player := range r.players {
		if player.MapID == mapID && player.ZoneID == zoneID {
			players = append(players, player)
		}
	}
	return players
}
