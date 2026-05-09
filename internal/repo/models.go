package repo

import (
	"context"
	"time"
)

type Account struct {
	ID           uint64
	Username     string
	PasswordHash []byte
	PasswordAlgo string
	Status       string
	GMRole       string
	LastLoginAt  *time.Time
	LastLoginIP  *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Character struct {
	ID         uint64
	AccountID  uint64
	SlotIndex  uint8
	Name       string
	Race       uint8
	Sex        uint8
	Hair       uint8
	Level      uint32
	Experience uint64
	MapID      uint32
	ZoneID     uint32
	PosX       float64
	PosY       float64
	PosZ       float64
	Direction  float64
	Money      uint64
	IsDeleted  bool
	RowVersion uint64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CharacterStats struct {
	CharacterID  uint64
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
	SkillPoints  uint32
	StatusPoints uint32
	RowVersion   uint64
	UpdatedAt    time.Time
}

type Inventory struct {
	ID            uint64
	CharacterID   uint64
	InventoryType string
	Capacity      uint32
	RowVersion    uint64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type InventoryItem struct {
	ID           uint64
	InventoryID  uint64
	SlotIndex    uint32
	ItemVNUM     uint32
	Quantity     uint32
	PlusPoint    int32
	SpecialFlag1 int32
	SpecialFlag2 int32
	Endurance    int32
	MaxEndurance int32
	ExtraJSON    []byte
	RowVersion   uint64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Equipment struct {
	CharacterID     uint64
	EquipmentSlot   uint8
	InventoryItemID uint64
	RowVersion      uint64
	UpdatedAt       time.Time
}

type EquippedItem struct {
	EquipmentSlot uint8
	Item          InventoryItem
}

type GMLog struct {
	ID                uint64
	OperatorAccountID uint64
	TargetAccountID   *uint64
	TargetCharacterID *uint64
	Action            string
	Reason            *string
	RequestIP         *string
	PayloadJSON       []byte
	CreatedAt         time.Time
}

type CreateAccountParams struct {
	Username     string
	PasswordHash []byte
	PasswordAlgo string
}

type UpdateLoginMetadataParams struct {
	AccountID   uint64
	LastLoginAt time.Time
	LastLoginIP string
}

type CreateCharacterParams struct {
	AccountID uint64
	SlotIndex uint8
	Name      string
	Race      uint8
	Sex       uint8
	Hair      uint8
	Level     uint32
	MapID     uint32
	ZoneID    uint32
	PosX      float64
	PosY      float64
	PosZ      float64
	Direction float64
	Money     uint64
}

type CreateCharacterStatsParams struct {
	CharacterID  uint64
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
	SkillPoints  uint32
	StatusPoints uint32
}

type UpsertCharacterLocationParams struct {
	CharacterID uint64
	MapID       uint32
	ZoneID      uint32
	PosX        float64
	PosY        float64
	PosZ        float64
	Direction   float64
}

type InsertGMLogParams struct {
	OperatorAccountID uint64
	TargetAccountID   *uint64
	TargetCharacterID *uint64
	Action            string
	Reason            *string
	RequestIP         *string
	PayloadJSON       []byte
}

type CreateInventoryItemParams struct {
	InventoryID  uint64
	SlotIndex    uint32
	ItemVNUM     uint32
	Quantity     uint32
	PlusPoint    int32
	SpecialFlag1 int32
	SpecialFlag2 int32
	Endurance    int32
	MaxEndurance int32
	ExtraJSON    []byte
}

type UpsertEquipmentParams struct {
	CharacterID     uint64
	EquipmentSlot   uint8
	InventoryItemID uint64
}

type MoveInventoryItemParams struct {
	CharacterID uint64
	ItemID      uint64
	InventoryID uint64
	SlotIndex   uint32
}

type AccountRepository interface {
	GetAccountByID(ctx context.Context, id uint64) (Account, error)
	GetAccountByUsername(ctx context.Context, username string) (Account, error)
	CreateAccount(ctx context.Context, params CreateAccountParams) (Account, error)
	UpdateLoginMetadata(ctx context.Context, params UpdateLoginMetadataParams) error
}

type CharacterRepository interface {
	ListCharactersByAccount(ctx context.Context, accountID uint64) ([]Character, error)
	GetCharacterByID(ctx context.Context, id uint64) (Character, error)
	GetCharacterByName(ctx context.Context, name string) (Character, error)
	CreateCharacter(ctx context.Context, params CreateCharacterParams) (Character, error)
	SoftDeleteCharacter(ctx context.Context, accountID, characterID uint64) error
	UpsertCharacterLocation(ctx context.Context, params UpsertCharacterLocationParams) error
}

type CharacterStatsRepository interface {
	GetCharacterStats(ctx context.Context, characterID uint64) (CharacterStats, error)
	CreateCharacterStats(ctx context.Context, params CreateCharacterStatsParams) (CharacterStats, error)
}

type InventoryRepository interface {
	CreateDefaultInventories(ctx context.Context, characterID uint64) error
	ListInventoriesByCharacter(ctx context.Context, characterID uint64) ([]Inventory, error)
	ListInventoryItemsByInventory(ctx context.Context, inventoryID uint64) ([]InventoryItem, error)
	CreateInventoryItem(ctx context.Context, params CreateInventoryItemParams) (InventoryItem, error)
	GetInventoryByType(ctx context.Context, characterID uint64, inventoryType string) (Inventory, error)
	GetInventoryItemForCharacter(ctx context.Context, characterID, itemID uint64) (InventoryItem, Inventory, error)
	MoveInventoryItem(ctx context.Context, params MoveInventoryItemParams) (InventoryItem, error)
	DeleteInventoryItem(ctx context.Context, characterID, itemID uint64) (InventoryItem, error)
	UpsertEquipment(ctx context.Context, params UpsertEquipmentParams) error
	RemoveEquipment(ctx context.Context, characterID uint64, equipmentSlot uint8) error
	ListEquippedItemsByCharacter(ctx context.Context, characterID uint64) ([]EquippedItem, error)
}

type AuditRepository interface {
	InsertGMLog(ctx context.Context, params InsertGMLogParams) (GMLog, error)
}
