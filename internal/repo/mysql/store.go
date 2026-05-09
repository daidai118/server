package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"

	"laghaim-go/internal/repo"
)

const (
	accountColumns   = "id, username, password_hash, password_algo, status, gm_role, last_login_at, last_login_ip, created_at, updated_at"
	characterColumns = "id, account_id, slot_index, name, race, sex, hair, level, experience, map_id, zone_id, pos_x, pos_y, pos_z, direction, money, is_deleted, row_version, created_at, updated_at"
	statsColumns     = "character_id, strength, intelligence, dexterity, constitution, charisma, hp, max_hp, mp, max_mp, stamina, max_stamina, epower, max_epower, skill_points, status_points, row_version, updated_at"
	gmLogColumns     = "id, operator_account_id, target_account_id, target_character_id, action, reason, request_ip, payload_json, created_at"

	starterWeaponVNUM = 5001
	starterQuickVNUM  = 5001
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetAccountByID(ctx context.Context, id uint64) (repo.Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx, "SELECT "+accountColumns+" FROM accounts WHERE id = ? LIMIT 1", id))
}

func (s *Store) GetAccountByUsername(ctx context.Context, username string) (repo.Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx, "SELECT "+accountColumns+" FROM accounts WHERE username = ? LIMIT 1", strings.TrimSpace(username)))
}

func (s *Store) CreateAccount(ctx context.Context, params repo.CreateAccountParams) (repo.Account, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO accounts (username, password_hash, password_algo) VALUES (?, ?, ?)`,
		strings.TrimSpace(params.Username),
		append([]byte(nil), params.PasswordHash...),
		params.PasswordAlgo,
	)
	if err != nil {
		return repo.Account{}, translateError(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return repo.Account{}, fmt.Errorf("last insert account id: %w", err)
	}
	return s.GetAccountByID(ctx, uint64(id))
}

func (s *Store) UpdateLoginMetadata(ctx context.Context, params repo.UpdateLoginMetadataParams) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE accounts SET last_login_at = ?, last_login_ip = ? WHERE id = ?`,
		params.LastLoginAt,
		params.LastLoginIP,
		params.AccountID,
	)
	if err != nil {
		return translateError(err)
	}
	return expectRows(result)
}

func (s *Store) ListCharactersByAccount(ctx context.Context, accountID uint64) ([]repo.Character, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+characterColumns+" FROM characters WHERE account_id = ? AND is_deleted = 0 ORDER BY slot_index ASC",
		accountID,
	)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	characters := make([]repo.Character, 0, 5)
	for rows.Next() {
		character, err := scanCharacter(rows)
		if err != nil {
			return nil, err
		}
		characters = append(characters, character)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}
	return characters, nil
}

func (s *Store) GetCharacterByID(ctx context.Context, id uint64) (repo.Character, error) {
	return scanCharacter(s.db.QueryRowContext(ctx,
		"SELECT "+characterColumns+" FROM characters WHERE id = ? AND is_deleted = 0 LIMIT 1",
		id,
	))
}

func (s *Store) GetCharacterByName(ctx context.Context, name string) (repo.Character, error) {
	return scanCharacter(s.db.QueryRowContext(ctx,
		"SELECT "+characterColumns+" FROM characters WHERE name = ? AND is_deleted = 0 LIMIT 1",
		strings.TrimSpace(name),
	))
}

func (s *Store) CreateCharacter(ctx context.Context, params repo.CreateCharacterParams) (repo.Character, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO characters (
			account_id, slot_index, name, race, sex, hair, level,
			map_id, zone_id, pos_x, pos_y, pos_z, direction, money
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.AccountID,
		params.SlotIndex,
		strings.TrimSpace(params.Name),
		params.Race,
		params.Sex,
		params.Hair,
		params.Level,
		params.MapID,
		params.ZoneID,
		params.PosX,
		params.PosY,
		params.PosZ,
		params.Direction,
		params.Money,
	)
	if err != nil {
		return repo.Character{}, translateError(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return repo.Character{}, fmt.Errorf("last insert character id: %w", err)
	}
	return s.GetCharacterByID(ctx, uint64(id))
}

func (s *Store) SoftDeleteCharacter(ctx context.Context, accountID, characterID uint64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE characters SET is_deleted = 1, row_version = row_version + 1 WHERE account_id = ? AND id = ? AND is_deleted = 0`,
		accountID,
		characterID,
	)
	if err != nil {
		return translateError(err)
	}
	return expectRows(result)
}

func (s *Store) UpsertCharacterLocation(ctx context.Context, params repo.UpsertCharacterLocationParams) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE characters
		 SET map_id = ?, zone_id = ?, pos_x = ?, pos_y = ?, pos_z = ?, direction = ?, row_version = row_version + 1
		 WHERE id = ? AND is_deleted = 0`,
		params.MapID,
		params.ZoneID,
		params.PosX,
		params.PosY,
		params.PosZ,
		params.Direction,
		params.CharacterID,
	)
	if err != nil {
		return translateError(err)
	}
	return expectRows(result)
}

func (s *Store) GetCharacterStats(ctx context.Context, characterID uint64) (repo.CharacterStats, error) {
	return scanCharacterStats(s.db.QueryRowContext(ctx,
		"SELECT "+statsColumns+" FROM character_stats WHERE character_id = ? LIMIT 1",
		characterID,
	))
}

func (s *Store) CreateCharacterStats(ctx context.Context, params repo.CreateCharacterStatsParams) (repo.CharacterStats, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO character_stats (
			character_id, strength, intelligence, dexterity, constitution, charisma,
			hp, max_hp, mp, max_mp, stamina, max_stamina, epower, max_epower, skill_points, status_points
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.CharacterID,
		params.Strength,
		params.Intelligence,
		params.Dexterity,
		params.Constitution,
		params.Charisma,
		params.HP,
		params.MaxHP,
		params.MP,
		params.MaxMP,
		params.Stamina,
		params.MaxStamina,
		params.EPower,
		params.MaxEPower,
		params.SkillPoints,
		params.StatusPoints,
	)
	if err != nil {
		return repo.CharacterStats{}, translateError(err)
	}
	return s.GetCharacterStats(ctx, params.CharacterID)
}

func (s *Store) CreateDefaultInventories(ctx context.Context, characterID uint64) error {
	for _, inventory := range []struct {
		inventoryType string
		capacity      uint32
	}{
		{inventoryType: "bag", capacity: 60},
		{inventoryType: "quickbar", capacity: 12},
	} {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO inventories (character_id, inventory_type, capacity)
			VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE capacity = VALUES(capacity)`,
			characterID,
			inventory.inventoryType,
			inventory.capacity,
		); err != nil {
			return translateError(err)
		}
	}

	bag, err := s.GetInventoryByType(ctx, characterID, "bag")
	if err != nil {
		return err
	}
	quickbar, err := s.GetInventoryByType(ctx, characterID, "quickbar")
	if err != nil {
		return err
	}
	bagItem, err := s.createOrUpdateInventoryItem(ctx, repo.CreateInventoryItemParams{
		InventoryID:  bag.ID,
		SlotIndex:    0,
		ItemVNUM:     starterWeaponVNUM,
		Quantity:     1,
		Endurance:    100,
		MaxEndurance: 100,
	})
	if err != nil {
		return err
	}
	if _, err := s.createOrUpdateInventoryItem(ctx, repo.CreateInventoryItemParams{
		InventoryID: quickbar.ID,
		SlotIndex:   0,
		ItemVNUM:    starterQuickVNUM,
		Quantity:    1,
	}); err != nil {
		return err
	}
	return s.UpsertEquipment(ctx, repo.UpsertEquipmentParams{
		CharacterID:     characterID,
		EquipmentSlot:   0,
		InventoryItemID: bagItem.ID,
	})
}

func (s *Store) GetInventoryByType(ctx context.Context, characterID uint64, inventoryType string) (repo.Inventory, error) {
	var inventory repo.Inventory
	err := s.db.QueryRowContext(ctx,
		`SELECT id, character_id, inventory_type, capacity, row_version, created_at, updated_at
		 FROM inventories
		 WHERE character_id = ? AND inventory_type = ?
		 LIMIT 1`,
		characterID,
		inventoryType,
	).Scan(
		&inventory.ID,
		&inventory.CharacterID,
		&inventory.InventoryType,
		&inventory.Capacity,
		&inventory.RowVersion,
		&inventory.CreatedAt,
		&inventory.UpdatedAt,
	)
	if err != nil {
		return repo.Inventory{}, translateError(err)
	}
	return inventory, nil
}

func (s *Store) createOrUpdateInventoryItem(ctx context.Context, params repo.CreateInventoryItemParams) (repo.InventoryItem, error) {
	if params.Quantity == 0 {
		params.Quantity = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO inventory_items (
			inventory_id, slot_index, item_vnum, quantity, plus_point, special_flag_1, special_flag_2,
			endurance, max_endurance, extra_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			item_vnum = VALUES(item_vnum),
			quantity = VALUES(quantity),
			plus_point = VALUES(plus_point),
			special_flag_1 = VALUES(special_flag_1),
			special_flag_2 = VALUES(special_flag_2),
			endurance = VALUES(endurance),
			max_endurance = VALUES(max_endurance),
			extra_json = VALUES(extra_json),
			row_version = row_version + 1`,
		params.InventoryID,
		params.SlotIndex,
		params.ItemVNUM,
		params.Quantity,
		params.PlusPoint,
		params.SpecialFlag1,
		params.SpecialFlag2,
		params.Endurance,
		params.MaxEndurance,
		nullBytes(params.ExtraJSON),
	)
	if err != nil {
		return repo.InventoryItem{}, translateError(err)
	}
	return s.getInventoryItemBySlot(ctx, params.InventoryID, params.SlotIndex)
}

func (s *Store) getInventoryItemBySlot(ctx context.Context, inventoryID uint64, slotIndex uint32) (repo.InventoryItem, error) {
	return scanInventoryItem(s.db.QueryRowContext(ctx, `
		SELECT id, inventory_id, slot_index, item_vnum, quantity, plus_point, special_flag_1, special_flag_2,
		       endurance, max_endurance, extra_json, row_version, created_at, updated_at
		FROM inventory_items
		WHERE inventory_id = ? AND slot_index = ?
		LIMIT 1`, inventoryID, slotIndex))
}

func (s *Store) getInventoryItemByID(ctx context.Context, itemID uint64) (repo.InventoryItem, error) {
	return scanInventoryItem(s.db.QueryRowContext(ctx, `
		SELECT id, inventory_id, slot_index, item_vnum, quantity, plus_point, special_flag_1, special_flag_2,
		       endurance, max_endurance, extra_json, row_version, created_at, updated_at
		FROM inventory_items
		WHERE id = ?
		LIMIT 1`, itemID))
}

func (s *Store) GetInventoryItemForCharacter(ctx context.Context, characterID, itemID uint64) (repo.InventoryItem, repo.Inventory, error) {
	var inventory repo.Inventory
	item, err := scanInventoryItemWithPrefix(s.db.QueryRowContext(ctx, `
		SELECT inv.id, inv.character_id, inv.inventory_type, inv.capacity, inv.row_version, inv.created_at, inv.updated_at,
		       i.id, i.inventory_id, i.slot_index, i.item_vnum, i.quantity, i.plus_point, i.special_flag_1, i.special_flag_2,
		       i.endurance, i.max_endurance, i.extra_json, i.row_version, i.created_at, i.updated_at
		FROM inventory_items i
		JOIN inventories inv ON inv.id = i.inventory_id
		WHERE i.id = ? AND inv.character_id = ?
		LIMIT 1`,
		itemID,
		characterID,
	),
		&inventory.ID,
		&inventory.CharacterID,
		&inventory.InventoryType,
		&inventory.Capacity,
		&inventory.RowVersion,
		&inventory.CreatedAt,
		&inventory.UpdatedAt,
	)
	if err != nil {
		return repo.InventoryItem{}, repo.Inventory{}, err
	}
	return item, inventory, nil
}

func (s *Store) CreateInventoryItem(ctx context.Context, params repo.CreateInventoryItemParams) (repo.InventoryItem, error) {
	if params.Quantity == 0 {
		params.Quantity = 1
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO inventory_items (
			inventory_id, slot_index, item_vnum, quantity, plus_point, special_flag_1, special_flag_2,
			endurance, max_endurance, extra_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.InventoryID,
		params.SlotIndex,
		params.ItemVNUM,
		params.Quantity,
		params.PlusPoint,
		params.SpecialFlag1,
		params.SpecialFlag2,
		params.Endurance,
		params.MaxEndurance,
		nullBytes(params.ExtraJSON),
	)
	if err != nil {
		return repo.InventoryItem{}, translateError(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return repo.InventoryItem{}, fmt.Errorf("last insert inventory item id: %w", err)
	}
	return s.getInventoryItemByID(ctx, uint64(id))
}

func (s *Store) MoveInventoryItem(ctx context.Context, params repo.MoveInventoryItemParams) (repo.InventoryItem, error) {
	ok, err := s.inventoryBelongsToCharacter(ctx, params.InventoryID, params.CharacterID)
	if err != nil {
		return repo.InventoryItem{}, err
	}
	if !ok {
		return repo.InventoryItem{}, repo.ErrNotFound
	}
	if ok, err := s.inventoryItemBelongsToCharacter(ctx, params.ItemID, params.CharacterID); err != nil {
		return repo.InventoryItem{}, err
	} else if !ok {
		return repo.InventoryItem{}, repo.ErrNotFound
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE inventory_items
		SET inventory_id = ?, slot_index = ?, row_version = row_version + 1
		WHERE id = ?`,
		params.InventoryID,
		params.SlotIndex,
		params.ItemID,
	)
	if err != nil {
		return repo.InventoryItem{}, translateError(err)
	}
	if err := expectRows(result); err != nil {
		return repo.InventoryItem{}, err
	}
	return s.getInventoryItemByID(ctx, params.ItemID)
}

func (s *Store) DeleteInventoryItem(ctx context.Context, characterID, itemID uint64) (repo.InventoryItem, error) {
	item, _, err := s.GetInventoryItemForCharacter(ctx, characterID, itemID)
	if err != nil {
		return repo.InventoryItem{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return repo.InventoryItem{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM equipments WHERE character_id = ? AND inventory_item_id = ?`, characterID, itemID); err != nil {
		return repo.InventoryItem{}, translateError(err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM inventory_items WHERE id = ?`, itemID)
	if err != nil {
		return repo.InventoryItem{}, translateError(err)
	}
	if err := expectRows(result); err != nil {
		return repo.InventoryItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return repo.InventoryItem{}, err
	}
	return item, nil
}

func (s *Store) UpsertEquipment(ctx context.Context, params repo.UpsertEquipmentParams) error {
	ok, err := s.inventoryItemBelongsToCharacter(ctx, params.InventoryItemID, params.CharacterID)
	if err != nil {
		return err
	}
	if !ok {
		return repo.ErrNotFound
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO equipments (character_id, equipment_slot, inventory_item_id)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			inventory_item_id = VALUES(inventory_item_id),
			row_version = row_version + 1`,
		params.CharacterID,
		params.EquipmentSlot,
		params.InventoryItemID,
	)
	return translateError(err)
}

func (s *Store) RemoveEquipment(ctx context.Context, characterID uint64, equipmentSlot uint8) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM equipments WHERE character_id = ? AND equipment_slot = ?`, characterID, equipmentSlot)
	if err != nil {
		return translateError(err)
	}
	return expectRows(result)
}

func (s *Store) inventoryItemBelongsToCharacter(ctx context.Context, itemID, characterID uint64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM inventory_items i
			JOIN inventories inv ON inv.id = i.inventory_id
			WHERE i.id = ? AND inv.character_id = ?
		)`,
		itemID,
		characterID,
	).Scan(&exists)
	if err != nil {
		return false, translateError(err)
	}
	return exists, nil
}

func (s *Store) inventoryBelongsToCharacter(ctx context.Context, inventoryID, characterID uint64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM inventories
			WHERE id = ? AND character_id = ?
		)`,
		inventoryID,
		characterID,
	).Scan(&exists)
	if err != nil {
		return false, translateError(err)
	}
	return exists, nil
}

func (s *Store) ListEquippedItemsByCharacter(ctx context.Context, characterID uint64) ([]repo.EquippedItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.equipment_slot,
		       i.id, i.inventory_id, i.slot_index, i.item_vnum, i.quantity, i.plus_point, i.special_flag_1, i.special_flag_2,
		       i.endurance, i.max_endurance, i.extra_json, i.row_version, i.created_at, i.updated_at
		FROM equipments e
		JOIN inventory_items i ON i.id = e.inventory_item_id
		WHERE e.character_id = ?
		ORDER BY e.equipment_slot ASC`,
		characterID,
	)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	items := make([]repo.EquippedItem, 0)
	for rows.Next() {
		var equipped repo.EquippedItem
		item, err := scanInventoryItemWithPrefix(rows, &equipped.EquipmentSlot)
		if err != nil {
			return nil, err
		}
		equipped.Item = item
		items = append(items, equipped)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}
	return items, nil
}

func (s *Store) ListInventoriesByCharacter(ctx context.Context, characterID uint64) ([]repo.Inventory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, character_id, inventory_type, capacity, row_version, created_at, updated_at
		 FROM inventories
		 WHERE character_id = ?
		 ORDER BY inventory_type ASC, id ASC`,
		characterID,
	)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	inventories := make([]repo.Inventory, 0, 2)
	for rows.Next() {
		var inventory repo.Inventory
		if err := rows.Scan(
			&inventory.ID,
			&inventory.CharacterID,
			&inventory.InventoryType,
			&inventory.Capacity,
			&inventory.RowVersion,
			&inventory.CreatedAt,
			&inventory.UpdatedAt,
		); err != nil {
			return nil, translateError(err)
		}
		inventories = append(inventories, inventory)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}
	return inventories, nil
}

func (s *Store) ListInventoryItemsByInventory(ctx context.Context, inventoryID uint64) ([]repo.InventoryItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, inventory_id, slot_index, item_vnum, quantity, plus_point, special_flag_1, special_flag_2,
		       endurance, max_endurance, extra_json, row_version, created_at, updated_at
		FROM inventory_items
		WHERE inventory_id = ?
		ORDER BY slot_index ASC, id ASC`, inventoryID)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	items := make([]repo.InventoryItem, 0)
	for rows.Next() {
		item, err := scanInventoryItem(rows)
		if err != nil {
			return nil, translateError(err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}
	return items, nil
}

func (s *Store) InsertGMLog(ctx context.Context, params repo.InsertGMLogParams) (repo.GMLog, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO gm_logs (
			operator_account_id, target_account_id, target_character_id, action, reason, request_ip, payload_json
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		params.OperatorAccountID,
		nullUint64(params.TargetAccountID),
		nullUint64(params.TargetCharacterID),
		params.Action,
		nullString(params.Reason),
		nullString(params.RequestIP),
		nullBytes(params.PayloadJSON),
	)
	if err != nil {
		return repo.GMLog{}, translateError(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return repo.GMLog{}, fmt.Errorf("last insert gm_log id: %w", err)
	}
	return scanGMLog(s.db.QueryRowContext(ctx, "SELECT "+gmLogColumns+" FROM gm_logs WHERE id = ? LIMIT 1", id))
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAccount(row scanner) (repo.Account, error) {
	var account repo.Account
	var lastLoginAt sql.NullTime
	var lastLoginIP sql.NullString
	if err := row.Scan(
		&account.ID,
		&account.Username,
		&account.PasswordHash,
		&account.PasswordAlgo,
		&account.Status,
		&account.GMRole,
		&lastLoginAt,
		&lastLoginIP,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return repo.Account{}, translateError(err)
	}
	if lastLoginAt.Valid {
		value := lastLoginAt.Time
		account.LastLoginAt = &value
	}
	if lastLoginIP.Valid {
		value := lastLoginIP.String
		account.LastLoginIP = &value
	}
	return account, nil
}

func scanCharacter(row scanner) (repo.Character, error) {
	var character repo.Character
	var isDeleted uint8
	if err := row.Scan(
		&character.ID,
		&character.AccountID,
		&character.SlotIndex,
		&character.Name,
		&character.Race,
		&character.Sex,
		&character.Hair,
		&character.Level,
		&character.Experience,
		&character.MapID,
		&character.ZoneID,
		&character.PosX,
		&character.PosY,
		&character.PosZ,
		&character.Direction,
		&character.Money,
		&isDeleted,
		&character.RowVersion,
		&character.CreatedAt,
		&character.UpdatedAt,
	); err != nil {
		return repo.Character{}, translateError(err)
	}
	character.IsDeleted = isDeleted != 0
	return character, nil
}

func scanCharacterStats(row scanner) (repo.CharacterStats, error) {
	var stats repo.CharacterStats
	if err := row.Scan(
		&stats.CharacterID,
		&stats.Strength,
		&stats.Intelligence,
		&stats.Dexterity,
		&stats.Constitution,
		&stats.Charisma,
		&stats.HP,
		&stats.MaxHP,
		&stats.MP,
		&stats.MaxMP,
		&stats.Stamina,
		&stats.MaxStamina,
		&stats.EPower,
		&stats.MaxEPower,
		&stats.SkillPoints,
		&stats.StatusPoints,
		&stats.RowVersion,
		&stats.UpdatedAt,
	); err != nil {
		return repo.CharacterStats{}, translateError(err)
	}
	return stats, nil
}

func scanInventoryItem(row scanner) (repo.InventoryItem, error) {
	var item repo.InventoryItem
	var extraJSON []byte
	if err := row.Scan(
		&item.ID,
		&item.InventoryID,
		&item.SlotIndex,
		&item.ItemVNUM,
		&item.Quantity,
		&item.PlusPoint,
		&item.SpecialFlag1,
		&item.SpecialFlag2,
		&item.Endurance,
		&item.MaxEndurance,
		&extraJSON,
		&item.RowVersion,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return repo.InventoryItem{}, translateError(err)
	}
	item.ExtraJSON = append([]byte(nil), extraJSON...)
	return item, nil
}

func scanInventoryItemWithPrefix(row scanner, prefix ...any) (repo.InventoryItem, error) {
	var item repo.InventoryItem
	var extraJSON []byte
	dest := append(prefix,
		&item.ID,
		&item.InventoryID,
		&item.SlotIndex,
		&item.ItemVNUM,
		&item.Quantity,
		&item.PlusPoint,
		&item.SpecialFlag1,
		&item.SpecialFlag2,
		&item.Endurance,
		&item.MaxEndurance,
		&extraJSON,
		&item.RowVersion,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err := row.Scan(dest...); err != nil {
		return repo.InventoryItem{}, translateError(err)
	}
	item.ExtraJSON = append([]byte(nil), extraJSON...)
	return item, nil
}

func scanGMLog(row scanner) (repo.GMLog, error) {
	var gmLog repo.GMLog
	var targetAccountID sql.NullInt64
	var targetCharacterID sql.NullInt64
	var reason sql.NullString
	var requestIP sql.NullString
	var payload []byte
	if err := row.Scan(
		&gmLog.ID,
		&gmLog.OperatorAccountID,
		&targetAccountID,
		&targetCharacterID,
		&gmLog.Action,
		&reason,
		&requestIP,
		&payload,
		&gmLog.CreatedAt,
	); err != nil {
		return repo.GMLog{}, translateError(err)
	}
	if targetAccountID.Valid {
		value := uint64(targetAccountID.Int64)
		gmLog.TargetAccountID = &value
	}
	if targetCharacterID.Valid {
		value := uint64(targetCharacterID.Int64)
		gmLog.TargetCharacterID = &value
	}
	if reason.Valid {
		value := reason.String
		gmLog.Reason = &value
	}
	if requestIP.Valid {
		value := requestIP.String
		gmLog.RequestIP = &value
	}
	gmLog.PayloadJSON = append([]byte(nil), payload...)
	return gmLog, nil
}

func expectRows(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return repo.ErrNotFound
	}
	return nil
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return repo.ErrNotFound
	}
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1062:
			return repo.ErrConflict
		case 1451, 1452:
			return repo.ErrNotFound
		}
	}
	return err
}

func nullUint64(value *uint64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func nullBytes(value []byte) any {
	if value == nil {
		return nil
	}
	return append([]byte(nil), value...)
}

var _ repo.AccountRepository = (*Store)(nil)
var _ repo.CharacterRepository = (*Store)(nil)
var _ repo.CharacterStatsRepository = (*Store)(nil)
var _ repo.InventoryRepository = (*Store)(nil)
var _ repo.AuditRepository = (*Store)(nil)
