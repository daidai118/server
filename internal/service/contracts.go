package service

import "context"

type LoginResult struct {
	AccountID uint64
	SessionID string
	GMSTicket string
}

type CharacterSummary struct {
	CharacterID uint64
	SlotIndex   uint8
	Name        string
	Race        uint8
	Sex         uint8
	Hair        uint8
	Level       uint32
	MapID       uint32
}

type CreateCharacterRequest struct {
	AccountID uint64
	SlotIndex uint8
	Name      string
	Race      uint8
	Sex       uint8
	Hair      uint8
}

type CharacterSelectionResult struct {
	CharacterID uint64
	ZoneTicket  string
}

type OnlineSpawnResult struct {
	AccountID   uint64
	CharacterID uint64
	SessionID   string
}

type AuthService interface {
	Login(ctx context.Context, username, password, remoteIP string) (LoginResult, error)
}

type CharacterService interface {
	ListCharacters(ctx context.Context, accountID uint64) ([]CharacterSummary, error)
	CreateCharacter(ctx context.Context, request CreateCharacterRequest) (CharacterSummary, error)
	DeleteCharacter(ctx context.Context, accountID, characterID uint64) error
	SelectCharacter(ctx context.Context, accountID, characterID uint64) (CharacterSelectionResult, error)
}

type ZoneEntryService interface {
	EnterWorld(ctx context.Context, zoneTicket string) (OnlineSpawnResult, error)
	SaveLogoutPosition(ctx context.Context, characterID uint64, mapID, zoneID uint32, posX, posY, posZ, direction float64) error
}
