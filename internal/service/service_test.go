package service

import (
	"context"
	"testing"
	"time"

	"laghaim-go/internal/repo/memory"
	"laghaim-go/internal/session"
)

func newTestServices() (AuthService, CharacterService, ZoneEntryService, *ZoneHandoffRegistry) {
	store := memory.NewStore()
	sessions := session.NewManager()
	handoffs := NewZoneHandoffRegistry()
	hasher := DefaultPasswordHasher()

	auth := NewAuthService(store, sessions, hasher, AuthConfig{GMSTicketTTL: time.Minute})
	chars := NewCharacterService(store, store, store, sessions, handoffs, CharacterConfig{ZoneTicketTTL: time.Minute})
	zone := NewZoneEntryService(store, store, store, sessions)
	return auth, chars, zone, handoffs
}

func TestRegisterCreateSelectEnterWorld(t *testing.T) {
	ctx := context.Background()
	auth, chars, zone, handoffs := newTestServices()

	login, err := auth.Register(ctx, "alice", "secret", "127.0.0.1")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if login.AccountID == 0 || login.SessionID == "" || login.GMSTicket == "" {
		t.Fatalf("unexpected login result: %+v", login)
	}

	available, err := chars.IsNameAvailable(ctx, "AliceHero")
	if err != nil {
		t.Fatalf("IsNameAvailable() error = %v", err)
	}
	if !available {
		t.Fatal("name should be available before character creation")
	}

	created, err := chars.CreateCharacter(ctx, CreateCharacterRequest{
		AccountID: login.AccountID,
		SlotIndex: 0,
		Name:      "AliceHero",
		Race:      2,
		Sex:       0,
		Hair:      1,
	})
	if err != nil {
		t.Fatalf("CreateCharacter() error = %v", err)
	}
	if created.CharacterID == 0 || created.Level != 1 || created.MapID != 1 {
		t.Fatalf("unexpected created character: %+v", created)
	}

	characters, err := chars.ListCharacters(ctx, login.AccountID)
	if err != nil {
		t.Fatalf("ListCharacters() error = %v", err)
	}
	if len(characters) != 1 {
		t.Fatalf("character count mismatch: got %d want 1", len(characters))
	}

	selection, err := chars.SelectCharacter(ctx, login.AccountID, created.CharacterID)
	if err != nil {
		t.Fatalf("SelectCharacter() error = %v", err)
	}
	if selection.ZoneTicket == "" {
		t.Fatal("zone ticket should not be empty")
	}

	consumed, ok := handoffs.Consume(login.AccountID)
	if !ok || consumed != selection.ZoneTicket {
		t.Fatalf("handoff mismatch: got %q ok=%v want %q", consumed, ok, selection.ZoneTicket)
	}
	// Put it back so the zone service can consume it for the real check.
	handoffs.Put(login.AccountID, selection.ZoneTicket)

	spawn, err := zone.EnterWorld(ctx, selection.ZoneTicket)
	if err != nil {
		t.Fatalf("EnterWorld() error = %v", err)
	}
	if spawn.CharacterID != created.CharacterID || spawn.AccountID != login.AccountID {
		t.Fatalf("unexpected spawn result: %+v", spawn)
	}
	if spawn.Status.Vital != 100 || spawn.Status.MaxVital != 100 || spawn.Status.Level != 1 {
		t.Fatalf("unexpected spawn status: %+v", spawn.Status)
	}
	if len(spawn.Inventory) != 2 {
		t.Fatalf("unexpected initial inventory snapshot: %+v", spawn.Inventory)
	}
	if len(spawn.Equipment) != 1 || spawn.MapWearings[0] == 0 {
		t.Fatalf("unexpected initial equipment snapshot: equipment=%+v mapWearings=%+v", spawn.Equipment, spawn.MapWearings)
	}

	picked, err := zone.PickGroundItem(ctx, created.CharacterID, GroundItemSnapshot{ItemIndex: 90001, ItemVNUM: 7001, Endurance: 80, MaxEndurance: 100})
	if err != nil {
		t.Fatalf("PickGroundItem() error = %v", err)
	}
	if picked.InventoryType != "bag" || picked.SlotIndex == 0 || picked.ItemVNUM != 7001 {
		t.Fatalf("unexpected picked item: %+v", picked)
	}

	equipped, wearings, err := zone.EquipInventoryItem(ctx, created.CharacterID, picked.ItemIndex, 1)
	if err != nil {
		t.Fatalf("EquipInventoryItem() error = %v", err)
	}
	if equipped.EquipmentSlot != 1 || wearings[1] != int32(picked.ItemVNUM) {
		t.Fatalf("unexpected equipped item: equipment=%+v wearings=%+v", equipped, wearings)
	}

	dropped, err := zone.DropInventoryItem(ctx, created.CharacterID, picked.ItemIndex)
	if err != nil {
		t.Fatalf("DropInventoryItem() error = %v", err)
	}
	if dropped.ItemVNUM != picked.ItemVNUM || dropped.ItemIndex != picked.ItemIndex {
		t.Fatalf("unexpected dropped item: %+v", dropped)
	}

	if err := zone.SaveLogoutPosition(ctx, created.CharacterID, 2, 0, 100, 0, 200, 1.5); err != nil {
		t.Fatalf("SaveLogoutPosition() error = %v", err)
	}

	characters, err = chars.ListCharacters(ctx, login.AccountID)
	if err != nil {
		t.Fatalf("ListCharacters() after move error = %v", err)
	}
	if characters[0].MapID != 2 {
		t.Fatalf("map update mismatch: got %d want 2", characters[0].MapID)
	}
}

func TestCharacterLimitAndDelete(t *testing.T) {
	ctx := context.Background()
	auth, chars, _, _ := newTestServices()

	login, err := auth.Register(ctx, "bob", "secret", "127.0.0.1")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	for i := 0; i < 5; i++ {
		_, err := chars.CreateCharacter(ctx, CreateCharacterRequest{
			AccountID: login.AccountID,
			SlotIndex: uint8(i),
			Name:      "BobHero" + string(rune('A'+i)),
			Race:      2,
			Sex:       0,
			Hair:      0,
		})
		if err != nil {
			t.Fatalf("CreateCharacter(%d) error = %v", i, err)
		}
	}

	_, err = chars.CreateCharacter(ctx, CreateCharacterRequest{
		AccountID: login.AccountID,
		SlotIndex: 5,
		Name:      "BobOverflow",
		Race:      2,
		Sex:       0,
		Hair:      0,
	})
	if err != ErrCharacterLimit {
		t.Fatalf("CreateCharacter(limit) error = %v want %v", err, ErrCharacterLimit)
	}

	list, err := chars.ListCharacters(ctx, login.AccountID)
	if err != nil {
		t.Fatalf("ListCharacters() error = %v", err)
	}
	if err := chars.DeleteCharacter(ctx, login.AccountID, list[0].CharacterID); err != nil {
		t.Fatalf("DeleteCharacter() error = %v", err)
	}

	list, err = chars.ListCharacters(ctx, login.AccountID)
	if err != nil {
		t.Fatalf("ListCharacters() after delete error = %v", err)
	}
	if len(list) != 4 {
		t.Fatalf("character count after delete mismatch: got %d want 4", len(list))
	}
}

func TestDeleteCharacterFreesSlotForNewCharacter(t *testing.T) {
	ctx := context.Background()
	auth, chars, _, _ := newTestServices()

	login, err := auth.Register(ctx, "carol", "secret", "127.0.0.1")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	created, err := chars.CreateCharacter(ctx, CreateCharacterRequest{
		AccountID: login.AccountID,
		SlotIndex: 0,
		Name:      "CarolHero",
		Race:      2,
		Sex:       0,
		Hair:      0,
	})
	if err != nil {
		t.Fatalf("CreateCharacter() error = %v", err)
	}

	if err := chars.DeleteCharacter(ctx, login.AccountID, created.CharacterID); err != nil {
		t.Fatalf("DeleteCharacter() error = %v", err)
	}

	recreated, err := chars.CreateCharacter(ctx, CreateCharacterRequest{
		AccountID: login.AccountID,
		SlotIndex: 0,
		Name:      "CarolHeroTwo",
		Race:      2,
		Sex:       0,
		Hair:      1,
	})
	if err != nil {
		t.Fatalf("CreateCharacter(recreate) error = %v", err)
	}
	if recreated.SlotIndex != 0 {
		t.Fatalf("recreated slot mismatch: got %d want 0", recreated.SlotIndex)
	}
}
