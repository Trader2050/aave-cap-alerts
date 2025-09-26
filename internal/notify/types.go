package notify

import (
	"math/big"
	"time"
)

// SupplyChangeEvent captures the details of an asset total supply change.
type SupplyChangeEvent struct {
	AssetName         string
	AssetAddress      string
	OldTotalSupply    *big.Int
	NewTotalSupply    *big.Int
	TargetTotalSupply *big.Int
	Decimals          uint8
	TriggerReasons    []string
	ObservedAt        time.Time
}
