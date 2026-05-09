package service

import (
	"context"
	"errors"
	"sort"

	"laghaim-go/internal/repo"
	"laghaim-go/internal/session"
)

type zoneEntryService struct {
	characters  repo.CharacterRepository
	stats       repo.CharacterStatsRepository
	inventories repo.InventoryRepository
	sessions    *session.Manager
}

func NewZoneEntryService(characters repo.CharacterRepository, stats repo.CharacterStatsRepository, inventories repo.InventoryRepository, sessions *session.Manager) ZoneEntryService {
	return &zoneEntryService{characters: characters, stats: stats, inventories: inventories, sessions: sessions}
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

	stats, err := s.stats.GetCharacterStats(ctx, character.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return OnlineSpawnResult{}, ErrCharacterNotFound
		}
		return OnlineSpawnResult{}, err
	}

	inventories, err := s.inventories.ListInventoriesByCharacter(ctx, character.ID)
	if err != nil {
		return OnlineSpawnResult{}, err
	}

	inventoryItems := make([]InventorySnapshot, 0)
	for _, inventory := range inventories {
		items, err := s.inventories.ListInventoryItemsByInventory(ctx, inventory.ID)
		if err != nil {
			return OnlineSpawnResult{}, err
		}
		for _, item := range items {
			inventoryItems = append(inventoryItems, InventorySnapshot{
				InventoryType: inventory.InventoryType,
				SlotIndex:     item.SlotIndex,
				ItemIndex:     item.ID,
				ItemVNUM:      item.ItemVNUM,
				Quantity:      item.Quantity,
				PlusPoint:     item.PlusPoint,
				SpecialFlag1:  item.SpecialFlag1,
				SpecialFlag2:  item.SpecialFlag2,
				Endurance:     item.Endurance,
				MaxEndurance:  item.MaxEndurance,
			})
		}
	}

	equippedItems, err := s.inventories.ListEquippedItemsByCharacter(ctx, character.ID)
	if err != nil {
		return OnlineSpawnResult{}, err
	}
	equipment, mapWearings := equipmentSnapshots(equippedItems)

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
		Status: CharacterStatusSnapshot{
			Vital:         stats.HP,
			MaxVital:      stats.MaxHP,
			Mana:          stats.MP,
			MaxMana:       stats.MaxMP,
			Stamina:       stats.Stamina,
			MaxStamina:    stats.MaxStamina,
			EPower:        stats.EPower,
			MaxEPower:     stats.MaxEPower,
			Level:         character.Level,
			Experience:    character.Experience,
			Money:         character.Money,
			LevelUpPoints: stats.StatusPoints,
			Strength:      stats.Strength,
			Intelligence:  stats.Intelligence,
			Dexterity:     stats.Dexterity,
			Constitution:  stats.Constitution,
			Charisma:      stats.Charisma,
		},
		Inventory:   inventoryItems,
		Equipment:   equipment,
		MapWearings: mapWearings,
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

func (s *zoneEntryService) PickGroundItem(ctx context.Context, characterID uint64, item GroundItemSnapshot) (InventorySnapshot, error) {
	bag, items, err := s.loadBagItems(ctx, characterID)
	if err != nil {
		return InventorySnapshot{}, err
	}
	slot, ok := firstFreeSlot(bag.Capacity, items)
	if !ok {
		return InventorySnapshot{}, ErrInventoryFull
	}
	created, err := s.inventories.CreateInventoryItem(ctx, repo.CreateInventoryItemParams{
		InventoryID:  bag.ID,
		SlotIndex:    slot,
		ItemVNUM:     item.ItemVNUM,
		Quantity:     1,
		PlusPoint:    item.PlusPoint,
		SpecialFlag1: item.SpecialFlag1,
		SpecialFlag2: item.SpecialFlag2,
		Endurance:    item.Endurance,
		MaxEndurance: item.MaxEndurance,
	})
	if err != nil {
		return InventorySnapshot{}, err
	}
	return inventorySnapshot(bag.InventoryType, created), nil
}

func (s *zoneEntryService) FindBagItemBySlot(ctx context.Context, characterID uint64, slotIndex uint32) (InventorySnapshot, bool, error) {
	bag, items, err := s.loadBagItems(ctx, characterID)
	if err != nil {
		return InventorySnapshot{}, false, err
	}
	for _, item := range items {
		if item.SlotIndex == slotIndex {
			return inventorySnapshot(bag.InventoryType, item), true, nil
		}
	}
	return InventorySnapshot{}, false, nil
}

func (s *zoneEntryService) MoveBagItem(ctx context.Context, characterID, itemID uint64, slotIndex uint32) (InventorySnapshot, error) {
	bag, err := s.inventories.GetInventoryByType(ctx, characterID, "bag")
	if err != nil {
		return InventorySnapshot{}, err
	}
	moved, err := s.inventories.MoveInventoryItem(ctx, repo.MoveInventoryItemParams{
		CharacterID: characterID,
		ItemID:      itemID,
		InventoryID: bag.ID,
		SlotIndex:   slotIndex,
	})
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return InventorySnapshot{}, ErrItemNotFound
		}
		if errors.Is(err, repo.ErrConflict) {
			return InventorySnapshot{}, ErrInventorySlotTaken
		}
		return InventorySnapshot{}, err
	}
	return inventorySnapshot(bag.InventoryType, moved), nil
}

func (s *zoneEntryService) DropInventoryItem(ctx context.Context, characterID, itemID uint64) (GroundItemSnapshot, error) {
	item, err := s.inventories.DeleteInventoryItem(ctx, characterID, itemID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return GroundItemSnapshot{}, ErrItemNotFound
		}
		return GroundItemSnapshot{}, err
	}
	return GroundItemSnapshot{
		ItemIndex:    item.ID,
		ItemVNUM:     item.ItemVNUM,
		PlusPoint:    item.PlusPoint,
		SpecialFlag1: item.SpecialFlag1,
		SpecialFlag2: item.SpecialFlag2,
		Endurance:    item.Endurance,
		MaxEndurance: item.MaxEndurance,
	}, nil
}

func (s *zoneEntryService) EquipInventoryItem(ctx context.Context, characterID, itemID uint64, equipmentSlot uint8) (EquipmentSnapshot, [7]int32, error) {
	item, _, err := s.inventories.GetInventoryItemForCharacter(ctx, characterID, itemID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return EquipmentSnapshot{}, [7]int32{}, ErrItemNotFound
		}
		return EquipmentSnapshot{}, [7]int32{}, err
	}
	if err := s.inventories.UpsertEquipment(ctx, repo.UpsertEquipmentParams{
		CharacterID:     characterID,
		EquipmentSlot:   equipmentSlot,
		InventoryItemID: item.ID,
	}); err != nil {
		return EquipmentSnapshot{}, [7]int32{}, err
	}
	equippedItems, err := s.inventories.ListEquippedItemsByCharacter(ctx, characterID)
	if err != nil {
		return EquipmentSnapshot{}, [7]int32{}, err
	}
	_, mapWearings := equipmentSnapshots(equippedItems)
	return EquipmentSnapshot{
		EquipmentSlot: equipmentSlot,
		ItemIndex:     item.ID,
		ItemVNUM:      item.ItemVNUM,
		PlusPoint:     item.PlusPoint,
		SpecialFlag1:  item.SpecialFlag1,
		SpecialFlag2:  item.SpecialFlag2,
		Endurance:     item.Endurance,
		MaxEndurance:  item.MaxEndurance,
	}, mapWearings, nil
}

func (s *zoneEntryService) UnequipSlot(ctx context.Context, characterID uint64, equipmentSlot uint8) (EquipmentSnapshot, [7]int32, error) {
	equippedItems, err := s.inventories.ListEquippedItemsByCharacter(ctx, characterID)
	if err != nil {
		return EquipmentSnapshot{}, [7]int32{}, err
	}
	var removed repo.EquippedItem
	found := false
	for _, equipped := range equippedItems {
		if equipped.EquipmentSlot == equipmentSlot {
			removed = equipped
			found = true
			break
		}
	}
	if !found {
		return EquipmentSnapshot{}, [7]int32{}, ErrItemNotFound
	}
	if err := s.inventories.RemoveEquipment(ctx, characterID, equipmentSlot); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return EquipmentSnapshot{}, [7]int32{}, ErrItemNotFound
		}
		return EquipmentSnapshot{}, [7]int32{}, err
	}
	equippedItems, err = s.inventories.ListEquippedItemsByCharacter(ctx, characterID)
	if err != nil {
		return EquipmentSnapshot{}, [7]int32{}, err
	}
	_, mapWearings := equipmentSnapshots(equippedItems)
	item := removed.Item
	return EquipmentSnapshot{
		EquipmentSlot: equipmentSlot,
		ItemIndex:     item.ID,
		ItemVNUM:      item.ItemVNUM,
		PlusPoint:     item.PlusPoint,
		SpecialFlag1:  item.SpecialFlag1,
		SpecialFlag2:  item.SpecialFlag2,
		Endurance:     item.Endurance,
		MaxEndurance:  item.MaxEndurance,
	}, mapWearings, nil
}

func (s *zoneEntryService) loadBagItems(ctx context.Context, characterID uint64) (repo.Inventory, []repo.InventoryItem, error) {
	bag, err := s.inventories.GetInventoryByType(ctx, characterID, "bag")
	if err != nil {
		return repo.Inventory{}, nil, err
	}
	items, err := s.inventories.ListInventoryItemsByInventory(ctx, bag.ID)
	if err != nil {
		return repo.Inventory{}, nil, err
	}
	return bag, items, nil
}

func firstFreeSlot(capacity uint32, items []repo.InventoryItem) (uint32, bool) {
	used := make(map[uint32]struct{}, len(items))
	for _, item := range items {
		used[item.SlotIndex] = struct{}{}
	}
	for slot := uint32(0); slot < capacity; slot++ {
		if _, ok := used[slot]; !ok {
			return slot, true
		}
	}
	return 0, false
}

func inventorySnapshot(inventoryType string, item repo.InventoryItem) InventorySnapshot {
	return InventorySnapshot{
		InventoryType: inventoryType,
		SlotIndex:     item.SlotIndex,
		ItemIndex:     item.ID,
		ItemVNUM:      item.ItemVNUM,
		Quantity:      item.Quantity,
		PlusPoint:     item.PlusPoint,
		SpecialFlag1:  item.SpecialFlag1,
		SpecialFlag2:  item.SpecialFlag2,
		Endurance:     item.Endurance,
		MaxEndurance:  item.MaxEndurance,
	}
}

func equipmentSnapshots(equippedItems []repo.EquippedItem) ([]EquipmentSnapshot, [7]int32) {
	sort.Slice(equippedItems, func(i, j int) bool {
		return equippedItems[i].EquipmentSlot < equippedItems[j].EquipmentSlot
	})
	equipment := make([]EquipmentSnapshot, 0, len(equippedItems))
	var mapWearings [7]int32
	for _, equipped := range equippedItems {
		item := equipped.Item
		equipment = append(equipment, EquipmentSnapshot{
			EquipmentSlot: equipped.EquipmentSlot,
			ItemIndex:     item.ID,
			ItemVNUM:      item.ItemVNUM,
			PlusPoint:     item.PlusPoint,
			SpecialFlag1:  item.SpecialFlag1,
			SpecialFlag2:  item.SpecialFlag2,
			Endurance:     item.Endurance,
			MaxEndurance:  item.MaxEndurance,
		})
		if int(equipped.EquipmentSlot) < len(mapWearings) {
			mapWearings[equipped.EquipmentSlot] = int32(item.ItemVNUM)
		}
	}
	return equipment, mapWearings
}
