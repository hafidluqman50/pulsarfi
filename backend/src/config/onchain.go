package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// OnchainConfig is every chain setting the backend needs, read once at boot so
// no service reaches for os.Getenv.
type OnchainConfig struct {
	RPCURL                string
	AgentWalletPrivateKey string
	AgentTaskManager      string
	PulsarProtocol        string
	IDRXAddress           string

	// Instant execution: the Arm click signs grant+execute and waits for one
	// receipt. The flag is a kill switch, on unless explicitly set to false.
	InstantExecution  bool
	ExecutionGasLimit uint64
	GrantGasLimit     uint64
	FeeCacheTTL       time.Duration
}

// LoadOnchain reads and validates the on-chain settings. The first three are
// required: the agent service degrades to "disabled" when they are missing,
// matching the existing zero-fallback rule. PULSAR_PROTOCOL and IDRX_ADDRESS
// are optional (used by the price/balance adapters).
func LoadOnchain() (OnchainConfig, error) {
	cfg := OnchainConfig{
		RPCURL:                strings.TrimSpace(os.Getenv("ALCHEMY_RPC_URL")),
		AgentWalletPrivateKey: strings.TrimPrefix(strings.TrimSpace(os.Getenv("AGENT_WALLET_PRIVATE_KEY")), "0x"),
		AgentTaskManager:      strings.TrimSpace(os.Getenv("AGENT_TASK_MANAGER_ADDRESS")),
		PulsarProtocol:        strings.TrimSpace(os.Getenv("PULSAR_PROTOCOL")),
		IDRXAddress:           strings.TrimSpace(os.Getenv("IDRX_ADDRESS")),
		InstantExecution:      os.Getenv("INSTANT_EXECUTION_ENABLED") != "false",
		ExecutionGasLimit:     uintEnv("EXECUTION_GAS_LIMIT", 700000),
		GrantGasLimit:         uintEnv("GRANT_GAS_LIMIT", 200000),
		FeeCacheTTL:           time.Duration(uintEnv("FEE_CACHE_TTL_SEC", 30)) * time.Second,
	}

	switch {
	case cfg.RPCURL == "":
		return OnchainConfig{}, fmt.Errorf("onchain: ALCHEMY_RPC_URL not set")
	case cfg.AgentWalletPrivateKey == "":
		return OnchainConfig{}, fmt.Errorf("onchain: AGENT_WALLET_PRIVATE_KEY not set")
	case cfg.AgentTaskManager == "":
		return OnchainConfig{}, fmt.Errorf("onchain: AGENT_TASK_MANAGER_ADDRESS not set")
	}

	return cfg, nil
}

func uintEnv(key string, fallback uint64) uint64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return fallback
	}
	return value
}
