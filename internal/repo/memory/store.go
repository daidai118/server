package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"laghaim-go/internal/repo"
)

type Store struct {
	mu sync.Mutex

	now func() time.Time

	nextAccountID   uint64
	nextCharacterID uint64
	nextInventoryID uint64
	nextGMLogID     uint64

	accountsByID       map[uint64]repo.Account
	accountIDsByName   map[string]uint64
	charactersByID     map[uint64]repo.Character
	characterIDsByName map[string]uint64
	statsByCharacterID map[uint64]repo.CharacterStats
	inventoriesByID    map[uint64]repo.Inventory
	inventoryIDsByChar map[uint64][]uint64
	gmLogsByID         map[uint64]repo.GMLog
}

func NewStore() *Store {
	return &Store{
		now:                time.Now,
		accountsByID:       make(map[uint64]repo.Account),
		accountIDsByName:   make(map[string]uint64),
		charactersByID:     make(map[uint64]repo.Character),
		characterIDsByName: make(map[string]uint64),
		statsByCharacterID: make(map[uint64]repo.CharacterStats),
		inventoriesByID:    make(map[uint64]repo.Inventory),
		inventoryIDsByChar: make(map[uint64][]uint64),
		gmLogsByID:         make(map[uint64]repo.GMLog),
		nextAccountID:      1,
		nextCharacterID:    1,
		nextInventoryID:    1,
		nextGMLogID:        1,
	}
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func normalizeCharacterName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (s *Store) GetAccountByID(_ context.Context, id uint64) (repo.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.accountsByID[id]
	if !ok {
		return repo.Account{}, repo.ErrNotFound
	}
	return account, nil
}

func (s *Store) GetAccountByUsername(_ context.Context, username string) (repo.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.accountIDsByName[normalizeUsername(username)]
	if !ok {
		return repo.Account{}, repo.ErrNotFound
	}
	return s.accountsByID[id], nil
}

func (s *Store) CreateAccount(_ context.Context, params repo.CreateAccountParams) (repo.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := normalizeUsername(params.Username)
	if _, exists := s.accountIDsByName[key]; exists {
		return repo.Account{}, repo.ErrConflict
	}

	now := s.now()
	account := repo.Account{
		ID:           s.nextAccountID,
		Username:     strings.TrimSpace(params.Username),
		PasswordHash: append([]byte(nil), params.PasswordHash...),
		PasswordAlgo: params.PasswordAlgo,
		Status:       "active",
		GMRole:       "player",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.nextAccountID++
	s.accountsByID[account.ID] = account
	s.accountIDsByName[key] = account.ID
	return account, nil
}

func (s *Store) UpdateLoginMetadata(_ context.Context, params repo.UpdateLoginMetadataParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.accountsByID[params.AccountID]
	if !ok {
		return repo.ErrNotFound
	}
	account.LastLoginAt = &params.LastLoginAt
	ip := params.LastLoginIP
	account.LastLoginIP = &ip
	account.UpdatedAt = s.now()
	s.accountsByID[params.AccountID] = account
	return nil
}

func (s *Store) ListCharactersByAccount(_ context.Context, accountID uint64) ([]repo.Character, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	characters := make([]repo.Character, 0, 5)
	for _, character := range s.charactersByID {
		if character.AccountID == accountID && !character.IsDeleted {
			characters = append(characters, character)
		}
	}
	for i := 0; i < len(characters)-1; i++ {
		for j := i + 1; j < len(characters); j++ {
			if characters[j].SlotIndex < characters[i].SlotIndex {
				characters[i], characters[j] = characters[j], characters[i]
			}
		}
	}
	return characters, nil
}

func (s *Store) GetCharacterByID(_ context.Context, id uint64) (repo.Character, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	character, ok := s.charactersByID[id]
	if !ok || character.IsDeleted {
		return repo.Character{}, repo.ErrNotFound
	}
	return character, nil
}

func (s *Store) GetCharacterByName(_ context.Context, name string) (repo.Character, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.characterIDsByName[normalizeCharacterName(name)]
	if !ok {
		return repo.Character{}, repo.ErrNotFound
	}
	character := s.charactersByID[id]
	if character.IsDeleted {
		return repo.Character{}, repo.ErrNotFound
	}
	return character, nil
}

func (s *Store) CreateCharacter(_ context.Context, params repo.CreateCharacterParams) (repo.Character, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nameKey := normalizeCharacterName(params.Name)
	if _, exists := s.characterIDsByName[nameKey]; exists {
		return repo.Character{}, repo.ErrConflict
	}
	for _, character := range s.charactersByID {
		if character.AccountID == params.AccountID && !character.IsDeleted && character.SlotIndex == params.SlotIndex {
			return repo.Character{}, repo.ErrConflict
		}
	}

	now := s.now()
	character := repo.Character{
		ID:         s.nextCharacterID,
		AccountID:  params.AccountID,
		SlotIndex:  params.SlotIndex,
		Name:       strings.TrimSpace(params.Name),
		Race:       params.Race,
		Sex:        params.Sex,
		Hair:       params.Hair,
		Level:      params.Level,
		MapID:      params.MapID,
		ZoneID:     params.ZoneID,
		PosX:       params.PosX,
		PosY:       params.PosY,
		PosZ:       params.PosZ,
		Direction:  params.Direction,
		Money:      params.Money,
		RowVersion: 1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.nextCharacterID++
	s.charactersByID[character.ID] = character
	s.characterIDsByName[nameKey] = character.ID
	return character, nil
}

func (s *Store) SoftDeleteCharacter(_ context.Context, accountID, characterID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	character, ok := s.charactersByID[characterID]
	if !ok || character.IsDeleted {
		return repo.ErrNotFound
	}
	if character.AccountID != accountID {
		return repo.ErrNotFound
	}
	character.IsDeleted = true
	character.UpdatedAt = s.now()
	character.RowVersion++
	s.charactersByID[characterID] = character
	return nil
}

func (s *Store) UpsertCharacterLocation(_ context.Context, params repo.UpsertCharacterLocationParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	character, ok := s.charactersByID[params.CharacterID]
	if !ok || character.IsDeleted {
		return repo.ErrNotFound
	}
	character.MapID = params.MapID
	character.ZoneID = params.ZoneID
	character.PosX = params.PosX
	character.PosY = params.PosY
	character.PosZ = params.PosZ
	character.Direction = params.Direction
	character.RowVersion++
	character.UpdatedAt = s.now()
	s.charactersByID[params.CharacterID] = character
	return nil
}

func (s *Store) GetCharacterStats(_ context.Context, characterID uint64) (repo.CharacterStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats, ok := s.statsByCharacterID[characterID]
	if !ok {
		return repo.CharacterStats{}, repo.ErrNotFound
	}
	return stats, nil
}

func (s *Store) CreateCharacterStats(_ context.Context, params repo.CreateCharacterStatsParams) (repo.CharacterStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.statsByCharacterID[params.CharacterID]; exists {
		return repo.CharacterStats{}, repo.ErrConflict
	}
	stats := repo.CharacterStats{
		CharacterID:  params.CharacterID,
		Strength:     params.Strength,
		Intelligence: params.Intelligence,
		Dexterity:    params.Dexterity,
		Constitution: params.Constitution,
		Charisma:     params.Charisma,
		HP:           params.HP,
		MaxHP:        params.MaxHP,
		MP:           params.MP,
		MaxMP:        params.MaxMP,
		Stamina:      params.Stamina,
		MaxStamina:   params.MaxStamina,
		EPower:       params.EPower,
		MaxEPower:    params.MaxEPower,
		SkillPoints:  params.SkillPoints,
		StatusPoints: params.StatusPoints,
		RowVersion:   1,
		UpdatedAt:    s.now(),
	}
	s.statsByCharacterID[params.CharacterID] = stats
	return stats, nil
}

func (s *Store) CreateDefaultInventories(_ context.Context, characterID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.charactersByID[characterID]; !ok {
		return repo.ErrNotFound
	}
	if len(s.inventoryIDsByChar[characterID]) > 0 {
		return nil
	}

	now := s.now()
	defaults := []repo.Inventory{
		{
			ID:            s.nextInventoryID,
			CharacterID:   characterID,
			InventoryType: "bag",
			Capacity:      60,
			RowVersion:    1,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            s.nextInventoryID + 1,
			CharacterID:   characterID,
			InventoryType: "quickbar",
			Capacity:      10,
			RowVersion:    1,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}
	for _, inventory := range defaults {
		s.inventoriesByID[inventory.ID] = inventory
		s.inventoryIDsByChar[characterID] = append(s.inventoryIDsByChar[characterID], inventory.ID)
	}
	s.nextInventoryID += uint64(len(defaults))
	return nil
}

func (s *Store) InsertGMLog(_ context.Context, params repo.InsertGMLogParams) (repo.GMLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gmLog := repo.GMLog{
		ID:                s.nextGMLogID,
		OperatorAccountID: params.OperatorAccountID,
		TargetAccountID:   params.TargetAccountID,
		TargetCharacterID: params.TargetCharacterID,
		Action:            params.Action,
		Reason:            params.Reason,
		RequestIP:         params.RequestIP,
		PayloadJSON:       append([]byte(nil), params.PayloadJSON...),
		CreatedAt:         s.now(),
	}
	s.nextGMLogID++
	s.gmLogsByID[gmLog.ID] = gmLog
	return gmLog, nil
}

var _ repo.AccountRepository = (*Store)(nil)
var _ repo.CharacterRepository = (*Store)(nil)
var _ repo.CharacterStatsRepository = (*Store)(nil)
var _ repo.InventoryRepository = (*Store)(nil)
var _ repo.AuditRepository = (*Store)(nil)
