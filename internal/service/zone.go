package service

import (
	"context"
	"errors"

	"laghaim-go/internal/repo"
	"laghaim-go/internal/session"
)

type zoneEntryService struct {
	characters repo.CharacterRepository
	sessions   *session.Manager
}

func NewZoneEntryService(characters repo.CharacterRepository, sessions *session.Manager) ZoneEntryService {
	return &zoneEntryService{characters: characters, sessions: sessions}
}

func (s *zoneEntryService) EnterWorld(ctx context.Context, zoneTicket string) (OnlineSpawnResult, error) {
	sessionState, _, err := s.sessions.ConsumeZoneTicket(zoneTicket)
	if err != nil {
		return OnlineSpawnResult{}, err
	}

	character, err := s.characters.GetCharacterByID(ctx, uint64(sessionState.CharacterID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return OnlineSpawnResult{}, ErrCharacterNotFound
		}
		return OnlineSpawnResult{}, err
	}

	return OnlineSpawnResult{
		AccountID:   uint64(sessionState.AccountID),
		CharacterID: character.ID,
		SessionID:   sessionState.SessionID,
		Name:        character.Name,
		Race:        character.Race,
		Sex:         character.Sex,
		Hair:        character.Hair,
		MapID:       character.MapID,
		ZoneID:      character.ZoneID,
		PosX:        character.PosX,
		PosY:        character.PosY,
		PosZ:        character.PosZ,
		Direction:   character.Direction,
	}, nil
}

func (s *zoneEntryService) SaveLogoutPosition(ctx context.Context, characterID uint64, mapID, zoneID uint32, posX, posY, posZ, direction float64) error {
	return s.characters.UpsertCharacterLocation(ctx, repo.UpsertCharacterLocationParams{
		CharacterID: characterID,
		MapID:       mapID,
		ZoneID:      zoneID,
		PosX:        posX,
		PosY:        posY,
		PosZ:        posZ,
		Direction:   direction,
	})
}
