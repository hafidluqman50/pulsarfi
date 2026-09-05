package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
)

// StubAgentContractClient is a placeholder until the real AgentTaskManager
// Go client (signing with AGENT_WALLET_PRIVATE_KEY) is wired up — mirrors
// executor.StubTaskExecutor's role on the execute side.
// AgentTaskManager.sol itself is written and fork-tested
// (smart-contract/src/AgentTaskManager.sol); only this Go-side
// signer/caller is still deferred. Satisfies both AgentContractClient
// (TaskService) and ChainClient (SubTaskRetryService).
type StubAgentContractClient struct {
	nextOnChainTaskID uint64
}

func (s *StubAgentContractClient) CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (uint64, error) {
	s.nextOnChainTaskID++
	return s.nextOnChainTaskID, nil
}

func (s *StubAgentContractClient) GrantTradePermission(ctx context.Context, onChainTaskID uint64, totalBudget string, duration time.Duration) error {
	return nil
}

func (s *StubAgentContractClient) RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) (string, error) {
	return fmt.Sprintf("0xstub-recordsubtasks-%d", onChainTaskID), nil
}

func (s *StubAgentContractClient) CancelTask(ctx context.Context, onChainTaskID uint64) error {
	return nil
}
