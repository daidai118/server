package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"laghaim-go/internal/platform/config"
	"laghaim-go/internal/platform/logging"
	"laghaim-go/internal/protocol"
	"laghaim-go/internal/repo/memory"
	authselectserver "laghaim-go/internal/server/authselect"
	zoneserver "laghaim-go/internal/server/zone"
	"laghaim-go/internal/service"
	"laghaim-go/internal/session"
	"laghaim-go/internal/world"
)

func main() {
	configPath := flag.String("config", "configs/dev.yaml", "path to yaml config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic(err)
	}

	logger := logging.New("dev-cluster", cfg.Environment)
	store := memory.NewStore()
	sessions := session.NewManager()
	handoffs := service.NewZoneHandoffRegistry()
	hasher := service.DefaultPasswordHasher()
	auth := service.NewAuthService(store, sessions, hasher, service.AuthConfig{GMSTicketTTL: 2 * time.Minute})
	chars := service.NewCharacterService(store, store, store, sessions, handoffs, service.CharacterConfig{ZoneTicketTTL: 2 * time.Minute})
	zoneSvc := service.NewZoneEntryService(store, sessions)
	codec := protocol.MustNewDefaultSeedCodec()

	zoneListener, err := net.Listen("tcp", net.JoinHostPort(cfg.Zone.Host, itoa(cfg.Zone.Port)))
	if err != nil {
		panic(err)
	}
	advertisedZoneHost := cfg.Zone.Host
	if advertisedZoneHost == "0.0.0.0" {
		advertisedZoneHost = "127.0.0.1"
	}
	zoneSrv := zoneserver.NewServer(zoneListener, codec, store, hasher, handoffs, zoneSvc, world.NewRuntime(), zoneserver.Config{
		StaticNPCs:  []zoneserver.StaticNPC{{Index: 10001, VNUM: 2001, MapID: 1, ZoneID: 0, PosX: 33100, PosZ: 33100, Vital: 100}},
		StaticItems: []zoneserver.StaticItem{{Index: 20001, ItemVNUM: 5001, MapID: 1, ZoneID: 0, PosX: 32950, PosZ: 32950}},
	})
	go func() {
		if err := zoneSrv.Serve(); err != nil {
			logger.Error("zone server stopped", "error", err)
		}
	}()

	gatewayListener, err := net.Listen("tcp", net.JoinHostPort(cfg.Login.Host, itoa(cfg.Login.Port)))
	if err != nil {
		panic(err)
	}
	gatewaySrv := authselectserver.NewServer(gatewayListener, codec, auth, chars, authselectserver.Config{ZoneHost: advertisedZoneHost, ZonePort: cfg.Zone.Port})
	go func() {
		if err := gatewaySrv.Serve(); err != nil {
			logger.Error("auth/select gateway stopped", "error", err)
		}
	}()

	logger.Info("dev cluster ready", "gateway_addr", gatewayListener.Addr().String(), "zone_addr", zoneListener.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	_ = gatewaySrv.Close()
	_ = zoneSrv.Close()
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
