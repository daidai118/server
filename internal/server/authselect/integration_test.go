package authselect_test

import (
	"encoding/binary"
	"math"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"laghaim-go/internal/protocol"
	"laghaim-go/internal/repo/memory"
	authselectserver "laghaim-go/internal/server/authselect"
	zoneserver "laghaim-go/internal/server/zone"
	"laghaim-go/internal/service"
	"laghaim-go/internal/session"
	"laghaim-go/internal/world"
)

func TestGatewayAndZoneIntegration(t *testing.T) {
	store := memory.NewStore()
	sessions := session.NewManager()
	handoffs := service.NewZoneHandoffRegistry()
	hasher := service.DefaultPasswordHasher()
	auth := service.NewAuthService(store, sessions, hasher, service.AuthConfig{GMSTicketTTL: time.Minute})
	chars := service.NewCharacterService(store, store, store, sessions, handoffs, service.CharacterConfig{ZoneTicketTTL: time.Minute})
	zoneSvc := service.NewZoneEntryService(store, sessions)
	codec := protocol.MustNewDefaultSeedCodec()

	zoneListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(zone) error = %v", err)
	}
	defer zoneListener.Close()
	zoneHost, zonePortRaw, _ := strings.Cut(zoneListener.Addr().String(), ":")

	zoneSrv := zoneserver.NewServer(zoneListener, codec, store, hasher, handoffs, zoneSvc, world.NewRuntime(), zoneserver.Config{
		StaticNPCs:  []zoneserver.StaticNPC{{Index: 10001, VNUM: 2001, MapID: 1, ZoneID: 0, PosX: 33100, PosZ: 33100, Dir: 0, Vital: 100}},
		StaticItems: []zoneserver.StaticItem{{Index: 20001, ItemVNUM: 5001, MapID: 1, ZoneID: 0, PosX: 32950, PosZ: 32950, Dir: 0}},
	})
	go func() { _ = zoneSrv.Serve() }()
	defer zoneSrv.Close()

	gatewayListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(gateway) error = %v", err)
	}
	defer gatewayListener.Close()
	gatewaySrv := authselectserver.NewServer(gatewayListener, codec, auth, chars, authselectserver.Config{ZoneHost: zoneHost, ZonePort: mustAtoi(zonePortRaw)})
	go func() { _ = gatewaySrv.Serve() }()
	defer gatewaySrv.Close()

	client1WorldAddr := registerCreateStart(t, gatewayListener.Addr().String(), codec, "alice", "secret", "AliceHero")
	worldConn1 := connectAndEnterWorld(t, client1WorldAddr, codec, "alice", "secret")
	defer worldConn1.Close()

	if frame := readFrameByOpcode(t, worldConn1, codec, protocol.UpdMapInNpc); frame.SubHeader.Index != protocol.UpdMapInNpc {
		t.Fatalf("expected UpdMapInNpc, got %+v", frame.SubHeader)
	}
	if frame := readFrameByOpcode(t, worldConn1, codec, protocol.UpdMapInItem); frame.SubHeader.Index != protocol.UpdMapInItem {
		t.Fatalf("expected UpdMapInItem, got %+v", frame.SubHeader)
	}

	client2WorldAddr := registerCreateStart(t, gatewayListener.Addr().String(), codec, "bob", "secret", "BobHero")
	worldConn2 := connectAndEnterWorld(t, client2WorldAddr, codec, "bob", "secret")
	defer worldConn2.Close()

	frame := readFrameByOpcode(t, worldConn2, codec, protocol.UpdMapInChar)
	if got, want := decodeCharacterID(frame.Payload[1:5]), int32(1); got != want {
		t.Fatalf("second client should see first player character id %d, got %d", want, got)
	}

	frame = readFrameByOpcode(t, worldConn1, codec, protocol.UpdMapInChar)
	if got, want := decodeCharacterID(frame.Payload[1:5]), int32(2); got != want {
		t.Fatalf("first client should see second player character id %d, got %d", want, got)
	}

	sendWalk(t, worldConn2, codec, 34000, 34100, true)
	frame = readFrameByOpcode(t, worldConn1, codec, protocol.UpdCharWalk)
	if got, want := decodeCharacterID(frame.Payload[1:5]), int32(2); got != want {
		t.Fatalf("walk broadcast character mismatch: got %d want %d", got, want)
	}

	_ = worldConn2.Close()
	frame = readFrameByOpcode(t, worldConn1, codec, protocol.UpdMapOut)
	if got, want := decodeCharacterID(frame.Payload[1:5]), int32(2); got != want {
		t.Fatalf("map out character mismatch: got %d want %d", got, want)
	}
}

func registerCreateStart(t *testing.T, gatewayAddr string, codec protocol.SeedCodec, username, password, characterName string) string {
	t.Helper()
	conn, err := net.Dial("tcp", gatewayAddr)
	if err != nil {
		t.Fatalf("net.Dial(gateway) error = %v", err)
	}
	defer conn.Close()

	sendText(t, conn, codec, "100\n")
	sendText(t, conn, codec, "register\n")
	sendText(t, conn, codec, username+"\n")
	sendText(t, conn, codec, password+" d n 0\n")

	if got := readText(t, conn, codec); got != "chars_start" {
		t.Fatalf("expected chars_start, got %q", got)
	}
	if got := readText(t, conn, codec); got != "chars_end 0 0" {
		t.Fatalf("expected chars_end, got %q", got)
	}

	sendText(t, conn, codec, "char_exist "+characterName+"\n")
	if got := readText(t, conn, codec); got != "success" {
		t.Fatalf("expected char_exist success, got %q", got)
	}

	sendText(t, conn, codec, "char_new 0 "+characterName+" 2 0 0 10 10 12 10 8 0\n")
	if got := readText(t, conn, codec); got != "success" {
		t.Fatalf("expected char_new success, got %q", got)
	}

	sendText(t, conn, codec, "chars\n")
	if got := readText(t, conn, codec); got != "chars_start" {
		t.Fatalf("expected chars_start after chars command, got %q", got)
	}
	charLine := readText(t, conn, codec)
	if !strings.HasPrefix(charLine, "chars_exist 0 ") {
		t.Fatalf("expected chars_exist line, got %q", charLine)
	}
	if got := readText(t, conn, codec); got != "chars_end 0 0" {
		t.Fatalf("expected chars_end after chars command, got %q", got)
	}

	sendText(t, conn, codec, "start 0 0 1 0 1 1 1 1\n")
	goWorld := readText(t, conn, codec)
	if !strings.HasPrefix(goWorld, "go_world ") {
		t.Fatalf("expected go_world, got %q", goWorld)
	}
	fields := strings.Fields(goWorld)
	if len(fields) != 5 {
		t.Fatalf("unexpected go_world format: %q", goWorld)
	}
	return fields[1] + ":" + fields[2]
}

func connectAndEnterWorld(t *testing.T, worldAddr string, codec protocol.SeedCodec, username, password string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", worldAddr)
	if err != nil {
		t.Fatalf("net.Dial(world) error = %v", err)
	}
	sendText(t, conn, codec, "100\n")
	sendText(t, conn, codec, "play\n")
	sendText(t, conn, codec, username+"\n")
	sendText(t, conn, codec, password+" d n 0\n")
	return conn
}

func sendText(t *testing.T, conn net.Conn, codec protocol.SeedCodec, text string) {
	t.Helper()
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := protocol.WriteTextCommand(conn, codec, text); err != nil {
		t.Fatalf("WriteTextCommand(%q) error = %v", text, err)
	}
}

func readText(t *testing.T, conn net.Conn, codec protocol.SeedCodec) string {
	t.Helper()
	frame := readFrame(t, conn, codec)
	if !protocol.IsTextCommand(frame) {
		t.Fatalf("expected text frame, got %+v", frame.SubHeader)
	}
	return strings.TrimSpace(string(frame.Payload))
}

func readFrameByOpcode(t *testing.T, conn net.Conn, codec protocol.SeedCodec, opcode protocol.Opcode) protocol.Frame {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		frame := readFrame(t, conn, codec)
		if frame.SubHeader.Index == opcode {
			return frame
		}
	}
	t.Fatalf("timed out waiting for opcode %d", opcode)
	return protocol.Frame{}
}

func readFrame(t *testing.T, conn net.Conn, codec protocol.SeedCodec) protocol.Frame {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	frame, err := protocol.ReadFrame(conn, codec)
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	return frame
}

func sendWalk(t *testing.T, conn net.Conn, codec protocol.SeedCodec, posX, posZ float32, run bool) {
	t.Helper()
	body := make([]byte, 9)
	binary.LittleEndian.PutUint32(body[0:4], math32bits(posX))
	binary.LittleEndian.PutUint32(body[4:8], math32bits(posZ))
	if run {
		body[8] = 1
	}
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := protocol.WriteFrame(conn, codec, protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.ReqCharWalk, Type: protocol.PacketTypeRequest}, Payload: body}); err != nil {
		t.Fatalf("WriteFrame(walk) error = %v", err)
	}
}

func decodeCharacterID(body []byte) int32 {
	return int32(binary.LittleEndian.Uint32(body))
}

func mustAtoi(raw string) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		panic(err)
	}
	return value
}

func math32bits(v float32) uint32 { return math.Float32bits(v) }
