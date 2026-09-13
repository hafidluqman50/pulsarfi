package agent

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/horizonlabs/pulsarfi-backend/src/abi"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
)

// Client wraps the abigen-generated AgentTaskManager binding (src/abi) with
// a real signer (AGENT_WALLET_PRIVATE_KEY) — the piece the old
// StubAgentContractClient (deleted, docs/plans/agent-orchestration-graph-rebuild.md
// v2.7) used to stand in for; executor.UnimplementedTaskExecutor (also since
// deleted, docs/plans/agent-trade-execution.md) used to stand in for
// ExecuteTrade specifically, until this Client's own real implementation,
// below, replaced it. Implements AgentContractClient, ChainClient (both
// same-package), and executor.TaskExecutor (a different package) all at
// once — all three are narrow views of the same underlying contract calls
// (ISP for each consumer), satisfied structurally; executor.TaskExecutor is
// never imported here, since Go interface satisfaction needs no import from
// the implementing side.
//
// Known simplification, not hidden: nonce management relies on
// go-ethereum's default PendingNonceAt-per-call behavior (TransactOpts.Nonce
// left nil). That is correct for serial calls but not safe under truly
// concurrent submissions from the same AGENT_WALLET — acceptable for this
// pass, revisit with an explicit nonce manager if throughput ever requires
// concurrent sends.
type Client struct {
	contract   *abi.AgentTaskManager
	ethClient  *ethclient.Client
	privateKey *bind.TransactOpts
	chainID    *big.Int
}

// NewClientFromEnv dials ALCHEMY_RPC_URL, loads AGENT_WALLET_PRIVATE_KEY,
// and binds to AGENT_TASK_MANAGER_ADDRESS. Returns an error (never panics)
// if any of these aren't configured — callers should degrade to a stub,
// matching the existing "disabled, not fatal" pattern already used for
// DeepSeek/search-tool wiring in service/index.go.
func NewClient(ctx context.Context) (*Client, error) {
	rpcURL := os.Getenv("ALCHEMY_RPC_URL")
	if rpcURL == "" {
		return nil, fmt.Errorf("onchain: ALCHEMY_RPC_URL not set")
	}
	privateKeyHex := strings.TrimPrefix(os.Getenv("AGENT_WALLET_PRIVATE_KEY"), "0x")
	if privateKeyHex == "" {
		return nil, fmt.Errorf("onchain: AGENT_WALLET_PRIVATE_KEY not set")
	}
	contractAddr := os.Getenv("AGENT_TASK_MANAGER_ADDRESS")
	if contractAddr == "" {
		return nil, fmt.Errorf("onchain: AGENT_TASK_MANAGER_ADDRESS not set")
	}

	ethClient, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("onchain: dial rpc: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("onchain: parse AGENT_WALLET_PRIVATE_KEY: %w", err)
	}

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("onchain: fetch chain id: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("onchain: build transactor: %w", err)
	}

	contract, err := abi.NewAgentTaskManager(common.HexToAddress(contractAddr), ethClient)
	if err != nil {
		return nil, fmt.Errorf("onchain: bind contract: %w", err)
	}

	return &Client{contract: contract, ethClient: ethClient, privateKey: auth, chainID: chainID}, nil
}

// authFor returns a fresh TransactOpts per call bound to ctx — never
// mutates the shared c.privateKey concurrently.
func (c *Client) authFor(ctx context.Context) *bind.TransactOpts {
	opts := *c.privateKey
	opts.Context = ctx
	return &opts
}

func (c *Client) CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (uint64, error) {
	opts := c.authFor(ctx)
	tx, err := c.contract.CreateTask(opts, common.HexToAddress(owner), isActionable, summary, promptHash)
	if err != nil {
		return 0, fmt.Errorf("onchain: createTask: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, c.ethClient, tx)
	if err != nil {
		return 0, fmt.Errorf("onchain: wait createTask: %w", err)
	}
	for _, log := range receipt.Logs {
		if event, err := c.contract.ParseTaskCreated(*log); err == nil {
			return event.TaskId.Uint64(), nil
		}
	}
	return 0, fmt.Errorf("onchain: createTask: TaskCreated event not found in receipt %s", tx.Hash())
}

func (c *Client) GrantTradePermission(ctx context.Context, onChainTaskID uint64, totalBudget string, duration time.Duration) error {
	budget, ok := new(big.Int).SetString(totalBudget, 10)
	if !ok {
		return fmt.Errorf("onchain: grantTradePermission: invalid totalBudget %q", totalBudget)
	}
	opts := c.authFor(ctx)
	tx, err := c.contract.GrantTradePermission(opts, new(big.Int).SetUint64(onChainTaskID), budget, big.NewInt(int64(duration.Seconds())))
	if err != nil {
		return fmt.Errorf("onchain: grantTradePermission: %w", err)
	}
	_, err = bind.WaitMined(ctx, c.ethClient, tx)
	return err
}

// subTaskAgentEnum/subTaskStatusEnum mirror
// AgentTaskManager.sol's TaskAgent/SubTaskStatus enum ordering exactly —
// Solidity enums are just integers in declaration order, there is no
// runtime name lookup to rely on.
func subTaskAgentEnum(agentName string) (uint8, error) {
	switch strings.ToLower(agentName) {
	case "supervisor":
		return 0, nil
	case "analyzer":
		return 1, nil
	case "executor":
		return 2, nil
	default:
		return 0, fmt.Errorf("onchain: unknown agent %q", agentName)
	}
}

func subTaskStatusEnum(status string) (uint8, error) {
	switch strings.ToLower(status) {
	case "done":
		return 0, nil
	case "failed":
		return 1, nil
	case "needs_input":
		return 2, nil
	default:
		return 0, fmt.Errorf("onchain: unknown sub task status %q", status)
	}
}

// RecordSubTasks converts each off-chain row into the on-chain
// SubTaskRecord shape. reasoningHash/outputHash are computed here, not
// stored off-chain — agent_sub_tasks keeps the full Reasoning/Output text
// verbatim (see model.AgentSubTask's own comment on why it's TEXT, not
// JSONB), this is simply keccak256 of those exact bytes.
//
// RoutingTarget is not currently captured by agent_sub_tasks at all (no
// such column exists) — passed through as "" until that gap is closed,
// not invented.
// RecordSubTasks returns the contract's own assigned ids alongside the tx
// hash — previously discarded entirely (docs/plans/agent-orchestration-graph-rebuild.md
// v2.14). Parsed from each row's own SubTaskRecorded event in the mined
// receipt, in emission order, which the contract guarantees matches the
// input order (recordSubTasks's own for-loop emits one event per record,
// in the order given).
func (c *Client) RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) ([]uint64, string, error) {
	records := make([]abi.AgentTaskManagerSubTaskRecord, 0, len(rows))
	for _, row := range rows {
		agentEnum, err := subTaskAgentEnum(row.Agent)
		if err != nil {
			return nil, "", err
		}
		statusEnum, err := subTaskStatusEnum(row.Status)
		if err != nil {
			return nil, "", err
		}
		output := ""
		if row.Output != nil {
			output = *row.Output
		}
		records = append(records, abi.AgentTaskManagerSubTaskRecord{
			TaskId:               new(big.Int).SetUint64(onChainTaskID),
			Agent:                agentEnum,
			StepName:             row.StepName,
			Status:               statusEnum,
			RoutingTarget:        "",
			Summary:              truncateSummary(row.Reasoning),
			ReasoningHash:        crypto.Keccak256Hash([]byte(row.Reasoning)),
			OutputHash:           crypto.Keccak256Hash([]byte(output)),
			DecisionHash:         common.HexToHash(row.DecisionHash),
			PreviousDecisionHash: common.HexToHash(row.PrevDecisionHash),
		})
	}

	opts := c.authFor(ctx)
	tx, err := c.contract.RecordSubTasks(opts, new(big.Int).SetUint64(onChainTaskID), records)
	if err != nil {
		return nil, "", fmt.Errorf("onchain: recordSubTasks: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, c.ethClient, tx)
	if err != nil {
		return nil, "", fmt.Errorf("onchain: wait recordSubTasks: %w", err)
	}

	subTaskIDs := make([]uint64, 0, len(rows))
	for _, log := range receipt.Logs {
		event, err := c.contract.ParseSubTaskRecorded(*log)
		if err != nil {
			continue
		}
		subTaskIDs = append(subTaskIDs, event.SubTaskId.Uint64())
	}
	if len(subTaskIDs) != len(rows) {
		return nil, "", fmt.Errorf("onchain: recordSubTasks: expected %d SubTaskRecorded events, got %d in receipt %s", len(rows), len(subTaskIDs), tx.Hash())
	}
	return subTaskIDs, tx.Hash().Hex(), nil
}

func (c *Client) CancelTask(ctx context.Context, onChainTaskID uint64) error {
	opts := c.authFor(ctx)
	tx, err := c.contract.CancelTask(opts, new(big.Int).SetUint64(onChainTaskID))
	if err != nil {
		return fmt.Errorf("onchain: cancelTask: %w", err)
	}
	_, err = bind.WaitMined(ctx, c.ethClient, tx)
	return err
}

// tradeSideEnum mirrors AgentTaskManager.sol's TradeSide enum ordering
// exactly (Buy=0, Sell=1) — same integer-by-declaration-order rule as
// subTaskAgentEnum/subTaskStatusEnum above.
func tradeSideEnum(side contracts.TradeSide) uint8 {
	if side == contracts.TradeSideSell {
		return 1
	}
	return 0
}

type ExecuteTradeOutput struct {
	TxHash         string
	TradeID        uint64
	BlockNumber    int64
	LogIndex       int
	Amount         *big.Int
	ReceivedAmount *big.Int
	ProtocolFee    *big.Int
}

// ExecuteTrade calls the real contract — subTaskID must already be
// on-chain (SubTaskRecorder.Record is chain-first, docs/plans/agent-orchestration-graph-rebuild.md
// v2.14, so this is guaranteed by the time submit_trade reaches this call),
// token is the stock's own ERC20 contract address (used by the contract
// for a Sell; ignored for a Buy, which always pulls IDRX directly — passed
// through unconditionally regardless of side, since the contract decides
// what to do with it), and amount/minimumOutputAmount are already in the
// correct raw units for the given side (docs/plans/agent-trade-execution.md
// Gap C — that conversion happens in submit_trade, not here).
func (c *Client) ExecuteTrade(ctx context.Context, onChainTaskID uint64, subTaskID uint64, token common.Address, ticker string, side contracts.TradeSide, amount *big.Int, minimumOutputAmount *big.Int, summary string, reasoningHash [32]byte) (*ExecuteTradeOutput, error) {
	if side == contracts.TradeSideBuy {
		if idrxAddr, err := c.contract.Idrx(nil); err == nil && idrxAddr != (common.Address{}) {
			token = idrxAddr
		} else if envIDRX := os.Getenv("IDRX_ADDRESS"); envIDRX != "" {
			token = common.HexToAddress(envIDRX)
		}
	}
	opts := c.authFor(ctx)
	tx, err := c.contract.ExecuteTrade(opts,
		new(big.Int).SetUint64(onChainTaskID),
		new(big.Int).SetUint64(subTaskID),
		token, ticker, tradeSideEnum(side), amount, minimumOutputAmount, summary, reasoningHash,
	)
	if err != nil {
		return nil, fmt.Errorf("onchain: executeTrade: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, c.ethClient, tx)
	if err != nil {
		return nil, fmt.Errorf("onchain: wait executeTrade: %w", err)
	}
	for _, log := range receipt.Logs {
		if event, err := c.contract.ParseTradeExecuted(*log); err == nil {
			tradeID := event.TradeId.Uint64()
			out := &ExecuteTradeOutput{
				TxHash:         tx.Hash().Hex(),
				TradeID:        tradeID,
				BlockNumber:    receipt.BlockNumber.Int64(),
				LogIndex:       int(log.Index),
				Amount:         amount,
				ReceivedAmount: minimumOutputAmount,
				ProtocolFee:    big.NewInt(0),
			}
			if tradeRecord, err := c.contract.Trades(&bind.CallOpts{Context: ctx}, event.TradeId); err == nil {
				if tradeRecord.Amount != nil && tradeRecord.Amount.Sign() > 0 {
					out.Amount = tradeRecord.Amount
				}
				if tradeRecord.ReceivedAmount != nil && tradeRecord.ReceivedAmount.Sign() > 0 {
					out.ReceivedAmount = tradeRecord.ReceivedAmount
				}
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("onchain: executeTrade: TradeExecuted event not found in receipt %s", tx.Hash())
}

// truncateSummary bounds what gets written on-chain as a Sub Task's
// "short" summary — agent_sub_tasks has no separate short-summary column
// (only the full Reasoning text), so this is a defensive cap against an
// unexpectedly long reasoning string driving up gas, not a design choice
// to duplicate the full text on-chain.
func truncateSummary(s string) string {
	const max = 300
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func (c *Client) TradePermissionRemaining(ctx context.Context, onChainTaskID uint) (string, error) {
	permission, err := c.contract.TradePermissions(&bind.CallOpts{Context: ctx}, new(big.Int).SetUint64(uint64(onChainTaskID)))
	if err != nil {
		return "", fmt.Errorf("onchain: tradePermissions: %w", err)
	}
	remaining := new(big.Int).Sub(permission.TotalBudget, permission.UsedBudget)
	return remaining.String(), nil
}
