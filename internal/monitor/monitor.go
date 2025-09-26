package monitor

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"aave-cap-alerts/internal/aave"
	"aave-cap-alerts/internal/config"
	"aave-cap-alerts/internal/notify"
)

// Service coordinates polling the configured reserves and firing notifications when thresholds are crossed.
type Service struct {
	client      *aave.Client
	assets      []*assetWatcher
	notifiers   []notify.Notifier
	defaultPoll time.Duration
}

// NewService builds a monitoring service from the loaded configuration.
func NewService(client *aave.Client, cfg *config.Config, notifiers []notify.Notifier, defaultPoll time.Duration) (*Service, error) {
	if defaultPoll <= 0 {
		return nil, fmt.Errorf("default poll interval must be positive")
	}

	watchers := make([]*assetWatcher, 0, len(cfg.Assets))
	for _, assetCfg := range cfg.Assets {
		name := assetCfg.Name
		if name == "" {
			name = assetCfg.Address
		}
		if assetCfg.Address == "" {
			return nil, fmt.Errorf("asset %s address must be provided", name)
		}
		if !common.IsHexAddress(assetCfg.Address) {
			return nil, fmt.Errorf("asset %s address is not a valid hex string", name)
		}
		addr := common.HexToAddress(assetCfg.Address)
		target, err := parseBigInt(assetCfg.TargetCapTokens)
		if err != nil {
			return nil, fmt.Errorf("asset %s target threshold: %w", name, err)
		}

		watcher := &assetWatcher{
			name:              name,
			address:           addr,
			targetTotalSupply: target,
			notifyOnIncrease:  valueOrDefault(assetCfg.NotifyOnIncrease, true),
			notifyOnDecrease:  valueOrDefault(assetCfg.NotifyOnDecrease, false),
			pollInterval:      defaultPoll,
		}

		if assetCfg.PollInterval != "" {
			customPoll, err := time.ParseDuration(assetCfg.PollInterval)
			if err != nil {
				return nil, fmt.Errorf("parse asset %s poll interval: %w", assetCfg.Name, err)
			}
			if customPoll <= 0 {
				return nil, fmt.Errorf("asset %s poll interval must be positive", assetCfg.Name)
			}
			watcher.pollInterval = customPoll
		}

		watchers = append(watchers, watcher)
	}

	return &Service{
		client:      client,
		assets:      watchers,
		notifiers:   notifiers,
		defaultPoll: defaultPoll,
	}, nil
}

// Run launches the monitoring loops and blocks until the context is cancelled.
func (s *Service) Run(ctx context.Context) error {
	if len(s.assets) == 0 {
		return fmt.Errorf("no assets configured")
	}

	for _, asset := range s.assets {
		go asset.run(ctx, s.client, s.notifiers)
	}

	<-ctx.Done()
	return ctx.Err()
}

func parseBigInt(v string) (*big.Int, error) {
	if v == "" {
		return nil, nil
	}
	value, ok := new(big.Int).SetString(v, 10)
	if !ok {
		return nil, fmt.Errorf("invalid integer %q", v)
	}
	return value, nil
}

func valueOrDefault(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

type assetWatcher struct {
	name              string
	address           common.Address
	targetTotalSupply *big.Int
	notifyOnIncrease  bool
	notifyOnDecrease  bool
	pollInterval      time.Duration
	decimalsLoaded    bool
	decimals          uint8
	lastTotalSupply   *big.Int
}

func (a *assetWatcher) run(ctx context.Context, client *aave.Client, notifiers []notify.Notifier) {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	// Trigger an immediate check on startup.
	if err := a.check(ctx, client, notifiers); err != nil {
		log.Printf("asset %s initial check failed: %v", a.name, err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.check(ctx, client, notifiers); err != nil {
				log.Printf("asset %s check failed: %v", a.name, err)
			}
		}
	}
}

func (a *assetWatcher) check(ctx context.Context, client *aave.Client, notifiers []notify.Notifier) error {
	if !a.decimalsLoaded {
		decimals, err := client.Decimals(ctx, a.address)
		if err != nil {
			return fmt.Errorf("fetch decimals: %w", err)
		}
		a.decimals = decimals
		a.decimalsLoaded = true
	}

	if a.lastTotalSupply == nil {
		log.Printf("asset %s check: last total supply not yet recorded", a.name)
	} else {
		log.Printf("asset %s check: last total supply %s", a.name, a.lastTotalSupply.String())
	}

	totalSupply, err := client.TotalSupply(ctx, a.address)
	if err != nil {
		return fmt.Errorf("fetch totalSupply: %w", err)
	}

	if a.lastTotalSupply == nil {
		a.lastTotalSupply = new(big.Int).Set(totalSupply)
		log.Printf("asset %s initial total supply %s", a.name, totalSupply.String())
		return nil
	}

	if totalSupply.Cmp(a.lastTotalSupply) == 0 {
		return nil
	}

	reasons := a.evaluateTriggers(totalSupply)
	if len(reasons) == 0 {
		log.Printf("asset %s total supply changed to %s (no triggers matched)", a.name, totalSupply.String())
		a.lastTotalSupply = new(big.Int).Set(totalSupply)
		return nil
	}

	event := notify.SupplyChangeEvent{
		AssetName:         a.name,
		AssetAddress:      a.address.Hex(),
		OldTotalSupply:    new(big.Int).Set(a.lastTotalSupply),
		NewTotalSupply:    new(big.Int).Set(totalSupply),
		TargetTotalSupply: cloneBigInt(a.targetTotalSupply),
		Decimals:          a.decimals,
		TriggerReasons:    reasons,
		ObservedAt:        time.Now(),
	}

	log.Printf("asset %s total supply change detected: %s -> %s", a.name, a.lastTotalSupply.String(), totalSupply.String())
	for _, notifier := range notifiers {
		if err := notifier.Notify(ctx, event); err != nil {
			log.Printf("asset %s notifier error: %v", a.name, err)
		}
	}

	a.lastTotalSupply = new(big.Int).Set(totalSupply)
	return nil
}

func (a *assetWatcher) evaluateTriggers(newSupply *big.Int) []string {
	reasons := make([]string, 0, 2)

	if a.lastTotalSupply != nil {
		switch newSupply.Cmp(a.lastTotalSupply) {
		case 1:
			if a.notifyOnIncrease && increasedByMoreThanOnePercent(a.lastTotalSupply, newSupply) {
				reasons = append(reasons, fmt.Sprintf("total supply increased more than 1%%: %s -> %s", a.lastTotalSupply.String(), newSupply.String()))
			}
		case -1:
			if a.notifyOnDecrease {
				reasons = append(reasons, fmt.Sprintf("total supply decreased from %s to %s", a.lastTotalSupply.String(), newSupply.String()))
			}
		}
	}

	if a.targetTotalSupply != nil && a.lastTotalSupply != nil {
		if a.lastTotalSupply.Cmp(a.targetTotalSupply) < 0 && newSupply.Cmp(a.targetTotalSupply) >= 0 {
			reasons = append(reasons, fmt.Sprintf("total supply reached target %s", a.targetTotalSupply.String()))
		}
	}

	return reasons
}

func cloneBigInt(v *big.Int) *big.Int {
	if v == nil {
		return nil
	}
	return new(big.Int).Set(v)
}

func increasedByMoreThanOnePercent(oldSupply, newSupply *big.Int) bool {
	if oldSupply == nil || oldSupply.Sign() <= 0 {
		return false
	}

	scaledNew := new(big.Int).Mul(newSupply, big.NewInt(100))
	threshold := new(big.Int).Mul(oldSupply, big.NewInt(110))
	return scaledNew.Cmp(threshold) == 1
}
