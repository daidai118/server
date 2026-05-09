package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"laghaim-go/internal/repo"
	"laghaim-go/internal/session"
)

type CharacterConfig struct {
	MaxCharacters uint8
	ZoneTicketTTL time.Duration
	StartMapID    uint32
	StartZoneID   uint32
	StartPosX     float64
	StartPosY     float64
	StartPosZ     float64
	StartDir      float64
	StartMoney    uint64
}

type characterService struct {
	characters  repo.CharacterRepository
	stats       repo.CharacterStatsRepository
	inventories repo.InventoryRepository
	sessions    *session.Manager
	handoffs    *ZoneHandoffRegistry
	config      CharacterConfig
}

func NewCharacterService(characters repo.CharacterRepository, stats repo.CharacterStatsRepository, inventories repo.InventoryRepository, sessions *session.Manager, handoffs *ZoneHandoffRegistry, config CharacterConfig) CharacterService {
	if handoffs == nil {
		handoffs = NewZoneHandoffRegistry()
	}
	if config.MaxCharacters == 0 {
		config.MaxCharacters = 5
	}
	if config.ZoneTicketTTL <= 0 {
		config.ZoneTicketTTL = 2 * time.Minute
	}
	if config.StartMapID == 0 {
		config.StartMapID = 1
	}
	if config.StartPosX == 0 {
		config.StartPosX = 33000
	}
	if config.StartPosZ == 0 {
		config.StartPosZ = 33000
	}
	if config.StartMoney == 0 {
		config.StartMoney = 1000
	}
	return &characterService{
		characters:  characters,
		stats:       stats,
		inventories: inventories,
		sessions:    sessions,
		handoffs:    handoffs,
		config:      config,
	}
}

func (s *characterService) ListCharacters(ctx context.Context, accountID uint64) ([]CharacterSummary, error) {
	characters, err := s.characters.ListCharactersByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}

	out := make([]CharacterSummary, 0, len(characters))
	for _, character := range characters {
		stats, err := s.stats.GetCharacterStats(ctx, character.ID)
		if err != nil {
			return nil, err
		}
		equippedItems, err := s.inventories.ListEquippedItemsByCharacter(ctx, character.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, summarizeCharacter(character, stats, equippedItems))
	}
	return out, nil
}

func (s *characterService) IsNameAvailable(ctx context.Context, name string) (bool, error) {
	_, err := s.characters.GetCharacterByName(ctx, name)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, repo.ErrNotFound) {
		return true, nil
	}
	return false, err
}

func (s *characterService) CreateCharacter(ctx context.Context, request CreateCharacterRequest) (CharacterSummary, error) {
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) < 2 {
		return CharacterSummary{}, ErrCharacterNameTaken
	}

	characters, err := s.characters.ListCharactersByAccount(ctx, request.AccountID)
	if err != nil {
		return CharacterSummary{}, err
	}
	if len(characters) >= int(s.config.MaxCharacters) {
		return CharacterSummary{}, ErrCharacterLimit
	}

	for _, character := range characters {
		if character.SlotIndex == request.SlotIndex {
			return CharacterSummary{}, ErrCharacterSlotTaken
		}
	}

	statsConfig := starterStats(request.Race)
	character, err := s.characters.CreateCharacter(ctx, repo.CreateCharacterParams{
		AccountID: request.AccountID,
		SlotIndex: request.SlotIndex,
		Name:      request.Name,
		Race:      request.Race,
		Sex:       request.Sex,
		Hair:      request.Hair,
		Level:     1,
		MapID:     s.config.StartMapID,
		ZoneID:    s.config.StartZoneID,
		PosX:      s.config.StartPosX,
		PosY:      s.config.StartPosY,
		PosZ:      s.config.StartPosZ,
		Direction: s.config.StartDir,
		Money:     s.config.StartMoney,
	})
	if err != nil {
		if errors.Is(err, repo.ErrConflict) {
			return CharacterSummary{}, ErrCharacterNameTaken
		}
		return CharacterSummary{}, err
	}

	stats, err := s.stats.CreateCharacterStats(ctx, repo.CreateCharacterStatsParams{
		CharacterID:  character.ID,
		Strength:     statsConfig.Strength,
		Intelligence: statsConfig.Intelligence,
		Dexterity:    statsConfig.Dexterity,
		Constitution: statsConfig.Constitution,
		Charisma:     statsConfig.Charisma,
		HP:           statsConfig.HP,
		MaxHP:        statsConfig.MaxHP,
		MP:           statsConfig.MP,
		MaxMP:        statsConfig.MaxMP,
		Stamina:      statsConfig.Stamina,
		MaxStamina:   statsConfig.MaxStamina,
		EPower:       statsConfig.EPower,
		MaxEPower:    statsConfig.MaxEPower,
	})
	if err != nil {
		return CharacterSummary{}, err
	}

	if err := s.inventories.CreateDefaultInventories(ctx, character.ID); err != nil {
		return CharacterSummary{}, err
	}

	equippedItems, err := s.inventories.ListEquippedItemsByCharacter(ctx, character.ID)
	if err != nil {
		return CharacterSummary{}, err
	}
	return summarizeCharacter(character, stats, equippedItems), nil
}

func (s *characterService) DeleteCharacter(ctx context.Context, accountID, characterID uint64) error {
	if err := s.characters.SoftDeleteCharacter(ctx, accountID, characterID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrCharacterNotFound
		}
		return err
	}
	return nil
}

func (s *characterService) SelectCharacter(ctx context.Context, accountID, characterID uint64) (CharacterSelectionResult, error) {
	character, err := s.characters.GetCharacterByID(ctx, characterID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return CharacterSelectionResult{}, ErrCharacterNotFound
		}
		return CharacterSelectionResult{}, err
	}
	if character.AccountID != accountID {
		return CharacterSelectionResult{}, ErrCharacterNotFound
	}

	zoneTicket, err := s.sessions.IssueZoneTicket(int64(accountID), int64(characterID), s.config.ZoneTicketTTL)
	if err != nil {
		return CharacterSelectionResult{}, err
	}
	s.handoffs.Put(accountID, zoneTicket.ID)

	return CharacterSelectionResult{
		CharacterID: character.ID,
		ZoneTicket:  zoneTicket.ID,
	}, nil
}

type baseStats struct {
	Strength     uint32
	Intelligence uint32
	Dexterity    uint32
	Constitution uint32
	Charisma     uint32
	HP           uint32
	MaxHP        uint32
	MP           uint32
	MaxMP        uint32
	Stamina      uint32
	MaxStamina   uint32
	EPower       uint32
	MaxEPower    uint32
}

func starterStats(race uint8) baseStats {
	base := baseStats{HP: 100, MaxHP: 100, MP: 50, MaxMP: 50, Stamina: 100, MaxStamina: 100, EPower: 100, MaxEPower: 100}
	switch race {
	case 0:
		base.Strength = 12
		base.Intelligence = 8
		base.Dexterity = 10
		base.Constitution = 12
		base.Charisma = 8
	case 1:
		base.Strength = 8
		base.Intelligence = 14
		base.Dexterity = 10
		base.Constitution = 8
		base.Charisma = 10
	case 3:
		base.Strength = 14
		base.Intelligence = 6
		base.Dexterity = 8
		base.Constitution = 14
		base.Charisma = 8
	default:
		base.Strength = 10
		base.Intelligence = 10
		base.Dexterity = 12
		base.Constitution = 10
		base.Charisma = 8
	}
	return base
}

func summarizeCharacter(character repo.Character, stats repo.CharacterStats, equippedItems []repo.EquippedItem) CharacterSummary {
	var wearings [8]int32
	for _, equipped := range equippedItems {
		if int(equipped.EquipmentSlot) < len(wearings) {
			wearings[equipped.EquipmentSlot] = int32(equipped.Item.ItemVNUM)
		}
	}
	return CharacterSummary{
		CharacterID:  character.ID,
		SlotIndex:    character.SlotIndex,
		Name:         character.Name,
		Race:         character.Race,
		Sex:          character.Sex,
		Hair:         character.Hair,
		Level:        character.Level,
		MapID:        character.MapID,
		ZoneID:       character.ZoneID,
		Vital:        stats.HP,
		MaxVital:     stats.MaxHP,
		Mana:         stats.MP,
		MaxMana:      stats.MaxMP,
		Stamina:      stats.Stamina,
		MaxStamina:   stats.MaxStamina,
		EPower:       stats.EPower,
		MaxEPower:    stats.MaxEPower,
		Strength:     stats.Strength,
		Intelligence: stats.Intelligence,
		Dexterity:    stats.Dexterity,
		Constitution: stats.Constitution,
		Charisma:     stats.Charisma,
		Wearings:     wearings,
	}
}
