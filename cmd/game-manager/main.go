package main

import (
	"flag"
	"fmt"

	"laghaim-go/internal/platform/config"
	"laghaim-go/internal/platform/logging"
)

func main() {
	configPath := flag.String("config", "configs/dev.yaml", "path to yaml config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic(err)
	}

	logger := logging.New("game-manager", cfg.Environment)
	logger.Info("bootstrap ready", "listen_addr", fmt.Sprintf("%s:%d", cfg.GameManager.Host, cfg.GameManager.Port))
}
