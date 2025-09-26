package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"aave-cap-alerts/internal/aave"
	"aave-cap-alerts/internal/config"
	"aave-cap-alerts/internal/monitor"
	"aave-cap-alerts/internal/notify"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to the YAML configuration file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pollInterval := 1 * time.Minute
	if cfg.PollInterval != "" {
		pollInterval, err = time.ParseDuration(cfg.PollInterval)
		if err != nil {
			log.Fatalf("parse poll_interval: %v", err)
		}
		if pollInterval <= 0 {
			log.Fatalf("poll_interval must be positive")
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ethClient, err := ethclient.DialContext(ctx, cfg.RPCURL)
	if err != nil {
		log.Fatalf("connect RPC: %v", err)
	}
	defer ethClient.Close()

	aaveClient, err := aave.NewClient(ethClient)
	if err != nil {
		log.Fatalf("setup aave client: %v", err)
	}

	notifiers, err := buildNotifiers(cfg)
	if err != nil {
		log.Fatalf("configure notifiers: %v", err)
	}

	if len(notifiers) == 0 {
		log.Println("warning: no notifiers configured; total supply changes will only be written to stdout")
	}

	service, err := monitor.NewService(aaveClient, cfg, notifiers, pollInterval)
	if err != nil {
		log.Fatalf("build monitor: %v", err)
	}

	log.Printf("monitoring %d asset(s) with poll interval %s", len(cfg.Assets), pollInterval)
	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("monitor run error: %v", err)
	}

	log.Println("shutdown complete")
}

func buildNotifiers(cfg *config.Config) ([]notify.Notifier, error) {
	notifiers := make([]notify.Notifier, 0, 2)

	if tg := cfg.Notifications.Telegram; tg != nil {
		if tg.BotToken == "" {
			return nil, fmt.Errorf("telegram.bot_token is required")
		}
		if tg.ChatID == "" {
			return nil, fmt.Errorf("telegram.chat_id is required")
		}
		notifiers = append(notifiers, notify.NewTelegramNotifier(tg.BotToken, tg.ChatID))
	}

	if rpc := cfg.Notifications.JSONRPC; rpc != nil {
		if rpc.URL == "" {
			return nil, fmt.Errorf("json_rpc.url is required")
		}
		notifiers = append(notifiers, notify.NewJSONRPCNotifier(rpc.URL))
	}

	return notifiers, nil
}
