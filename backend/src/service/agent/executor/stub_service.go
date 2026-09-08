package executor

import (
	"context"
	"errors"

	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

// ErrTradeExecutionNotImplemented is returned by every UnimplementedTaskExecutor
// method — real trade execution against AgentTaskManager.sol has never been
// built (executor.TaskExecutor's interface is still missing subTaskId/token/
// minimumOutputAmount/summary, see onchain/agenttaskmanager/client_service.go),
// not merely unconfigured. Zero fallback
// (docs/plans/agent-orchestration-graph-rebuild.md v2.5/v2.7): a trade that
// cannot actually execute must fail loudly, the same way a failed on-chain
// createTask aborts its turn — never a fabricated tx hash pretending a real
// fill happened. Renamed from StubTaskExecutor, whose old behavior (return
// "0xstub-trade-N" and claim success) was exactly the fake-success pattern
// this rule exists to forbid.
var ErrTradeExecutionNotImplemented = errors.New("executor: real trade execution is not implemented yet")

type UnimplementedTaskExecutor struct{}

func (UnimplementedTaskExecutor) ExecuteTrade(ctx context.Context, onChainTaskID uint, intent agent.TradeIntent, reasoningHash [32]byte) (string, uint64, error) {
	return "", 0, ErrTradeExecutionNotImplemented
}

func (UnimplementedTaskExecutor) TradePermissionRemaining(ctx context.Context, onChainTaskID uint) (string, error) {
	return "", ErrTradeExecutionNotImplemented
}
