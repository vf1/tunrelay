package main

import (
	"fmt"
	"log/slog"
	"os"

	"tunrelay/internal/relay"
)

func main() {
	cfg, err := getConfig()
	if err != nil {
		fatal("get config", err)
	}

	relays, err := relay.NewRelays(cfg.Relays, slog.Default())
	if err != nil {
		fatal("create relays", err)
	}

	restoreSystem, err := setupSystem(cfg.System)
	if err != nil {
		fatal("setup system", err)
	}
	defer restoreSystem()

	waitForSigterm()
	fmt.Println("")

	slog.Info("terminating")
	restoreSystem()
	err = relays.Close()
	slog.Info("close relays")
	if err != nil {
		slog.Error(fmt.Sprintf("close relays: %v", err))
	}

	slog.Info("done.")
}

func fatal(msg string, err error) {
	if err == nil {
		slog.Error(msg)
	} else {
		slog.Error(fmt.Sprintf("%s: %v", msg, err))
	}
	os.Exit(1)
}
