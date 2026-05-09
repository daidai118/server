package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Environment string           `yaml:"environment"`
	Storage     StorageConfig    `yaml:"storage"`
	Database    DatabaseConfig   `yaml:"database"`
	Session     SessionConfig    `yaml:"session"`
	Login       TCPServer        `yaml:"login_server"`
	GameManager TCPServer        `yaml:"game_manager"`
	Zone        ZoneServerConfig `yaml:"zone_server"`
	Admin       HTTPServer       `yaml:"admin_web"`
}

type StorageConfig struct {
	Backend string `yaml:"backend"`
}

type DatabaseConfig struct {
	DSN             string `yaml:"dsn"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

type SessionConfig struct {
	TicketTTL string `yaml:"ticket_ttl"`
}

type TCPServer struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type ZoneServerConfig struct {
	Host          string             `yaml:"host"`
	Port          int                `yaml:"port"`
	AdvertiseHost string             `yaml:"advertise_host"`
	AdvertisePort int                `yaml:"advertise_port"`
	StaticNPCs    []StaticNPCConfig  `yaml:"static_npcs"`
	StaticItems   []StaticItemConfig `yaml:"static_items"`
}

type StaticNPCConfig struct {
	Index  int32   `yaml:"index"`
	VNUM   int32   `yaml:"vnum"`
	MapID  uint32  `yaml:"map_id"`
	ZoneID uint32  `yaml:"zone_id"`
	PosX   float32 `yaml:"pos_x"`
	PosZ   float32 `yaml:"pos_z"`
	Dir    float32 `yaml:"dir"`
	Vital  int32   `yaml:"vital"`
}

type StaticItemConfig struct {
	Index    int32   `yaml:"index"`
	ItemVNUM int32   `yaml:"item_vnum"`
	MapID    uint32  `yaml:"map_id"`
	ZoneID   uint32  `yaml:"zone_id"`
	PosX     float32 `yaml:"pos_x"`
	PosZ     float32 `yaml:"pos_z"`
	Dir      float32 `yaml:"dir"`
	Timed    bool    `yaml:"timed"`
}

type HTTPServer struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	applyEnv(&cfg)
	applyDefaults(&cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("LAGHAIM_ENVIRONMENT"); v != "" {
		cfg.Environment = v
	}
	if v := os.Getenv("LAGHAIM_STORAGE_BACKEND"); v != "" {
		cfg.Storage.Backend = v
	}
	if v := os.Getenv("LAGHAIM_DATABASE_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("LAGHAIM_SESSION_TICKET_TTL"); v != "" {
		cfg.Session.TicketTTL = v
	}

	if v := os.Getenv("LAGHAIM_ZONE_SERVER_ADVERTISE_HOST"); v != "" {
		cfg.Zone.AdvertiseHost = v
	}

	overridePort("LAGHAIM_LOGIN_SERVER_PORT", &cfg.Login.Port)
	overridePort("LAGHAIM_GAME_MANAGER_PORT", &cfg.GameManager.Port)
	overridePort("LAGHAIM_ZONE_SERVER_PORT", &cfg.Zone.Port)
	overridePort("LAGHAIM_ZONE_SERVER_ADVERTISE_PORT", &cfg.Zone.AdvertisePort)
	overridePort("LAGHAIM_ADMIN_WEB_PORT", &cfg.Admin.Port)
}

func applyDefaults(cfg *Config) {
	if cfg.Storage.Backend == "" {
		cfg.Storage.Backend = "memory"
	}
	if cfg.Session.TicketTTL == "" {
		cfg.Session.TicketTTL = "2m"
	}
}

func overridePort(env string, target *int) {
	raw := os.Getenv(env)
	if raw == "" {
		return
	}
	port, err := strconv.Atoi(raw)
	if err != nil {
		panic(fmt.Sprintf("invalid %s=%q", env, raw))
	}
	*target = port
}
