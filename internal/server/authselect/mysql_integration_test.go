package authselect_test

import (
	"database/sql"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"laghaim-go/internal/platform/config"
	"laghaim-go/internal/platform/database"
	"laghaim-go/internal/protocol"
	mysqlrepo "laghaim-go/internal/repo/mysql"
	authselectserver "laghaim-go/internal/server/authselect"
	zoneserver "laghaim-go/internal/server/zone"
	"laghaim-go/internal/service"
	"laghaim-go/internal/session"
	"laghaim-go/internal/world"
)

func TestMySQLGatewayAndZoneIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LAGHAIM_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("LAGHAIM_TEST_MYSQL_DSN is not set")
	}

	db, err := database.OpenMySQL(config.DatabaseConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("OpenMySQL() error = %v", err)
	}
	defer db.Close()

	resetMySQLSchema(t, db)
	cluster := startMySQLTestCluster(t, db)

	worldAddr := registerCreateStart(t, cluster.gatewayAddr, cluster.codec, "mysqlalice", "secret", "MysqlAlice")
	worldConn := connectAndEnterWorld(t, worldAddr, cluster.codec, "mysqlalice", "secret")
	defer worldConn.Close()

	if frame := readFrameByOpcode(t, worldConn, cluster.codec, protocol.UpdMapInNpc); frame.SubHeader.Index != protocol.UpdMapInNpc {
		t.Fatalf("expected UpdMapInNpc, got %+v", frame.SubHeader)
	}
	if frame := readFrameByOpcode(t, worldConn, cluster.codec, protocol.UpdMapInItem); frame.SubHeader.Index != protocol.UpdMapInItem {
		t.Fatalf("expected UpdMapInItem, got %+v", frame.SubHeader)
	}

	conn := registerAccount(t, cluster.gatewayAddr, cluster.codec, "mysqlslot", "secret")
	defer conn.Close()

	sendText(t, conn, cluster.codec, "char_new 0 MysqlSlotA 2 0 0 10 10 12 10 8 0\n")
	if got := readText(t, conn, cluster.codec); got != "success" {
		t.Fatalf("first mysql char_new response = %q, want success", got)
	}

	sendText(t, conn, cluster.codec, "chars\n")
	if got := readText(t, conn, cluster.codec); got != "chars_start" {
		t.Fatalf("expected chars_start, got %q", got)
	}
	charLine := readText(t, conn, cluster.codec)
	fields := strings.Fields(charLine)
	if len(fields) < 3 {
		t.Fatalf("unexpected chars_exist line: %q", charLine)
	}
	characterID, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		t.Fatalf("ParseUint(characterID) error = %v", err)
	}
	if got := readText(t, conn, cluster.codec); got != "chars_end 0 0" {
		t.Fatalf("expected chars_end, got %q", got)
	}

	sendText(t, conn, cluster.codec, "char_del 0 "+strconv.FormatUint(characterID, 10)+"\n")
	if got := readText(t, conn, cluster.codec); got != "success" {
		t.Fatalf("mysql char_del response = %q, want success", got)
	}

	sendText(t, conn, cluster.codec, "char_new 0 MysqlSlotB 2 0 1 10 10 12 10 8 0\n")
	if got := readText(t, conn, cluster.codec); got != "success" {
		t.Fatalf("mysql slot reuse char_new response = %q, want success", got)
	}
}

func startMySQLTestCluster(t *testing.T, db *sql.DB) testCluster {
	t.Helper()

	store := mysqlrepo.NewStore(db)
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

func resetMySQLSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	repoRoot := repoRootFromCaller(t)
	downSQL, err := os.ReadFile(filepath.Join(repoRoot, "migrations", "000001_p0_core.down.sql"))
	if err != nil {
		t.Fatalf("ReadFile(down) error = %v", err)
	}
	upSQL, err := os.ReadFile(filepath.Join(repoRoot, "migrations", "000001_p0_core.up.sql"))
	if err != nil {
		t.Fatalf("ReadFile(up) error = %v", err)
	}

	if _, err := db.Exec(string(downSQL)); err != nil {
		t.Fatalf("exec down migration error = %v", err)
	}
	if _, err := db.Exec(string(upSQL)); err != nil {
		t.Fatalf("exec up migration error = %v", err)
	}
}

func repoRootFromCaller(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}
