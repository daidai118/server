package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Environment string         `yaml:"environment"`
	Database    DatabaseConfig `yaml:"database"`
	Session     SessionConfig  `yaml:"session"`
	Login       TCPServer      `yaml:"login_server"`
	GameManager TCPServer      `yaml:"game_manager"`
	Zone        TCPServer      `yaml:"zone_server"`
	Admin       HTTPServer     `yaml:"admin_web"`
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
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("LAGHAIM_ENVIRONMENT"); v != "" {
		cfg.Environment = v
	}
	if v := os.Getenv("LAGHAIM_DATABASE_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("LAGHAIM_SESSION_TICKET_TTL"); v != "" {
		cfg.Session.TicketTTL = v
	}

	overridePort("LAGHAIM_LOGIN_SERVER_PORT", &cfg.Login.Port)
	overridePort("LAGHAIM_GAME_MANAGER_PORT", &cfg.GameManager.Port)
	overridePort("LAGHAIM_ZONE_SERVER_PORT", &cfg.Zone.Port)
	overridePort("LAGHAIM_ADMIN_WEB_PORT", &cfg.Admin.Port)
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
