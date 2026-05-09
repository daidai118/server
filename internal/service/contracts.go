package service

import "context"

type LoginResult struct {
	AccountID uint64
	SessionID string
	GMSTicket string
}

type CharacterSummary struct {
	CharacterID   uint64
	SlotIndex     uint8
	Name          string
	Race          uint8
	Sex           uint8
	Hair          uint8
	Level         uint32
	MapID         uint32
	ZoneID        uint32
	Vital         uint32
	MaxVital      uint32
	Mana          uint32
	MaxMana       uint32
	Stamina       uint32
	MaxStamina    uint32
	EPower        uint32
	MaxEPower     uint32
	Strength      uint32
	Intelligence  uint32
	Dexterity     uint32
	Constitution  uint32
	Charisma      uint32
	BlockedTime   int64
	GuildIndex    uint64
	Wearings      [8]int32
	IsGuildMaster bool
	IsSupport     bool
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

type CharacterStatusSnapshot struct {
	Vital         uint32
	MaxVital      uint32
	Mana          uint32
	MaxMana       uint32
	Stamina       uint32
	MaxStamina    uint32
	EPower        uint32
	MaxEPower     uint32
	Level         uint32
	Experience    uint64
	Money         uint64
	LevelUpPoints uint32
	Strength      uint32
	Intelligence  uint32
	Dexterity     uint32
	Constitution  uint32
	Charisma      uint32
}

type InventorySnapshot struct {
	InventoryType string
	SlotIndex     uint32
	ItemIndex     uint64
	ItemVNUM      uint32
	Quantity      uint32
	PlusPoint     int32
	SpecialFlag1  int32
	SpecialFlag2  int32
	Endurance     int32
	MaxEndurance  int32
}

type GroundItemSnapshot struct {
	ItemIndex    uint64
	ItemVNUM     uint32
	PlusPoint    int32
	SpecialFlag1 int32
	SpecialFlag2 int32
	Endurance    int32
	MaxEndurance int32
}

type EquipmentSnapshot struct {
	EquipmentSlot uint8
	ItemIndex     uint64
	ItemVNUM      uint32
	PlusPoint     int32
	SpecialFlag1  int32
	SpecialFlag2  int32
	Endurance     int32
	MaxEndurance  int32
}

type OnlineSpawnResult struct {
	AccountID   uint64
	CharacterID uint64
	SessionID   string
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
	Status      CharacterStatusSnapshot
	Inventory   []InventorySnapshot
	Equipment   []EquipmentSnapshot
	MapWearings [7]int32
}

type AuthService interface {
	Register(ctx context.Context, username, password, remoteIP string) (LoginResult, error)
	Login(ctx context.Context, username, password, remoteIP string) (LoginResult, error)
}

type CharacterService interface {
	ListCharacters(ctx context.Context, accountID uint64) ([]CharacterSummary, error)
	IsNameAvailable(ctx context.Context, name string) (bool, error)
	CreateCharacter(ctx context.Context, request CreateCharacterRequest) (CharacterSummary, error)
	DeleteCharacter(ctx context.Context, accountID, characterID uint64) error
	SelectCharacter(ctx context.Context, accountID, characterID uint64) (CharacterSelectionResult, error)
}

type ZoneEntryService interface {
	EnterWorld(ctx context.Context, zoneTicket string) (OnlineSpawnResult, error)
	SaveLogoutPosition(ctx context.Context, characterID uint64, mapID, zoneID uint32, posX, posY, posZ, direction float64) error
	PickGroundItem(ctx context.Context, characterID uint64, item GroundItemSnapshot) (InventorySnapshot, error)
	FindBagItemBySlot(ctx context.Context, characterID uint64, slotIndex uint32) (InventorySnapshot, bool, error)
	MoveBagItem(ctx context.Context, characterID, itemID uint64, slotIndex uint32) (InventorySnapshot, error)
	DropInventoryItem(ctx context.Context, characterID, itemID uint64) (GroundItemSnapshot, error)
	EquipInventoryItem(ctx context.Context, characterID, itemID uint64, equipmentSlot uint8) (EquipmentSnapshot, [7]int32, error)
	UnequipSlot(ctx context.Context, characterID uint64, equipmentSlot uint8) (EquipmentSnapshot, [7]int32, error)
}
