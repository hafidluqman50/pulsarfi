package executor

import (
	"context"
	"fmt"

	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

// StubTaskExecutor is a placeholder until the real AgentTaskManager Go
// client (signing with AGENT_WALLET_PRIVATE_KEY) is wired up — it never
// touches the chain, just returns a fake tx hash / an effectively
// unlimited remaining budget, so the pipeline is exercisable end-to-end
// before that wiring exists. AgentTaskManager.sol itself is written and
// fork-tested (smart-contract/src/AgentTaskManager.sol); only this Go-side
// signer/caller is still deferred.
type StubTaskExecutor struct{}

func (StubTaskExecutor) ExecuteTrade(ctx context.Context, onChainTaskID uint, intent agent.TradeIntent, reasoningHash [32]byte) (string, uint64, error) {
	return fmt.Sprintf("0xstub-trade-%d", onChainTaskID), 1, nil
}

func (StubTaskExecutor) TradePermissionRemaining(ctx context.Context, onChainTaskID uint) (string, error) {
	return "999999999999999999", nil
}
