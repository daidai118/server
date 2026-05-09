package zone

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
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
	Index    int32
	ItemVNUM int32
	MapID    uint32
	ZoneID   uint32
	PosX     float32
	PosZ     float32
	Dir      float32
	Timed    bool
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

	mu      sync.Mutex
	clients map[uint64]*clientConn

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
	return &Server{
		listener: listener,
		codec:    codec,
		accounts: accounts,
		hasher:   hasher,
		handoffs: handoffs,
		zone:     zone,
		world:    runtime,
		config:   config,
		clients:  make(map[uint64]*clientConn),
		closed:   make(chan struct{}),
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
	step     string
	username string
	account  repo.Account
	spawn    *service.OnlineSpawnResult
	client   *clientConn
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
			}
			continue
		}

		if state.spawn == nil || state.client == nil {
			continue
		}
		if err := s.handleTypedPacket(ctx, state, frame); err != nil {
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

		player, visible := s.world.Join(spawn)
		client := &clientConn{conn: conn, player: player}
		state.account = account
		state.spawn = &spawn
		state.client = client

		s.mu.Lock()
		s.clients[player.CharacterID] = client
		s.mu.Unlock()

		if err := s.sendInitialState(client, visible); err != nil {
			return err
		}
		for _, other := range visible {
			s.broadcastToPlayer(other.CharacterID, mapInCharFrame(player))
		}
		return nil
	default:
		return protocol.WriteTextCommand(conn, s.codec, "fail invalid_auth_state\n")
	}
}

func (s *Server) handleTypedPacket(_ context.Context, state *authState, frame protocol.Frame) error {
	switch frame.SubHeader.Index {
	case protocol.ReqCharWalk:
		posX, posZ, _, err := decodeWalk(frame.Payload)
		if err != nil {
			return err
		}
		player, visible, err := s.world.Move(state.client.player.CharacterID, posX, state.client.player.PosY, posZ, state.client.player.Direction)
		if err != nil {
			return err
		}
		state.client.player = player
		for _, other := range visible {
			s.broadcastToPlayer(other.CharacterID, charWalkFrame(player, true))
		}
	case protocol.ReqCharPlace:
		posX, posZ, dir, run, err := decodePlace(frame.Payload)
		if err != nil {
			return err
		}
		player, visible, err := s.world.Move(state.client.player.CharacterID, posX, state.client.player.PosY, posZ, dir)
		if err != nil {
			return err
		}
		state.client.player = player
		for _, other := range visible {
			s.broadcastToPlayer(other.CharacterID, charPlaceFrame(player, run))
		}
	case protocol.ReqCharStop:
		posX, posZ, dir, err := decodeStop(frame.Payload)
		if err != nil {
			return err
		}
		player, visible, err := s.world.Move(state.client.player.CharacterID, posX, state.client.player.PosY, posZ, dir)
		if err != nil {
			return err
		}
		state.client.player = player
		for _, other := range visible {
			s.broadcastToPlayer(other.CharacterID, charStopFrame(player))
		}
	}
	return nil
}

func (s *Server) sendInitialState(client *clientConn, visible []world.Player) error {
	for _, npc := range s.config.StaticNPCs {
		if npc.MapID == client.player.MapID && npc.ZoneID == client.player.ZoneID {
			if err := s.writeFrame(client, mapInNPCFrame(npc)); err != nil {
				return err
			}
		}
	}
	for _, item := range s.config.StaticItems {
		if item.MapID == client.player.MapID && item.ZoneID == client.player.ZoneID {
			if err := s.writeFrame(client, mapInItemFrame(item)); err != nil {
				return err
			}
		}
	}
	for _, other := range visible {
		if err := s.writeFrame(client, mapInCharFrame(other)); err != nil {
			return err
		}
	}
	return nil
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

func (s *Server) writeFrame(client *clientConn, frame protocol.Frame) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	return protocol.WriteFrame(client.conn, s.codec, frame)
}

func parsePassword(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func decodeWalk(body []byte) (float64, float64, bool, error) {
	if len(body) < 9 {
		return 0, 0, false, protocol.ErrFrameTooShort
	}
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(body[0:4]))), float64(math.Float32frombits(binary.LittleEndian.Uint32(body[4:8]))), body[8] != 0, nil
}

func decodePlace(body []byte) (float64, float64, float64, bool, error) {
	if len(body) < 18 {
		return 0, 0, 0, false, protocol.ErrFrameTooShort
	}
	posX := math.Float32frombits(binary.LittleEndian.Uint32(body[1:5]))
	posZ := math.Float32frombits(binary.LittleEndian.Uint32(body[5:9]))
	dir := math.Float32frombits(binary.LittleEndian.Uint32(body[9:13]))
	run := body[17] != 0
	return float64(posX), float64(posZ), float64(dir), run, nil
}

func decodeStop(body []byte) (float64, float64, float64, error) {
	if len(body) < 13 {
		return 0, 0, 0, protocol.ErrFrameTooShort
	}
	posX := math.Float32frombits(binary.LittleEndian.Uint32(body[1:5]))
	posZ := math.Float32frombits(binary.LittleEndian.Uint32(body[5:9]))
	dir := math.Float32frombits(binary.LittleEndian.Uint32(body[9:13]))
	return float64(posX), float64(posZ), float64(dir), nil
}

func mapInCharFrame(player world.Player) protocol.Frame {
	var body bytes.Buffer
	body.WriteByte(0)
	writeInt32(&body, int32(player.CharacterID))
	writeFixedString(&body, player.Name, protocol.UserIDLength)
	writeInt32(&body, int32(player.Race))
	writeInt32(&body, int32(player.Sex))
	writeInt32(&body, int32(player.Hair))
	writeFloat32(&body, float32(player.PosX))
	writeFloat32(&body, float32(player.PosZ))
	writeFloat32(&body, float32(player.Direction))
	for i := 0; i < protocol.CharMapInWearCount; i++ {
		writeInt32(&body, 0)
	}
	writeInt32(&body, 100)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeFixedString(&body, "", protocol.GuildNameLength)
	writeFixedString(&body, "", protocol.GuildGradeLength)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	return protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.UpdMapInChar, Type: protocol.PacketTypeUpdate}, Payload: body.Bytes()}
}

func mapInNPCFrame(npc StaticNPC) protocol.Frame {
	var body bytes.Buffer
	writeInt32(&body, npc.Index)
	writeInt32(&body, npc.VNUM)
	writeFloat32(&body, npc.PosX)
	writeFloat32(&body, npc.PosZ)
	writeFloat32(&body, npc.Dir)
	writeInt32(&body, 0)
	writeInt32(&body, npc.Vital)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	return protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.UpdMapInNpc, Type: protocol.PacketTypeUpdate}, Payload: body.Bytes()}
}

func mapInItemFrame(item StaticItem) protocol.Frame {
	var body bytes.Buffer
	writeInt32(&body, item.Index)
	writeInt32(&body, item.ItemVNUM)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeInt32(&body, 0)
	writeBool(&body, item.Timed)
	writeFloat32(&body, item.PosX)
	writeFloat32(&body, item.PosZ)
	writeFloat32(&body, item.Dir)
	return protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.UpdMapInItem, Type: protocol.PacketTypeUpdate}, Payload: body.Bytes()}
}

func mapOutFrame(characterID uint64, charType byte) protocol.Frame {
	var body bytes.Buffer
	body.WriteByte(charType)
	writeInt32(&body, int32(characterID))
	return protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.UpdMapOut, Type: protocol.PacketTypeUpdate}, Payload: body.Bytes()}
}

func charWalkFrame(player world.Player, run bool) protocol.Frame {
	var body bytes.Buffer
	body.WriteByte(charTypePlayer)
	writeInt32(&body, int32(player.CharacterID))
	writeFloat32(&body, float32(player.PosX))
	writeFloat32(&body, float32(player.PosZ))
	writeBool(&body, run)
	return protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.UpdCharWalk, Type: protocol.PacketTypeUpdate}, Payload: body.Bytes()}
}

func charPlaceFrame(player world.Player, run bool) protocol.Frame {
	var body bytes.Buffer
	body.WriteByte(charTypePlayer)
	writeInt32(&body, int32(player.CharacterID))
	writeFloat32(&body, float32(player.PosX))
	writeFloat32(&body, float32(player.PosZ))
	writeFloat32(&body, float32(player.Direction))
	writeInt32(&body, 0)
	writeBool(&body, run)
	return protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.UpdCharPlace, Type: protocol.PacketTypeUpdate}, Payload: body.Bytes()}
}

func charStopFrame(player world.Player) protocol.Frame {
	var body bytes.Buffer
	body.WriteByte(charTypePlayer)
	writeInt32(&body, int32(player.CharacterID))
	writeFloat32(&body, float32(player.PosX))
	writeFloat32(&body, float32(player.PosZ))
	writeFloat32(&body, float32(player.Direction))
	return protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.UpdCharStop, Type: protocol.PacketTypeUpdate}, Payload: body.Bytes()}
}

func writeInt32(buf *bytes.Buffer, value int32) { _ = binary.Write(buf, binary.LittleEndian, value) }
func writeFloat32(buf *bytes.Buffer, value float32) {
	_ = binary.Write(buf, binary.LittleEndian, value)
}
func writeBool(buf *bytes.Buffer, value bool) {
	if value {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
}
func writeFixedString(buf *bytes.Buffer, value string, size int) {
	data := make([]byte, size)
	copy(data, []byte(value))
	buf.Write(data)
}
