package zone

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"laghaim-go/internal/protocol"
	"laghaim-go/internal/repo"
	"laghaim-go/internal/service"
	"laghaim-go/internal/world"
)

const (
	charTypePlayer byte = 0
	charTypeNPC    byte = 1
	charTypeItem   byte = 3

	clientInventoryPackCount  = 5
	clientInventoryPackWidth  = 8
	clientInventoryPackHeight = 6
	clientInventoryPackSize   = clientInventoryPackWidth * clientInventoryPackHeight
	clientQuickSlotCount      = 12
	clientWearingCount        = 14

	statusVital         = 0
	statusMana          = 1
	statusStamina       = 2
	statusEPower        = 3
	statusLevel         = 4
	statusMoney         = 10
	statusExp           = 11
	statusNeedExp       = 12
	statusLevelUpPoints = 13
	statusStrength      = 14
	statusIntelligence  = 15
	statusDexterity     = 16
	statusConstitution  = 17
	statusCharisma      = 18
)

type StaticNPC struct {
	Index  int32
	VNUM   int32
	MapID  uint32
	ZoneID uint32
	PosX   float32
	PosZ   float32
	Dir    float32
	Vital  int32
}

type StaticItem struct {
	Index        int32
	ItemVNUM     int32
	PlusPoint    int32
	SpecialFlag1 int32
	SpecialFlag2 int32
	MapID        uint32
	ZoneID       uint32
	PosX         float32
	PosZ         float32
	Dir          float32
	Timed        bool
}

type Config struct {
	StaticNPCs  []StaticNPC
	StaticItems []StaticItem
}

type Server struct {
	listener net.Listener
	codec    protocol.SeedCodec
	accounts repo.AccountRepository
	hasher   service.PasswordHasher
	handoffs *service.ZoneHandoffRegistry
	zone     service.ZoneEntryService
	world    *world.Runtime
	config   Config

	mu                  sync.Mutex
	clients             map[uint64]*clientConn
	groundItems         map[int32]StaticItem
	nextGroundItemIndex int32

	closeOnce sync.Once
	closed    chan struct{}
}

type clientConn struct {
	conn   net.Conn
	mu     sync.Mutex
	player world.Player
}

func NewServer(listener net.Listener, codec protocol.SeedCodec, accounts repo.AccountRepository, hasher service.PasswordHasher, handoffs *service.ZoneHandoffRegistry, zone service.ZoneEntryService, runtime *world.Runtime, config Config) *Server {
	if runtime == nil {
		runtime = world.NewRuntime()
	}
	if handoffs == nil {
		handoffs = service.NewZoneHandoffRegistry()
	}
	groundItems := make(map[int32]StaticItem, len(config.StaticItems))
	nextGroundItemIndex := int32(30000)
	for _, item := range config.StaticItems {
		groundItems[item.Index] = item
		if item.Index >= nextGroundItemIndex {
			nextGroundItemIndex = item.Index + 1
		}
	}

	return &Server{
		listener:            listener,
		codec:               codec,
		accounts:            accounts,
		hasher:              hasher,
		handoffs:            handoffs,
		zone:                zone,
		world:               runtime,
		config:              config,
		clients:             make(map[uint64]*clientConn),
		groundItems:         groundItems,
		nextGroundItemIndex: nextGroundItemIndex,
		closed:              make(chan struct{}),
	}
}

func (s *Server) Serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closed:
				return nil
			default:
			}
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.closed)
		err = s.listener.Close()
	})
	return err
}

type authState struct {
	step         string
	username     string
	account      repo.Account
	spawn        *service.OnlineSpawnResult
	client       *clientConn
	initialized  bool
	cursorItemID uint64
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	ctx := context.Background()
	state := &authState{}

	for {
		frame, err := protocol.ReadFrame(conn, s.codec)
		if err != nil {
			s.disconnect(ctx, state)
			return
		}

		if protocol.IsTextCommand(frame) {
			line := strings.TrimSpace(string(frame.Payload))
			if line == "" {
				continue
			}
			if _, err := strconv.Atoi(line); err == nil {
				continue // version frame
			}
			if state.spawn == nil {
				if err := s.handleAuthLine(ctx, conn, state, line); err != nil {
					s.disconnect(ctx, state)
					return
				}
			} else {
				if err := s.handleTextCommand(ctx, state, line); err != nil {
					s.disconnect(ctx, state)
					return
				}
			}
			continue
		}

		if state.spawn == nil || state.client == nil || !state.initialized {
			continue
		}
		if err := s.handleTypedPacket(state, frame); err != nil {
			s.disconnect(ctx, state)
			return
		}
	}
}

func (s *Server) handleAuthLine(ctx context.Context, conn net.Conn, state *authState, line string) error {
	switch state.step {
	case "":
		if line != "play" && line != "login" {
			return protocol.WriteTextCommand(conn, s.codec, "fail unexpected_command\n")
		}
		state.step = "username"
		return nil
	case "username":
		state.username = line
		state.step = "password"
		return nil
	case "password":
		password := parsePassword(line)
		account, err := s.accounts.GetAccountByUsername(ctx, state.username)
		if err != nil || !s.hasher.Verify(password, account.PasswordHash) {
			return protocol.WriteTextCommand(conn, s.codec, "fail invalid_credentials\n")
		}

		zoneTicket, ok := s.handoffs.Consume(account.ID)
		if !ok {
			return protocol.WriteTextCommand(conn, s.codec, "fail missing_handoff\n")
		}

		spawn, err := s.zone.EnterWorld(ctx, zoneTicket)
		if err != nil {
			if errors.Is(err, service.ErrCharacterNotFound) {
				return protocol.WriteTextCommand(conn, s.codec, "fail char_not_found\n")
			}
			return protocol.WriteTextCommand(conn, s.codec, "fail handoff_rejected\n")
		}

		player, _ := s.world.Join(spawn)
		client := &clientConn{conn: conn, player: player}
		state.account = account
		state.spawn = &spawn
		state.client = client

		s.mu.Lock()
		s.clients[player.CharacterID] = client
		s.mu.Unlock()

		return s.writeTextCommand(client, "OK\n")
	default:
		return protocol.WriteTextCommand(conn, s.codec, "fail invalid_auth_state\n")
	}
}

func (s *Server) handleTextCommand(ctx context.Context, state *authState, line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}

	if !state.initialized {
		switch fields[0] {
		case "start_game":
			if err := s.writeFrame(state.client, protocol.ResponseGameStart{Result: 0}.Frame()); err != nil {
				return err
			}
			if err := s.sendPlayerBootstrap(state.client, *state.spawn); err != nil {
				return err
			}
			if err := s.sendInitialState(state.client); err != nil {
				return err
			}
			for _, other := range s.world.Snapshot(state.client.player.MapID, state.client.player.ZoneID) {
				if other.CharacterID == state.client.player.CharacterID {
					continue
				}
				s.broadcastToPlayer(other.CharacterID, mapInCharFrame(state.client.player))
			}
			if err := s.writeFrame(state.client, protocol.ResponseGamePlayReady{Result: 0}.Frame()); err != nil {
				return err
			}
			state.initialized = true
			return s.writeTextCommand(state.client, "charloadcomplete\n")
		case "alive":
			return nil
		default:
			return nil
		}
	}

	switch fields[0] {
	case "alive":
		return nil
	case "pick":
		if len(fields) < 2 {
			return nil
		}
		itemIndex, err := strconv.ParseInt(fields[1], 10, 32)
		if err != nil {
			return nil
		}
		return s.handleTextItemPick(state, int32(itemIndex))
	case "inven":
		if len(fields) < 4 {
			return nil
		}
		pack, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil
		}
		slotX, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil
		}
		slotY, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil
		}
		return s.handleInventoryCommand(ctx, state, pack, slotX, slotY)
	case "wear":
		if len(fields) < 2 {
			return nil
		}
		where, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil
		}
		return s.handleWearCommand(ctx, state, uint8(where))
	case "drop":
		if len(fields) < 2 {
			return nil
		}
		return s.handleDropCommand(ctx, state)
	default:
		return nil
	}
}

func (s *Server) handleTextItemPick(state *authState, itemIndex int32) error {
	item, ok := s.takeGroundItem(itemIndex, state.client.player.MapID, state.client.player.ZoneID)
	if !ok {
		return nil
	}
	picked, err := s.zone.PickGroundItem(context.Background(), state.client.player.CharacterID, service.GroundItemSnapshot{
		ItemIndex:    uint64(item.Index),
		ItemVNUM:     uint32(item.ItemVNUM),
		PlusPoint:    item.PlusPoint,
		SpecialFlag1: item.SpecialFlag1,
		SpecialFlag2: item.SpecialFlag2,
		Endurance:    100,
		MaxEndurance: 100,
	})
	if err != nil {
		s.putGroundItem(item)
		return nil
	}
	outCommand := buildOutItemCommand(item.Index)
	if err := s.writeTextCommand(state.client, outCommand); err != nil {
		return err
	}
	s.broadcastVisibleText(state.client.player, outCommand)
	if command, ok := buildPickCommand(picked); ok {
		if err := s.writeTextCommand(state.client, command); err != nil {
			return err
		}
	}
	pickupCommand := buildPickupCommand(state.client.player.CharacterID)
	if err := s.writeTextCommand(state.client, pickupCommand); err != nil {
		return err
	}
	s.broadcastVisibleText(state.client.player, pickupCommand)
	return nil
}

func (s *Server) handleInventoryCommand(ctx context.Context, state *authState, pack, slotX, slotY int) error {
	slotIndex, ok := clientSlotIndex(pack, slotX, slotY)
	if !ok {
		return nil
	}
	if state.cursorItemID == 0 {
		item, found, err := s.zone.FindBagItemBySlot(ctx, state.client.player.CharacterID, slotIndex)
		if err != nil || !found {
			return err
		}
		state.cursorItemID = item.ItemIndex
		return nil
	}
	target, found, err := s.zone.FindBagItemBySlot(ctx, state.client.player.CharacterID, slotIndex)
	if err != nil {
		return err
	}
	if found {
		if target.ItemIndex == state.cursorItemID {
			state.cursorItemID = 0
		}
		return nil
	}
	if _, err := s.zone.MoveBagItem(ctx, state.client.player.CharacterID, state.cursorItemID, slotIndex); err != nil {
		if errors.Is(err, service.ErrInventorySlotTaken) {
			return nil
		}
		return err
	}
	state.cursorItemID = 0
	return nil
}

func (s *Server) handleWearCommand(ctx context.Context, state *authState, equipmentSlot uint8) error {
	if state.cursorItemID == 0 {
		removed, wearings, err := s.zone.UnequipSlot(ctx, state.client.player.CharacterID, equipmentSlot)
		if err != nil {
			if errors.Is(err, service.ErrItemNotFound) {
				return nil
			}
			return err
		}
		player, visible, err := s.world.UpdateWearings(state.client.player.CharacterID, wearings)
		if err != nil {
			return err
		}
		state.client.player = player
		command := buildCharRemoveCommand(player.CharacterID, removed.EquipmentSlot)
		for _, other := range visible {
			s.broadcastTextToPlayer(other.CharacterID, command)
		}
		return s.writeTextCommand(state.client, command)
	}
	equipment, wearings, err := s.zone.EquipInventoryItem(ctx, state.client.player.CharacterID, state.cursorItemID, equipmentSlot)
	if err != nil {
		if errors.Is(err, service.ErrItemNotFound) {
			state.cursorItemID = 0
			return nil
		}
		return err
	}
	player, visible, err := s.world.UpdateWearings(state.client.player.CharacterID, wearings)
	if err != nil {
		return err
	}
	state.client.player = player
	state.cursorItemID = 0
	command := buildCharWearCommand(player.CharacterID, equipment)
	for _, other := range visible {
		s.broadcastTextToPlayer(other.CharacterID, command)
	}
	return s.writeTextCommand(state.client, command)
}

func (s *Server) handleDropCommand(ctx context.Context, state *authState) error {
	if state.cursorItemID == 0 {
		return nil
	}
	dropped, err := s.zone.DropInventoryItem(ctx, state.client.player.CharacterID, state.cursorItemID)
	if err != nil {
		if errors.Is(err, service.ErrItemNotFound) {
			state.cursorItemID = 0
			return nil
		}
		return err
	}
	state.cursorItemID = 0
	item := StaticItem{
		Index:        int32(dropped.ItemIndex),
		ItemVNUM:     int32(dropped.ItemVNUM),
		PlusPoint:    dropped.PlusPoint,
		SpecialFlag1: dropped.SpecialFlag1,
		SpecialFlag2: dropped.SpecialFlag2,
		MapID:        state.client.player.MapID,
		ZoneID:       state.client.player.ZoneID,
		PosX:         float32(state.client.player.PosX),
		PosZ:         float32(state.client.player.PosZ),
		Dir:          float32(state.client.player.Direction),
	}
	if item.Index == 0 {
		item.Index = s.nextGroundIndex()
		item.PlusPoint = dropped.PlusPoint
		item.SpecialFlag1 = dropped.SpecialFlag1
		item.SpecialFlag2 = dropped.SpecialFlag2
		item.ItemVNUM = int32(dropped.ItemVNUM)
	}
	s.putGroundItem(item)
	command := buildDropCommand(item)
	s.broadcastVisibleText(state.client.player, command)
	return s.writeTextCommand(state.client, command)
}

func (s *Server) handleTypedPacket(state *authState, frame protocol.Frame) error {
	switch frame.SubHeader.Index {
	case protocol.ReqCharWalk:
		packet, err := protocol.DecodeRequestCharWalk(frame.Payload)
		if err != nil {
			return err
		}
		player, visible, err := s.world.Move(
			state.client.player.CharacterID,
			float64(packet.PosX),
			state.client.player.PosY,
			float64(packet.PosZ),
			state.client.player.Direction,
		)
		if err != nil {
			return err
		}
		state.client.player = player
		for _, other := range visible {
			s.broadcastToPlayer(other.CharacterID, charWalkFrame(player, packet.Run))
		}
	case protocol.ReqCharPlace:
		packet, err := protocol.DecodeRequestCharPlace(frame.Payload)
		if err != nil {
			return err
		}
		player, visible, err := s.world.Move(
			state.client.player.CharacterID,
			float64(packet.PosX),
			state.client.player.PosY,
			float64(packet.PosZ),
			float64(packet.Direction),
		)
		if err != nil {
			return err
		}
		state.client.player = player
		for _, other := range visible {
			s.broadcastToPlayer(other.CharacterID, charPlaceFrame(player, packet.Run))
		}
	case protocol.ReqCharStop:
		packet, err := protocol.DecodeRequestCharStop(frame.Payload)
		if err != nil {
			return err
		}
		player, visible, err := s.world.Move(
			state.client.player.CharacterID,
			float64(packet.PosX),
			state.client.player.PosY,
			float64(packet.PosZ),
			float64(packet.Direction),
		)
		if err != nil {
			return err
		}
		state.client.player = player
		for _, other := range visible {
			s.broadcastToPlayer(other.CharacterID, charStopFrame(player))
		}
	case protocol.ReqItemPick:
		packet, err := protocol.DecodeRequestItemPick(frame.Payload)
		if err != nil {
			return err
		}
		return s.handleItemPick(state, packet)
	case protocol.ReqItemDrop:
		packet, err := protocol.DecodeRequestItemDrop(frame.Payload)
		if err != nil {
			return err
		}
		return s.handleItemDrop(state, packet)
	case protocol.ReqWear:
		packet, err := protocol.DecodeRequestWear(frame.Payload)
		if err != nil {
			return err
		}
		return s.handleWear(state, packet)
	}
	return nil
}

func (s *Server) handleItemPick(state *authState, packet protocol.RequestItemPick) error {
	item, ok := s.takeGroundItem(packet.ItemIndex, state.client.player.MapID, state.client.player.ZoneID)
	if !ok {
		return nil
	}
	picked, err := s.zone.PickGroundItem(context.Background(), state.client.player.CharacterID, service.GroundItemSnapshot{
		ItemIndex:    uint64(item.Index),
		ItemVNUM:     uint32(item.ItemVNUM),
		PlusPoint:    item.PlusPoint,
		SpecialFlag1: item.SpecialFlag1,
		SpecialFlag2: item.SpecialFlag2,
		Endurance:    100,
		MaxEndurance: 100,
	})
	if err != nil {
		s.putGroundItem(item)
		return nil
	}
	pack, slotX, slotY, ok := inventoryPosition(picked.SlotIndex)
	if !ok {
		return nil
	}
	if err := s.writeFrame(state.client, protocol.ResponseItemPick{
		InventoryPack: int32(pack),
		SlotX:         int32(slotX),
		SlotY:         int32(slotY),
		Info: protocol.ItemInfo{
			ItemIndex:    int32(picked.ItemIndex),
			ItemVNUM:     int32(picked.ItemVNUM),
			PlusPoint:    picked.PlusPoint,
			SpecialFlag1: picked.SpecialFlag1,
			SpecialFlag2: picked.SpecialFlag2,
			Endurance:    picked.Endurance,
			EnduranceMax: picked.MaxEndurance,
		},
	}.Frame()); err != nil {
		return err
	}
	frame := itemPickFrame(state.client.player.CharacterID)
	s.broadcastVisible(state.client.player, frame)
	return s.writeFrame(state.client, frame)
}

func (s *Server) handleItemDrop(state *authState, packet protocol.RequestItemDrop) error {
	dropped, err := s.zone.DropInventoryItem(context.Background(), state.client.player.CharacterID, uint64(packet.ItemIndex))
	if err != nil {
		return err
	}
	item := StaticItem{
		Index:        int32(dropped.ItemIndex),
		ItemVNUM:     int32(dropped.ItemVNUM),
		PlusPoint:    dropped.PlusPoint,
		SpecialFlag1: dropped.SpecialFlag1,
		SpecialFlag2: dropped.SpecialFlag2,
		MapID:        state.client.player.MapID,
		ZoneID:       state.client.player.ZoneID,
		PosX:         float32(state.client.player.PosX),
		PosZ:         float32(state.client.player.PosZ),
		Dir:          float32(state.client.player.Direction),
	}
	if item.Index == 0 {
		item.Index = s.nextGroundIndex()
	}
	s.putGroundItem(item)
	frame := itemDropFrame(item)
	s.broadcastVisible(state.client.player, frame)
	return s.writeFrame(state.client, frame)
}

func (s *Server) handleWear(state *authState, packet protocol.RequestWear) error {
	if state.cursorItemID == 0 {
		return nil
	}
	equipment, wearings, err := s.zone.EquipInventoryItem(context.Background(), state.client.player.CharacterID, state.cursorItemID, uint8(packet.WearWhere))
	if err != nil {
		return err
	}
	player, visible, err := s.world.UpdateWearings(state.client.player.CharacterID, wearings)
	if err != nil {
		return err
	}
	state.client.player = player
	state.cursorItemID = 0
	frame := charWearFrame(player.CharacterID, equipment)
	for _, other := range visible {
		s.broadcastToPlayer(other.CharacterID, frame)
	}
	return s.writeFrame(state.client, frame)
}

func (s *Server) sendPlayerBootstrap(client *clientConn, spawn service.OnlineSpawnResult) error {
	commands := buildPlayerBootstrapCommands(client.player, spawn)
	for _, command := range commands {
		if err := s.writeTextCommand(client, command); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) sendInitialState(client *clientConn) error {
	for _, npc := range s.config.StaticNPCs {
		if npc.MapID == client.player.MapID && npc.ZoneID == client.player.ZoneID {
			if err := s.writeFrame(client, mapInNPCFrame(npc)); err != nil {
				return err
			}
		}
	}
	for _, item := range s.snapshotGroundItems(client.player.MapID, client.player.ZoneID) {
		if item.MapID == client.player.MapID && item.ZoneID == client.player.ZoneID {
			if err := s.writeFrame(client, mapInItemFrame(item)); err != nil {
				return err
			}
		}
	}
	for _, other := range s.world.Snapshot(client.player.MapID, client.player.ZoneID) {
		if other.CharacterID == client.player.CharacterID {
			continue
		}
		if err := s.writeFrame(client, mapInCharFrame(other)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) snapshotGroundItems(mapID, zoneID uint32) []StaticItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]StaticItem, 0, len(s.groundItems))
	for _, item := range s.groundItems {
		if item.MapID == mapID && item.ZoneID == zoneID {
			items = append(items, item)
		}
	}
	return items
}

func (s *Server) takeGroundItem(itemIndex int32, mapID, zoneID uint32) (StaticItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.groundItems[itemIndex]
	if !ok || item.MapID != mapID || item.ZoneID != zoneID {
		return StaticItem{}, false
	}
	delete(s.groundItems, itemIndex)
	return item, true
}

func (s *Server) putGroundItem(item StaticItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.groundItems[item.Index] = item
}

func (s *Server) nextGroundIndex() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.nextGroundItemIndex
	s.nextGroundItemIndex++
	return index
}

func (s *Server) broadcastVisible(player world.Player, frame protocol.Frame) {
	for _, other := range s.world.Snapshot(player.MapID, player.ZoneID) {
		if other.CharacterID == player.CharacterID {
			continue
		}
		s.broadcastToPlayer(other.CharacterID, frame)
	}
}

func (s *Server) disconnect(ctx context.Context, state *authState) {
	if state == nil || state.client == nil {
		return
	}

	s.mu.Lock()
	delete(s.clients, state.client.player.CharacterID)
	s.mu.Unlock()

	player, visible, err := s.world.Leave(state.client.player.CharacterID)
	if err != nil {
		return
	}
	_ = s.zone.SaveLogoutPosition(ctx, player.CharacterID, player.MapID, player.ZoneID, player.PosX, player.PosY, player.PosZ, player.Direction)
	for _, other := range visible {
		s.broadcastToPlayer(other.CharacterID, mapOutFrame(player.CharacterID, charTypePlayer))
	}
	state.client = nil
}

func (s *Server) broadcastToPlayer(characterID uint64, frame protocol.Frame) {
	s.mu.Lock()
	client := s.clients[characterID]
	s.mu.Unlock()
	if client == nil {
		return
	}
	_ = s.writeFrame(client, frame)
}

func (s *Server) broadcastTextToPlayer(characterID uint64, text string) {
	s.mu.Lock()
	client := s.clients[characterID]
	s.mu.Unlock()
	if client == nil {
		return
	}
	_ = s.writeTextCommand(client, text)
}

func (s *Server) broadcastVisibleText(player world.Player, text string) {
	for _, other := range s.world.Snapshot(player.MapID, player.ZoneID) {
		if other.CharacterID == player.CharacterID {
			continue
		}
		s.broadcastTextToPlayer(other.CharacterID, text)
	}
}

func (s *Server) writeFrame(client *clientConn, frame protocol.Frame) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	return protocol.WriteFrame(client.conn, s.codec, frame)
}

func (s *Server) writeTextCommand(client *clientConn, text string) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	return protocol.WriteTextCommand(client.conn, s.codec, text)
}

func parsePassword(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func buildPlayerBootstrapCommands(player world.Player, spawn service.OnlineSpawnResult) []string {
	commands := make([]string, 0, 32+len(spawn.Inventory))
	commands = append(commands,
		buildAtCommand(player),
		buildStatusCommand(statusVital, strconv.FormatUint(uint64(spawn.Status.Vital), 10), strconv.FormatUint(uint64(spawn.Status.MaxVital), 10)),
		buildStatusCommand(statusMana, strconv.FormatUint(uint64(spawn.Status.Mana), 10), strconv.FormatUint(uint64(spawn.Status.MaxMana), 10)),
		buildStatusCommand(statusStamina, strconv.FormatUint(uint64(spawn.Status.Stamina), 10), strconv.FormatUint(uint64(spawn.Status.MaxStamina), 10)),
		buildStatusCommand(statusEPower, strconv.FormatUint(uint64(spawn.Status.EPower), 10), strconv.FormatUint(uint64(spawn.Status.MaxEPower), 10)),
		buildStatusCommand(statusLevel, strconv.FormatUint(uint64(spawn.Status.Level), 10), "0"),
		buildStatusCommand(statusMoney, strconv.FormatUint(spawn.Status.Money, 10), "0"),
		buildStatusCommand(statusExp, strconv.FormatUint(spawn.Status.Experience, 10), "0"),
		buildStatusCommand(statusNeedExp, "0", "0"),
		buildStatusCommand(statusLevelUpPoints, strconv.FormatUint(uint64(spawn.Status.LevelUpPoints), 10), "0"),
		buildStatusCommand(statusStrength, strconv.FormatUint(uint64(spawn.Status.Strength), 10), "0"),
		buildStatusCommand(statusIntelligence, strconv.FormatUint(uint64(spawn.Status.Intelligence), 10), "0"),
		buildStatusCommand(statusDexterity, strconv.FormatUint(uint64(spawn.Status.Dexterity), 10), "0"),
		buildStatusCommand(statusConstitution, strconv.FormatUint(uint64(spawn.Status.Constitution), 10), "0"),
		buildStatusCommand(statusCharisma, strconv.FormatUint(uint64(spawn.Status.Charisma), 10), "0"),
		buildWearingCommand(spawn.Equipment),
		"lp 0\n",
	)
	for pack := 0; pack < clientInventoryPackCount; pack++ {
		commands = append(commands, "init_inven "+strconv.Itoa(pack)+"\n")
	}
	for _, item := range spawn.Inventory {
		switch item.InventoryType {
		case "bag":
			command, ok := buildInventoryCommand(item)
			if ok {
				commands = append(commands, command)
			}
		case "quickbar":
			command, ok := buildQuickCommand(item)
			if ok {
				commands = append(commands, command)
			}
		}
	}
	return commands
}

func buildAtCommand(player world.Player) string {
	return fmt.Sprintf("at %d %d %d %d 0\n", player.CharacterID, int(player.PosX), int(player.PosZ), int(player.PosY))
}

func buildStatusCommand(statusType int, value string, maxValue string) string {
	return fmt.Sprintf("status %d %s %s\n", statusType, value, maxValue)
}

func buildWearingCommand(equipment []service.EquipmentSnapshot) string {
	bySlot := make(map[uint8]service.EquipmentSnapshot, len(equipment))
	for _, item := range equipment {
		bySlot[item.EquipmentSlot] = item
	}

	var line strings.Builder
	line.WriteString("wearing")
	for slot := 0; slot < clientWearingCount; slot++ {
		item, ok := bySlot[uint8(slot)]
		if !ok {
			line.WriteString(" 0 0 0 0 0 0 0")
			continue
		}
		line.WriteString(fmt.Sprintf(
			" %d %d %d %d %d %d %d",
			item.ItemIndex,
			item.ItemVNUM,
			item.PlusPoint,
			item.SpecialFlag1,
			item.SpecialFlag2,
			item.Endurance,
			item.MaxEndurance,
		))
	}
	line.WriteString("\n")
	return line.String()
}

func clientSlotIndex(pack, x, y int) (uint32, bool) {
	if pack < 0 || pack >= clientInventoryPackCount {
		return 0, false
	}
	if x < 0 || x >= clientInventoryPackWidth || y < 0 || y >= clientInventoryPackHeight {
		return 0, false
	}
	return uint32(pack*clientInventoryPackSize + y*clientInventoryPackWidth + x), true
}

func inventoryPosition(slotIndex uint32) (int, int, int, bool) {
	pack := int(slotIndex) / clientInventoryPackSize
	if pack < 0 || pack >= clientInventoryPackCount {
		return 0, 0, 0, false
	}
	offset := int(slotIndex) % clientInventoryPackSize
	x := offset % clientInventoryPackWidth
	y := offset / clientInventoryPackWidth
	return pack, x, y, true
}

func buildInventoryCommand(item service.InventorySnapshot) (string, bool) {
	pack, x, y, ok := inventoryPosition(item.SlotIndex)
	if !ok {
		return "", false
	}
	return fmt.Sprintf(
		"inven %d %d %d %d %d %d %d %d %d %d\n",
		pack,
		item.ItemIndex,
		item.ItemVNUM,
		x,
		y,
		item.PlusPoint,
		item.SpecialFlag1,
		item.SpecialFlag2,
		item.Endurance,
		item.MaxEndurance,
	), true
}

func buildPickCommand(item service.InventorySnapshot) (string, bool) {
	pack, x, y, ok := inventoryPosition(item.SlotIndex)
	if !ok {
		return "", false
	}
	return fmt.Sprintf(
		"pick %d %d %d %d %d %d %d %d %d %d\n",
		item.ItemIndex,
		item.ItemVNUM,
		x,
		y,
		pack,
		item.PlusPoint,
		item.SpecialFlag1,
		item.SpecialFlag2,
		item.Endurance,
		item.MaxEndurance,
	), true
}

func buildDropCommand(item StaticItem) string {
	return fmt.Sprintf(
		"drop %d %d %d %d %d %g %d %d %d %d\n",
		item.Index,
		item.ItemVNUM,
		int(item.PosX),
		int(item.PosZ),
		0,
		item.Dir,
		item.PlusPoint,
		item.SpecialFlag1,
		item.SpecialFlag2,
		boolToInt(item.Timed),
	)
}

func buildPickupCommand(characterID uint64) string {
	return fmt.Sprintf("pickup %d\n", characterID)
}

func buildOutItemCommand(itemIndex int32) string {
	return fmt.Sprintf("out i %d\n", itemIndex)
}

func buildCharWearCommand(characterID uint64, equipment service.EquipmentSnapshot) string {
	return fmt.Sprintf("char_wear %d %d %d %d\n", characterID, equipment.EquipmentSlot, equipment.ItemVNUM, equipment.PlusPoint)
}

func buildCharRemoveCommand(characterID uint64, equipmentSlot uint8) string {
	return fmt.Sprintf("char_remove %d %d\n", characterID, equipmentSlot)
}

func buildQuickCommand(item service.InventorySnapshot) (string, bool) {
	if item.SlotIndex >= clientQuickSlotCount {
		return "", false
	}
	return fmt.Sprintf(
		"quick %d %d %d %d %d %d\n",
		item.SlotIndex,
		item.ItemIndex,
		item.ItemVNUM,
		item.PlusPoint,
		item.SpecialFlag1,
		item.SpecialFlag2,
	), true
}

func mapInCharFrame(player world.Player) protocol.Frame {
	return protocol.UpdateMapInChar{
		Type:        0,
		CharacterID: int32(player.CharacterID),
		Name:        player.Name,
		Race:        int32(player.Race),
		Sex:         int32(player.Sex),
		Hair:        int32(player.Hair),
		PosX:        float32(player.PosX),
		PosZ:        float32(player.PosZ),
		Direction:   float32(player.Direction),
		Wearings:    player.Wearings,
		Vital:       100,
	}.Frame()
}

func mapInNPCFrame(npc StaticNPC) protocol.Frame {
	return protocol.UpdateMapInNPC{
		NPCIndex:  npc.Index,
		NPCVNUM:   npc.VNUM,
		PosX:      npc.PosX,
		PosZ:      npc.PosZ,
		Direction: npc.Dir,
		Vital:     npc.Vital,
	}.Frame()
}

func mapInItemFrame(item StaticItem) protocol.Frame {
	return protocol.UpdateMapInItem{
		Info: protocol.ItemInfo{
			ItemIndex:    item.Index,
			ItemVNUM:     item.ItemVNUM,
			PlusPoint:    item.PlusPoint,
			SpecialFlag1: item.SpecialFlag1,
			SpecialFlag2: item.SpecialFlag2,
			Endurance:    100,
			EnduranceMax: 100,
		},
		TimedItem: item.Timed,
		PosX:      item.PosX,
		PosZ:      item.PosZ,
		Direction: item.Dir,
	}.Frame()
}

func itemDropFrame(item StaticItem) protocol.Frame {
	return protocol.UpdateItemDrop{
		PosX:      item.PosX,
		PosZ:      item.PosZ,
		Direction: item.Dir,
		Info: protocol.ItemInfo{
			ItemIndex:    item.Index,
			ItemVNUM:     item.ItemVNUM,
			PlusPoint:    item.PlusPoint,
			SpecialFlag1: item.SpecialFlag1,
			SpecialFlag2: item.SpecialFlag2,
			Endurance:    100,
			EnduranceMax: 100,
		},
	}.Frame()
}

func itemPickFrame(characterID uint64) protocol.Frame {
	return protocol.UpdateItemPick{CharacterID: int32(characterID)}.Frame()
}

func charWearFrame(characterID uint64, equipment service.EquipmentSnapshot) protocol.Frame {
	return protocol.UpdateCharWear{
		CharacterID:   int32(characterID),
		WearWhere:     int32(equipment.EquipmentSlot),
		ItemVNUM:      int32(equipment.ItemVNUM),
		ItemPlusPoint: equipment.PlusPoint,
	}.Frame()
}

func mapOutFrame(characterID uint64, charType byte) protocol.Frame {
	return protocol.UpdateMapOut{CharType: charType, TargetID: int32(characterID)}.Frame()
}

func charWalkFrame(player world.Player, run bool) protocol.Frame {
	return protocol.UpdateCharWalk{
		CharType:    charTypePlayer,
		TargetIndex: int32(player.CharacterID),
		PosX:        float32(player.PosX),
		PosZ:        float32(player.PosZ),
		Run:         run,
	}.Frame()
}

func charPlaceFrame(player world.Player, run bool) protocol.Frame {
	return protocol.UpdateCharPlace{
		CharType:    charTypePlayer,
		TargetIndex: int32(player.CharacterID),
		PosX:        float32(player.PosX),
		PosZ:        float32(player.PosZ),
		Direction:   float32(player.Direction),
		RemainFrame: 0,
		Run:         run,
	}.Frame()
}

func charStopFrame(player world.Player) protocol.Frame {
	return protocol.UpdateCharStop{
		CharType:    charTypePlayer,
		TargetIndex: int32(player.CharacterID),
		PosX:        float32(player.PosX),
		PosZ:        float32(player.PosZ),
		Direction:   float32(player.Direction),
	}.Frame()
}
