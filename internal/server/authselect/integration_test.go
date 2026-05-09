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

type testCluster struct {
	gatewayAddr string
	worldAddr   string
	codec       protocol.SeedCodec
}

func startTestCluster(t *testing.T) testCluster {
	t.Helper()

	store := memory.NewStore()
	sessions := session.NewManager()
	handoffs := service.NewZoneHandoffRegistry()
	hasher := service.DefaultPasswordHasher()
	auth := service.NewAuthService(store, sessions, hasher, service.AuthConfig{GMSTicketTTL: time.Minute})
	chars := service.NewCharacterService(store, store, store, sessions, handoffs, service.CharacterConfig{ZoneTicketTTL: time.Minute})
	zoneSvc := service.NewZoneEntryService(store, store, store, sessions)
	codec := protocol.MustNewDefaultSeedCodec()

	zoneListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(zone) error = %v", err)
	}
	zoneHost, zonePortRaw, _ := strings.Cut(zoneListener.Addr().String(), ":")
	zoneSrv := zoneserver.NewServer(zoneListener, codec, store, hasher, handoffs, zoneSvc, world.NewRuntime(), zoneserver.Config{
		StaticNPCs:  []zoneserver.StaticNPC{{Index: 10001, VNUM: 2001, MapID: 1, ZoneID: 0, PosX: 33100, PosZ: 33100, Dir: 0, Vital: 100}},
		StaticItems: []zoneserver.StaticItem{{Index: 20001, ItemVNUM: 5001, MapID: 1, ZoneID: 0, PosX: 32950, PosZ: 32950, Dir: 0}},
	})
	go func() { _ = zoneSrv.Serve() }()
	t.Cleanup(func() {
		_ = zoneSrv.Close()
		_ = zoneListener.Close()
	})

	gatewayListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(gateway) error = %v", err)
	}
	gatewaySrv := authselectserver.NewServer(gatewayListener, codec, auth, chars, authselectserver.Config{ZoneHost: zoneHost, ZonePort: mustAtoi(zonePortRaw)})
	go func() { _ = gatewaySrv.Serve() }()
	t.Cleanup(func() {
		_ = gatewaySrv.Close()
		_ = gatewayListener.Close()
	})

	return testCluster{
		gatewayAddr: gatewayListener.Addr().String(),
		worldAddr:   zoneListener.Addr().String(),
		codec:       codec,
	}
}

func TestGatewayAndZoneIntegration(t *testing.T) {
	cluster := startTestCluster(t)

	client1WorldAddr := registerCreateStart(t, cluster.gatewayAddr, cluster.codec, "alice", "secret", "AliceHero")
	worldConn1 := connectAndEnterWorld(t, client1WorldAddr, cluster.codec, "alice", "secret")
	defer worldConn1.Close()

	if frame := readFrameByOpcode(t, worldConn1, cluster.codec, protocol.UpdMapInNpc); frame.SubHeader.Index != protocol.UpdMapInNpc {
		t.Fatalf("expected UpdMapInNpc, got %+v", frame.SubHeader)
	}
	if frame := readFrameByOpcode(t, worldConn1, cluster.codec, protocol.UpdMapInItem); frame.SubHeader.Index != protocol.UpdMapInItem {
		t.Fatalf("expected UpdMapInItem, got %+v", frame.SubHeader)
	}
	sendItemPick(t, worldConn1, cluster.codec, 20001)
	pickResponse := readFrameByOpcode(t, worldConn1, cluster.codec, protocol.ResItemPick)
	if pickResponse.SubHeader.Index != protocol.ResItemPick {
		t.Fatalf("expected ResItemPick, got %+v", pickResponse.SubHeader)
	}
	if frame := readFrameByOpcode(t, worldConn1, cluster.codec, protocol.UpdItemPick); frame.SubHeader.Index != protocol.UpdItemPick {
		t.Fatalf("expected UpdItemPick, got %+v", frame.SubHeader)
	}
	pickedItemIndex := parseItemIndexFromPickResponse(t, pickResponse.Payload)
	sendItemDrop(t, worldConn1, cluster.codec, pickedItemIndex)
	if frame := readFrameByOpcode(t, worldConn1, cluster.codec, protocol.UpdItemDrop); frame.SubHeader.Index != protocol.UpdItemDrop {
		t.Fatalf("expected UpdItemDrop, got %+v", frame.SubHeader)
	}

	client2WorldAddr := registerCreateStart(t, cluster.gatewayAddr, cluster.codec, "bob", "secret", "BobHero")
	worldConn2 := connectAndEnterWorld(t, client2WorldAddr, cluster.codec, "bob", "secret")
	defer worldConn2.Close()

	frame := readFrameByOpcode(t, worldConn2, cluster.codec, protocol.UpdMapInChar)
	if got, want := decodeCharacterID(frame.Payload[1:5]), int32(1); got != want {
		t.Fatalf("second client should see first player character id %d, got %d", want, got)
	}

	frame = readFrameByOpcode(t, worldConn1, cluster.codec, protocol.UpdMapInChar)
	if got, want := decodeCharacterID(frame.Payload[1:5]), int32(2); got != want {
		t.Fatalf("first client should see second player character id %d, got %d", want, got)
	}

	sendWalk(t, worldConn2, cluster.codec, 34000, 34100, true)
	frame = readFrameByOpcode(t, worldConn1, cluster.codec, protocol.UpdCharWalk)
	if got, want := decodeCharacterID(frame.Payload[1:5]), int32(2); got != want {
		t.Fatalf("walk broadcast character mismatch: got %d want %d", got, want)
	}

	_ = worldConn2.Close()
	frame = readFrameByOpcode(t, worldConn1, cluster.codec, protocol.UpdMapOut)
	if got, want := decodeCharacterID(frame.Payload[1:5]), int32(2); got != want {
		t.Fatalf("map out character mismatch: got %d want %d", got, want)
	}
}

func TestZoneTextItemCommands(t *testing.T) {
	cluster := startTestCluster(t)

	client1WorldAddr := registerCreateStart(t, cluster.gatewayAddr, cluster.codec, "alice-text", "secret", "AliceTextHero")
	worldConn1 := connectAndLoadWorld(t, client1WorldAddr, cluster.codec, "alice-text", "secret")
	defer worldConn1.Close()

	client2WorldAddr := registerCreateStart(t, cluster.gatewayAddr, cluster.codec, "bob-text", "secret", "BobTextHero")
	worldConn2 := connectAndLoadWorld(t, client2WorldAddr, cluster.codec, "bob-text", "secret")
	defer worldConn2.Close()

	sendText(t, worldConn1, cluster.codec, "pick 20001\n")
	if got := readTextByPrefix(t, worldConn1, cluster.codec, "out i 20001"); got != "out i 20001" {
		t.Fatalf("pick out command = %q, want %q", got, "out i 20001")
	}
	pickLine := readTextByPrefix(t, worldConn1, cluster.codec, "pick ")
	if got := readTextByPrefix(t, worldConn1, cluster.codec, "pickup 1"); got != "pickup 1" {
		t.Fatalf("pickup effect = %q, want %q", got, "pickup 1")
	}
	if got := readTextByPrefix(t, worldConn2, cluster.codec, "out i 20001"); got != "out i 20001" {
		t.Fatalf("observer out command = %q, want %q", got, "out i 20001")
	}
	if got := readTextByPrefix(t, worldConn2, cluster.codec, "pickup 1"); got != "pickup 1" {
		t.Fatalf("observer pickup effect = %q, want %q", got, "pickup 1")
	}

	pickFields := strings.Fields(pickLine)
	if len(pickFields) < 6 {
		t.Fatalf("unexpected pick line: %q", pickLine)
	}
	sendText(t, worldConn1, cluster.codec, "inven "+pickFields[5]+" "+pickFields[3]+" "+pickFields[4]+"\n")
	sendText(t, worldConn1, cluster.codec, "wear 1\n")
	if got := readTextByPrefix(t, worldConn2, cluster.codec, "char_wear 1 1 "); !strings.HasPrefix(got, "char_wear 1 1 ") {
		t.Fatalf("observer wear update = %q", got)
	}

	sendText(t, worldConn1, cluster.codec, "wear 1\n")
	if got := readTextByPrefix(t, worldConn2, cluster.codec, "char_remove 1 1"); got != "char_remove 1 1" {
		t.Fatalf("observer wear remove = %q, want %q", got, "char_remove 1 1")
	}
}

func TestZoneBootstrapIncludesSelfStateCommands(t *testing.T) {
	cluster := startTestCluster(t)

	worldAddr := registerCreateStart(t, cluster.gatewayAddr, cluster.codec, "eve", "secret", "EveHero")
	conn := dialWorld(t, worldAddr)
	defer conn.Close()

	sendText(t, conn, cluster.codec, "100\n")
	sendText(t, conn, cluster.codec, "play\n")
	sendText(t, conn, cluster.codec, "eve\n")
	sendText(t, conn, cluster.codec, "secret d n 0\n")
	if got := readText(t, conn, cluster.codec); got != "OK" {
		t.Fatalf("world auth response = %q, want OK", got)
	}

	sendText(t, conn, cluster.codec, "start_game\n")
	bootstrap := readUntilCharLoadComplete(t, conn, cluster.codec)

	if bootstrap.opcodes[protocol.ResGameStart] == 0 {
		t.Fatal("expected ResGameStart during bootstrap")
	}
	if bootstrap.opcodes[protocol.ResGamePlayReady] == 0 {
		t.Fatal("expected ResGamePlayReady during bootstrap")
	}
	for _, prefix := range []string{
		"at ",
		"status 0 ",
		"status 1 ",
		"status 2 ",
		"status 3 ",
		"status 4 ",
		"status 10 ",
		"status 11 ",
		"status 13 ",
		"status 14 ",
		"status 15 ",
		"status 16 ",
		"status 17 ",
		"status 18 ",
		"wearing ",
		"lp 0",
		"init_inven 0",
		"init_inven 1",
		"init_inven 2",
		"init_inven 3",
		"init_inven 4",
		"inven 0 ",
		"quick 0 ",
	} {
		if !containsPrefix(bootstrap.texts, prefix) {
			t.Fatalf("expected bootstrap text with prefix %q, got %v", prefix, bootstrap.texts)
		}
	}
	if containsPrefix(bootstrap.texts, "wearing 0 0 0 0 0 0 0") {
		t.Fatalf("expected wearing bootstrap to include starter equipment, got %v", bootstrap.texts)
	}
}

func TestGatewayRejectsDuplicateRegisterAndBadLogin(t *testing.T) {
	cluster := startTestCluster(t)

	registered := registerAccount(t, cluster.gatewayAddr, cluster.codec, "alice", "secret")
	_ = registered.Close()

	dupConn := dialGateway(t, cluster.gatewayAddr)
	defer dupConn.Close()
	sendText(t, dupConn, cluster.codec, "100\n")
	sendText(t, dupConn, cluster.codec, "register\n")
	sendText(t, dupConn, cluster.codec, "alice\n")
	sendText(t, dupConn, cluster.codec, "secret d n 0\n")
	if got := readText(t, dupConn, cluster.codec); got != "fail account_exists" {
		t.Fatalf("duplicate register response = %q, want %q", got, "fail account_exists")
	}

	loginConn := dialGateway(t, cluster.gatewayAddr)
	defer loginConn.Close()
	sendText(t, loginConn, cluster.codec, "100\n")
	sendText(t, loginConn, cluster.codec, "login\n")
	sendText(t, loginConn, cluster.codec, "alice\n")
	sendText(t, loginConn, cluster.codec, "wrong d n 0\n")
	if got := readText(t, loginConn, cluster.codec); got != "fail invalid_credentials" {
		t.Fatalf("bad login response = %q, want %q", got, "fail invalid_credentials")
	}
}

func TestZoneRejectsMissingHandoff(t *testing.T) {
	cluster := startTestCluster(t)
	registered := registerAccount(t, cluster.gatewayAddr, cluster.codec, "carol", "secret")
	_ = registered.Close()

	conn := dialWorld(t, cluster.worldAddr)
	defer conn.Close()
	sendText(t, conn, cluster.codec, "100\n")
	sendText(t, conn, cluster.codec, "play\n")
	sendText(t, conn, cluster.codec, "carol\n")
	sendText(t, conn, cluster.codec, "secret d n 0\n")
	if got := readText(t, conn, cluster.codec); got != "fail missing_handoff" {
		t.Fatalf("missing handoff response = %q, want %q", got, "fail missing_handoff")
	}
}

func TestGatewayCharacterCommandFailures(t *testing.T) {
	cluster := startTestCluster(t)

	conn := registerAccount(t, cluster.gatewayAddr, cluster.codec, "dave", "secret")
	defer conn.Close()

	sendText(t, conn, cluster.codec, "char_new 0 DaveHero 2 0 0 10 10 12 10 8 0\n")
	if got := readText(t, conn, cluster.codec); got != "success" {
		t.Fatalf("first char_new response = %q, want success", got)
	}

	sendText(t, conn, cluster.codec, "char_new 0 DaveHeroTwo 2 0 0 10 10 12 10 8 0\n")
	if got := readText(t, conn, cluster.codec); got != "fail slot_taken" {
		t.Fatalf("duplicate slot response = %q, want %q", got, "fail slot_taken")
	}

	sendText(t, conn, cluster.codec, "start 4 0 0 0 0 0 0 0\n")
	if got := readText(t, conn, cluster.codec); got != "fail char_not_found" {
		t.Fatalf("bad start response = %q, want %q", got, "fail char_not_found")
	}
}

func dialGateway(t *testing.T, gatewayAddr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", gatewayAddr)
	if err != nil {
		t.Fatalf("net.Dial(gateway) error = %v", err)
	}
	return conn
}

func dialWorld(t *testing.T, worldAddr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", worldAddr)
	if err != nil {
		t.Fatalf("net.Dial(world) error = %v", err)
	}
	return conn
}

func registerAccount(t *testing.T, gatewayAddr string, codec protocol.SeedCodec, username, password string) net.Conn {
	t.Helper()
	conn := dialGateway(t, gatewayAddr)
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
	return conn
}

func registerCreateStart(t *testing.T, gatewayAddr string, codec protocol.SeedCodec, username, password, characterName string) string {
	t.Helper()
	conn := registerAccount(t, gatewayAddr, codec, username, password)
	defer conn.Close()

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
	conn := dialWorld(t, worldAddr)
	sendText(t, conn, codec, "100\n")
	sendText(t, conn, codec, "play\n")
	sendText(t, conn, codec, username+"\n")
	sendText(t, conn, codec, password+" d n 0\n")
	if got := readText(t, conn, codec); got != "OK" {
		t.Fatalf("world auth response = %q, want OK", got)
	}
	sendText(t, conn, codec, "start_game\n")
	frame := readFrameByOpcode(t, conn, codec, protocol.ResGameStart)
	if frame.SubHeader.Index != protocol.ResGameStart {
		t.Fatalf("expected ResGameStart, got %+v", frame.SubHeader)
	}
	return conn
}

func connectAndLoadWorld(t *testing.T, worldAddr string, codec protocol.SeedCodec, username, password string) net.Conn {
	t.Helper()
	conn := dialWorld(t, worldAddr)
	sendText(t, conn, codec, "100\n")
	sendText(t, conn, codec, "play\n")
	sendText(t, conn, codec, username+"\n")
	sendText(t, conn, codec, password+" d n 0\n")
	if got := readText(t, conn, codec); got != "OK" {
		t.Fatalf("world auth response = %q, want OK", got)
	}
	sendText(t, conn, codec, "start_game\n")
	bootstrap := readUntilCharLoadComplete(t, conn, codec)
	if bootstrap.opcodes[protocol.ResGameStart] == 0 || bootstrap.opcodes[protocol.ResGamePlayReady] == 0 {
		t.Fatalf("unexpected bootstrap opcodes: %+v", bootstrap.opcodes)
	}
	return conn
}

type bootstrapFrames struct {
	texts   []string
	opcodes map[protocol.Opcode]int
}

func readUntilCharLoadComplete(t *testing.T, conn net.Conn, codec protocol.SeedCodec) bootstrapFrames {
	t.Helper()
	result := bootstrapFrames{opcodes: make(map[protocol.Opcode]int)}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		frame := readFrame(t, conn, codec)
		if protocol.IsTextCommand(frame) {
			text := strings.TrimSpace(string(frame.Payload))
			result.texts = append(result.texts, text)
			if text == "charloadcomplete" {
				return result
			}
			continue
		}
		result.opcodes[frame.SubHeader.Index]++
	}
	t.Fatalf("timed out waiting for charloadcomplete")
	return bootstrapFrames{}
}

func containsPrefix(lines []string, prefix string) bool {
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
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

func readTextByPrefix(t *testing.T, conn net.Conn, codec protocol.SeedCodec, prefix string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		frame := readFrame(t, conn, codec)
		if !protocol.IsTextCommand(frame) {
			continue
		}
		text := strings.TrimSpace(string(frame.Payload))
		if strings.HasPrefix(text, prefix) {
			return text
		}
	}
	t.Fatalf("timed out waiting for text prefix %q", prefix)
	return ""
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

func sendItemPick(t *testing.T, conn net.Conn, codec protocol.SeedCodec, itemIndex int32) {
	t.Helper()
	body := make([]byte, 4)
	binary.LittleEndian.PutUint32(body[0:4], uint32(itemIndex))
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := protocol.WriteFrame(conn, codec, protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.ReqItemPick, Type: protocol.PacketTypeRequest}, Payload: body}); err != nil {
		t.Fatalf("WriteFrame(item pick) error = %v", err)
	}
}

func sendItemDrop(t *testing.T, conn net.Conn, codec protocol.SeedCodec, itemIndex int32) {
	t.Helper()
	body := make([]byte, 4)
	binary.LittleEndian.PutUint32(body[0:4], uint32(itemIndex))
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := protocol.WriteFrame(conn, codec, protocol.Frame{SubHeader: protocol.SubHeader{Index: protocol.ReqItemDrop, Type: protocol.PacketTypeRequest}, Payload: body}); err != nil {
		t.Fatalf("WriteFrame(item drop) error = %v", err)
	}
}

func parseItemIndexFromInven(t *testing.T, line string) int32 {
	t.Helper()
	fields := strings.Fields(line)
	if len(fields) < 3 {
		t.Fatalf("bad inven line: %q", line)
	}
	value, err := strconv.ParseInt(fields[2], 10, 32)
	if err != nil {
		t.Fatalf("parse inven item index from %q: %v", line, err)
	}
	return int32(value)
}

func parseItemIndexFromPickResponse(t *testing.T, payload []byte) int32 {
	t.Helper()
	if len(payload) < 16 {
		t.Fatalf("bad pick response payload length: %d", len(payload))
	}
	return int32(binary.LittleEndian.Uint32(payload[12:16]))
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
