package aave

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const scaledSupplyABIJSON = `[
    {
        "inputs": [],
        "name": "scaledTotalSupply",
        "outputs": [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    }
]`

const erc20ABIJSON = `[
    {
        "inputs": [],
        "name": "decimals",
        "outputs": [
            {
                "internalType": "uint8",
                "name": "",
                "type": "uint8"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs": [],
        "name": "totalSupply",
        "outputs": [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    }
]`

// Client wraps the low-level contract calls we need.
type Client struct {
	backend        *ethclient.Client
	supplyABI      abi.ABI
	erc20ABI       abi.ABI
	decimalsCache  map[common.Address]uint8
	decimalsLocker sync.RWMutex
}

// NewClient builds a client that can query scaled supply and ERC20 metadata.
func NewClient(backend *ethclient.Client) (*Client, error) {
	supplyABI, err := abi.JSON(strings.NewReader(scaledSupplyABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse scaled supply ABI: %w", err)
	}

	erc20ABI, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse erc20 ABI: %w", err)
	}

	return &Client{
		backend:       backend,
		supplyABI:     supplyABI,
		erc20ABI:      erc20ABI,
		decimalsCache: make(map[common.Address]uint8),
	}, nil
}

// ScaledTotalSupply fetches the current scaled total supply for an aToken.
func (c *Client) ScaledTotalSupply(ctx context.Context, asset common.Address) (*big.Int, error) {
	payload, err := c.supplyABI.Pack("scaledTotalSupply")
	if err != nil {
		return nil, fmt.Errorf("pack scaledTotalSupply call: %w", err)
	}

	call := ethereum.CallMsg{To: &asset, Data: payload}
	raw, err := c.backend.CallContract(ctx, call, nil)
	if err != nil {
		return nil, fmt.Errorf("call scaledTotalSupply: %w", err)
	}

	values, err := c.supplyABI.Unpack("scaledTotalSupply", raw)
	if err != nil {
		return nil, fmt.Errorf("unpack scaledTotalSupply: %w", err)
	}

	if len(values) != 1 {
		return nil, fmt.Errorf("unexpected scaledTotalSupply result length: %d", len(values))
	}

	supply, ok := values[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected scaledTotalSupply type %T", values[0])
	}

	return new(big.Int).Set(supply), nil
}

// Decimals returns the decimals for an ERC20 token, cached for repeated lookups.
func (c *Client) Decimals(ctx context.Context, asset common.Address) (uint8, error) {
	c.decimalsLocker.RLock()
	if decimals, ok := c.decimalsCache[asset]; ok {
		c.decimalsLocker.RUnlock()
		return decimals, nil
	}
	c.decimalsLocker.RUnlock()

	payload, err := c.erc20ABI.Pack("decimals")
	if err != nil {
		return 0, fmt.Errorf("pack decimals call: %w", err)
	}

	call := ethereum.CallMsg{To: &asset, Data: payload}
	raw, err := c.backend.CallContract(ctx, call, nil)
	if err != nil {
		return 0, fmt.Errorf("call decimals: %w", err)
	}

	values, err := c.erc20ABI.Unpack("decimals", raw)
	if err != nil {
		return 0, fmt.Errorf("unpack decimals: %w", err)
	}

	if len(values) != 1 {
		return 0, fmt.Errorf("unexpected decimals result length: %d", len(values))
	}

	// decimals() returns uint8 but is unpacked as uint8
	decimals, ok := values[0].(uint8)
	if !ok {
		return 0, fmt.Errorf("unexpected decimals type %T", values[0])
	}

	c.decimalsLocker.Lock()
	c.decimalsCache[asset] = decimals
	c.decimalsLocker.Unlock()

	return decimals, nil
}

// TotalSupply returns the current ERC20 totalSupply() value.
func (c *Client) TotalSupply(ctx context.Context, asset common.Address) (*big.Int, error) {
	payload, err := c.erc20ABI.Pack("totalSupply")
	if err != nil {
		return nil, fmt.Errorf("pack totalSupply call: %w", err)
	}

	call := ethereum.CallMsg{To: &asset, Data: payload}
	raw, err := c.backend.CallContract(ctx, call, nil)
	if err != nil {
		return nil, fmt.Errorf("call totalSupply: %w", err)
	}

	values, err := c.erc20ABI.Unpack("totalSupply", raw)
	if err != nil {
		return nil, fmt.Errorf("unpack totalSupply: %w", err)
	}

	if len(values) != 1 {
		return nil, fmt.Errorf("unexpected totalSupply result length: %d", len(values))
	}

	supply, ok := values[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected totalSupply type %T", values[0])
	}

	return new(big.Int).Set(supply), nil
}
