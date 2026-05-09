package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"laghaim-go/internal/platform/config"
	"laghaim-go/internal/platform/database"
	"laghaim-go/internal/platform/logging"
	"laghaim-go/internal/protocol"
	"laghaim-go/internal/repo"
	"laghaim-go/internal/repo/memory"
	mysqlrepo "laghaim-go/internal/repo/mysql"
	authselectserver "laghaim-go/internal/server/authselect"
	zoneserver "laghaim-go/internal/server/zone"
	"laghaim-go/internal/service"
	"laghaim-go/internal/session"
	"laghaim-go/internal/world"
)

type stores struct {
	accounts repo.AccountRepository
	chars    repo.CharacterRepository
	stats    repo.CharacterStatsRepository
	inv      repo.InventoryRepository
	closer   func() error
}

func main() {
	configPath := flag.String("config", "configs/dev.yaml", "path to yaml config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic(err)
	}

	logger := logging.New("dev-cluster", cfg.Environment)
	ticketTTL, err := time.ParseDuration(cfg.Session.TicketTTL)
	if err != nil {
		panic(err)
	}

	stores, err := openStores(cfg)
	if err != nil {
		panic(err)
	}
	defer func() {
		if stores.closer != nil {
			_ = stores.closer()
		}
	}()

	sessions := session.NewManager()
	handoffs := service.NewZoneHandoffRegistry()
	hasher := service.DefaultPasswordHasher()
	auth := service.NewAuthService(stores.accounts, sessions, hasher, service.AuthConfig{GMSTicketTTL: ticketTTL})
	chars := service.NewCharacterService(stores.chars, stores.stats, stores.inv, sessions, handoffs, service.CharacterConfig{ZoneTicketTTL: ticketTTL})
	zoneSvc := service.NewZoneEntryService(stores.chars, stores.stats, stores.inv, sessions)
	codec := protocol.MustNewDefaultSeedCodec()

	zoneListener, err := net.Listen("tcp", net.JoinHostPort(cfg.Zone.Host, itoa(cfg.Zone.Port)))
	if err != nil {
		panic(err)
	}
	advertisedZoneHost := strings.TrimSpace(cfg.Zone.AdvertiseHost)
	if advertisedZoneHost == "" {
		advertisedZoneHost = cfg.Zone.Host
	}
	if advertisedZoneHost == "0.0.0.0" {
		advertisedZoneHost = "127.0.0.1"
	}
	advertisedZonePort := cfg.Zone.AdvertisePort
	if advertisedZonePort == 0 {
		advertisedZonePort = cfg.Zone.Port
	}
	zoneSrv := zoneserver.NewServer(zoneListener, codec, stores.accounts, hasher, handoffs, zoneSvc, world.NewRuntime(), zoneserver.Config{
		StaticNPCs:  zoneStaticNPCs(cfg.Zone.StaticNPCs),
		StaticItems: zoneStaticItems(cfg.Zone.StaticItems),
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
	gatewaySrv := authselectserver.NewServer(gatewayListener, codec, auth, chars, authselectserver.Config{ZoneHost: advertisedZoneHost, ZonePort: advertisedZonePort})
	go func() {
		if err := gatewaySrv.Serve(); err != nil {
			logger.Error("auth/select gateway stopped", "error", err)
		}
	}()

	logger.Info("dev cluster ready", "storage_backend", cfg.Storage.Backend, "gateway_addr", gatewayListener.Addr().String(), "zone_addr", zoneListener.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	_ = gatewaySrv.Close()
	_ = zoneSrv.Close()
}

func openStores(cfg config.Config) (stores, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Storage.Backend)) {
	case "", "memory":
		store := memory.NewStore()
		return stores{
			accounts: store,
			chars:    store,
			stats:    store,
			inv:      store,
		}, nil
	case "mysql":
		db, err := database.OpenMySQL(cfg.Database)
		if err != nil {
			return stores{}, err
		}
		store := mysqlrepo.NewStore(db)
		return stores{
			accounts: store,
			chars:    store,
			stats:    store,
			inv:      store,
			closer:   db.Close,
		}, nil
	default:
		return stores{}, fmt.Errorf("unsupported storage backend %q", cfg.Storage.Backend)
	}
}

func zoneStaticNPCs(items []config.StaticNPCConfig) []zoneserver.StaticNPC {
	out := make([]zoneserver.StaticNPC, 0, len(items))
	for _, item := range items {
		out = append(out, zoneserver.StaticNPC{
			Index:  item.Index,
			VNUM:   item.VNUM,
			MapID:  item.MapID,
			ZoneID: item.ZoneID,
			PosX:   item.PosX,
			PosZ:   item.PosZ,
			Dir:    item.Dir,
			Vital:  item.Vital,
		})
	}
	return out
}

func zoneStaticItems(items []config.StaticItemConfig) []zoneserver.StaticItem {
	out := make([]zoneserver.StaticItem, 0, len(items))
	for _, item := range items {
		out = append(out, zoneserver.StaticItem{
			Index:    item.Index,
			ItemVNUM: item.ItemVNUM,
			MapID:    item.MapID,
			ZoneID:   item.ZoneID,
			PosX:     item.PosX,
			PosZ:     item.PosZ,
			Dir:      item.Dir,
			Timed:    item.Timed,
		})
	}
	return out
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
