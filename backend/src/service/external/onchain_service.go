package external

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/horizonlabs/pulsarfi-backend/src/abi"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
)

// OnchainService is the app's on-chain service: it wraps the abigen-generated
// AgentTaskManager binding (src/abi) with a real signer (AGENT_WALLET). It
// lives in external because it faces the chain, not the agent domain — the
// agent's own interfaces (OrchestratorChainClient, ChainSubTaskRecorder,
// executor.TaskExecutor) are satisfied structurally, so nothing here imports
// the agent packages.
//
// Nonce synchronization: a sync.Mutex protects all mutating on-chain
// submissions signed by AGENT_WALLET, ensuring serial execution and preventing
// nonce collisions under concurrent chat turns.
type OnchainService struct {
	contract    *abi.AgentTaskManager
	ethClient   *ethclient.Client
	privateKey  *bind.TransactOpts
	chainID     *big.Int
	idrxAddress common.Address
	address     common.Address
	rpc         *rpc.Client
	mu          sync.Mutex

	grantGasLimit     uint64
	executionGasLimit uint64

	feeMu  sync.Mutex
	feeCap *big.Int
	feeTip *big.Int
	feeAt  time.Time
	feeTTL time.Duration
}

// NewOnchainService dials the RPC, loads the signing key and binds the
// AgentTaskManager contract from cfg. It returns an error (never panics) when
// the configuration is incomplete or the chain is unreachable; callers keep the
// existing "disabled, not fatal" behaviour.
func NewOnchainService(ctx context.Context, cfg config.OnchainConfig) (*OnchainService, error) {
	ethClient, err := ethclient.DialContext(ctx, cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("onchain: dial rpc: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(cfg.AgentWalletPrivateKey)
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

	contract, err := abi.NewAgentTaskManager(common.HexToAddress(cfg.AgentTaskManager), ethClient)
	if err != nil {
		return nil, fmt.Errorf("onchain: bind contract: %w", err)
	}

	var idrx common.Address
	if cfg.IDRXAddress != "" {
		idrx = common.HexToAddress(cfg.IDRXAddress)
	}

	rpcClient, err := rpc.DialContext(ctx, cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("onchain: dial rpc client: %w", err)
	}

	grantGas := cfg.GrantGasLimit
	if grantGas == 0 {
		grantGas = 200000
	}
	executionGas := cfg.ExecutionGasLimit
	if executionGas == 0 {
		executionGas = 700000
	}
	feeTTL := cfg.FeeCacheTTL
	if feeTTL <= 0 {
		feeTTL = 30 * time.Second
	}

	service := &OnchainService{
		contract:          contract,
		ethClient:         ethClient,
		privateKey:        auth,
		chainID:           chainID,
		idrxAddress:       idrx,
		address:           crypto.PubkeyToAddress(privateKey.PublicKey),
		rpc:               rpcClient,
		grantGasLimit:     grantGas,
		executionGasLimit: executionGas,
		feeTTL:            feeTTL,
	}
	// Warm the fee cache once, then keep it warm off the click path.
	_, _ = service.currentFees(ctx)
	go service.refreshFeesLoop(ctx)
	return service, nil
}

func (s *OnchainService) refreshFeesLoop(ctx context.Context) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = s.currentFees(context.Background())
		}
	}
}

// authFor returns a fresh TransactOpts per call bound to ctx — never
// mutates the shared s.privateKey concurrently.
func (s *OnchainService) authFor(ctx context.Context) *bind.TransactOpts {
	opts := *s.privateKey
	opts.Context = ctx
	return &opts
}

func (s *OnchainService) CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (uint64, error) {
	s.mu.Lock()
	tx, err := s.contract.CreateTask(s.authFor(ctx), common.HexToAddress(owner), isActionable, summary, promptHash)
	s.mu.Unlock()
	if err != nil {
		return 0, fmt.Errorf("onchain: createTask: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, s.ethClient, tx)
	if err != nil {
		return 0, fmt.Errorf("onchain: wait createTask: %w", err)
	}
	for _, log := range receipt.Logs {
		if event, err := s.contract.ParseTaskCreated(*log); err == nil {
			return event.TaskId.Uint64(), nil
		}
	}
	return 0, fmt.Errorf("onchain: createTask: TaskCreated event not found in receipt %s", tx.Hash())
}

// MarkActionable flips a Task from informational to actionable on-chain —
// needed because a Task can open before its own parameters are fully known
// (Supervisor's intake may still be gathering answers), so isActionable at
// createTask time can genuinely still be false when the request turns out
// actionable once the user finishes answering.
func (s *OnchainService) MarkActionable(ctx context.Context, onChainTaskID uint64) error {
	s.mu.Lock()
	tx, err := s.contract.MarkActionable(s.authFor(ctx), new(big.Int).SetUint64(onChainTaskID))
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("onchain: markActionable: %w", err)
	}
	_, err = bind.WaitMined(ctx, s.ethClient, tx)
	return err
}

func (s *OnchainService) GrantTradePermission(ctx context.Context, onChainTaskID uint64, totalBudget string, duration time.Duration, maxAmountPerTrade string, cooldownInterval uint32) error {
	// Idempotency check: if trade permission was already confirmed on-chain
	// (e.g. previous network timeout or retry), treat as success.
	optsCall := &bind.CallOpts{Context: ctx}
	if perm, err := s.contract.TradePermissions(optsCall, new(big.Int).SetUint64(onChainTaskID)); err == nil && perm.ExpiresAt > 0 {
		return nil
	}

	budget, ok := new(big.Int).SetString(totalBudget, 10)
	if !ok {
		return fmt.Errorf("onchain: grantTradePermission: invalid totalBudget %q", totalBudget)
	}
	maxTrade := big.NewInt(0)
	if maxAmountPerTrade != "" {
		if m, ok := new(big.Int).SetString(maxAmountPerTrade, 10); ok {
			maxTrade = m
		}
	}
	cooldown := big.NewInt(int64(cooldownInterval))

	s.mu.Lock()
	tx, err := s.contract.GrantTradePermission(s.authFor(ctx), new(big.Int).SetUint64(onChainTaskID), budget, big.NewInt(int64(duration.Seconds())), maxTrade, cooldown)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("onchain: grantTradePermission: %w", err)
	}
	_, err = bind.WaitMined(ctx, s.ethClient, tx)
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
// hash — previously discarded entirely. Parsed from each row's own
// SubTaskRecorded event in the mined receipt, in emission order, which the
// contract guarantees matches the input order (recordSubTasks's own for-loop
// emits one event per record, in the order given).
func (s *OnchainService) RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) ([]uint64, string, error) {
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

	s.mu.Lock()
	tx, err := s.contract.RecordSubTasks(s.authFor(ctx), new(big.Int).SetUint64(onChainTaskID), records)
	s.mu.Unlock()
	if err != nil {
		return nil, "", fmt.Errorf("onchain: recordSubTasks: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, s.ethClient, tx)
	if err != nil {
		return nil, "", fmt.Errorf("onchain: wait recordSubTasks: %w", err)
	}

	subTaskIDs := make([]uint64, 0, len(rows))
	for _, log := range receipt.Logs {
		event, err := s.contract.ParseSubTaskRecorded(*log)
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

func (s *OnchainService) CancelTask(ctx context.Context, onChainTaskID uint64) error {
	s.mu.Lock()
	tx, err := s.contract.CancelTask(s.authFor(ctx), new(big.Int).SetUint64(onChainTaskID))
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("onchain: cancelTask: %w", err)
	}
	_, err = bind.WaitMined(ctx, s.ethClient, tx)
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

// ExecuteTrade calls the real contract — subTaskID must already be
// on-chain (SubTaskRecorder.Record is chain-first, so this is guaranteed by
// the time submit_trade reaches this call), token is the stock's own ERC20
// contract address (used by the contract for a Sell; ignored for a Buy, which
// always pulls IDRX directly — passed through unconditionally regardless of
// side, since the contract decides what to do with it), and
// amount/minimumOutputAmount are already in the correct raw units for the
// given side (that conversion happens in submit_trade, not here).
func (s *OnchainService) ExecuteTrade(ctx context.Context, onChainTaskID uint64, subTaskID uint64, token common.Address, ticker string, side contracts.TradeSide, amount *big.Int, minimumOutputAmount *big.Int, summary string, reasoningHash [32]byte) (*agentcore.ExecuteTradeOutput, error) {
	if side == contracts.TradeSideBuy {
		if s.idrxAddress != (common.Address{}) {
			token = s.idrxAddress
		} else if idrxAddr, err := s.contract.Idrx(nil); err == nil && idrxAddr != (common.Address{}) {
			token = idrxAddr
		}
	}

	s.mu.Lock()
	tx, err := s.contract.ExecuteTrade(s.authFor(ctx),
		new(big.Int).SetUint64(onChainTaskID),
		new(big.Int).SetUint64(subTaskID),
		token, ticker, tradeSideEnum(side), amount, minimumOutputAmount, summary, reasoningHash,
	)
	s.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("onchain: executeTrade: %w", err)
	}
	return s.waitTradeByHash(ctx, tx.Hash().Hex(), amount, minimumOutputAmount)
}

// SubmitTradePair signs the permission grant and the trade with consecutive
// nonces and sends both in one JSON-RPC batch. It deliberately does not wait
// for the grant receipt: the nonce order guarantees the grant is mined first.
func (s *OnchainService) SubmitTradePair(
	ctx context.Context,
	onChainTaskID, subTaskID uint64,
	token common.Address, ticker string, side contracts.TradeSide,
	amount, minOut *big.Int, summary string, reasoningHash [32]byte,
	grantTotalBudget string, grantDurationSec int64,
	grantMaxPerTrade string, grantCooldown uint32,
	gasLimit uint64,
) (string, string, uint64, error) {
	budget, ok := new(big.Int).SetString(grantTotalBudget, 10)
	if !ok {
		return "", "", 0, fmt.Errorf("onchain: submitTradePair: invalid totalBudget %q", grantTotalBudget)
	}
	maxTrade := big.NewInt(0)
	if grantMaxPerTrade != "" {
		if m, ok := new(big.Int).SetString(grantMaxPerTrade, 10); ok {
			maxTrade = m
		}
	}
	if gasLimit == 0 {
		gasLimit = s.executionGasLimit
	}
	if side == contracts.TradeSideBuy {
		if s.idrxAddress != (common.Address{}) {
			token = s.idrxAddress
		} else if idrxAddr, err := s.contract.Idrx(nil); err == nil && idrxAddr != (common.Address{}) {
			token = idrxAddr
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	nonceStarted := time.Now()
	nonce, err := s.ethClient.PendingNonceAt(ctx, s.address)
	if err != nil {
		return "", "", 0, fmt.Errorf("onchain: pending nonce: %w", err)
	}
	slog.InfoContext(ctx, "onchain: submitPair nonce", "duration_ms", time.Since(nonceStarted).Milliseconds())

	feesStarted := time.Now()
	feeCap, tip := s.currentFees(ctx)
	slog.InfoContext(ctx, "onchain: submitPair fees", "duration_ms", time.Since(feesStarted).Milliseconds())
	grantOpts := *s.authFor(ctx)
	grantOpts.NoSend = true
	grantOpts.Nonce = new(big.Int).SetUint64(nonce)
	grantOpts.GasLimit = s.grantGasLimit
	if feeCap != nil {
		grantOpts.GasFeeCap = feeCap
	}
	if tip != nil {
		grantOpts.GasTipCap = tip
	}

	execOpts := grantOpts
	execOpts.Nonce = new(big.Int).SetUint64(nonce + 1)
	execOpts.GasLimit = gasLimit

	signStarted := time.Now()
	grantTx, err := s.contract.GrantTradePermission(&grantOpts,
		new(big.Int).SetUint64(onChainTaskID), budget,
		big.NewInt(grantDurationSec), maxTrade, new(big.Int).SetUint64(uint64(grantCooldown)),
	)
	if err != nil {
		return "", "", 0, fmt.Errorf("onchain: sign grantTradePermission: %w", err)
	}
	execTx, err := s.contract.ExecuteTrade(&execOpts,
		new(big.Int).SetUint64(onChainTaskID),
		new(big.Int).SetUint64(subTaskID),
		token, ticker, tradeSideEnum(side), amount, minOut, summary, reasoningHash,
	)
	if err != nil {
		return "", "", 0, fmt.Errorf("onchain: sign executeTrade: %w", err)
	}

	grantRaw, err := grantTx.MarshalBinary()
	if err != nil {
		return "", "", 0, fmt.Errorf("onchain: encode grant: %w", err)
	}
	execRaw, err := execTx.MarshalBinary()
	if err != nil {
		return "", "", 0, fmt.Errorf("onchain: encode execute: %w", err)
	}
	grantHex := hexutil.Encode(grantRaw)
	execHex := hexutil.Encode(execRaw)
	slog.InfoContext(ctx, "onchain: submitPair sign", "duration_ms", time.Since(signStarted).Milliseconds())

	var grantHash, execHash common.Hash
	batch := []rpc.BatchElem{
		{Method: "eth_sendRawTransaction", Args: []any{grantHex}, Result: &grantHash},
		{Method: "eth_sendRawTransaction", Args: []any{execHex}, Result: &execHash},
	}
	sendStarted := time.Now()
	batchErr := s.rpc.BatchCallContext(ctx, batch)
	if batchErr != nil {
		if err := s.rpc.CallContext(ctx, &grantHash, "eth_sendRawTransaction", grantHex); err != nil {
			return "", "", 0, fmt.Errorf("onchain: send grant: %w", err)
		}
		if err := s.rpc.CallContext(ctx, &execHash, "eth_sendRawTransaction", execHex); err != nil {
			return grantHash.Hex(), "", nonce, fmt.Errorf("onchain: send execute: %w", err)
		}
	} else {
		if batch[0].Error != nil {
			if err := s.rpc.CallContext(ctx, &grantHash, "eth_sendRawTransaction", grantHex); err != nil {
				return "", "", 0, fmt.Errorf("onchain: send grant: %w", err)
			}
		}
		if batch[1].Error != nil {
			if err := s.rpc.CallContext(ctx, &execHash, "eth_sendRawTransaction", execHex); err != nil {
				return grantHash.Hex(), "", nonce, fmt.Errorf("onchain: send execute: %w", err)
			}
		}
	}
	slog.InfoContext(ctx, "onchain: submitPair send", "duration_ms", time.Since(sendStarted).Milliseconds(), "batch_error", batchErr != nil)
	return grantHash.Hex(), execHash.Hex(), nonce, nil
}

// WaitTradeByHash waits for one executeTrade receipt and parses it. The 100ms
// poll is the only wait on the instant path.
func (s *OnchainService) WaitTradeByHash(ctx context.Context, txHash string) (*agentcore.ExecuteTradeOutput, error) {
	return s.waitTradeByHash(ctx, txHash, nil, nil)
}

func (s *OnchainService) waitTradeByHash(ctx context.Context, txHash string, amount, minOut *big.Int) (*agentcore.ExecuteTradeOutput, error) {
	hash := common.HexToHash(txHash)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	started := time.Now()
	polls := 0
	defer func() {
		slog.InfoContext(ctx, "onchain: waitTrade", "tx", txHash, "polls", polls, "duration_ms", time.Since(started).Milliseconds())
	}()

	for {
		polls++
		receipt, err := s.ethClient.TransactionReceipt(ctx, hash)
		if err == nil {
			if receipt.Status == 0 {
				return nil, fmt.Errorf("onchain: executeTrade reverted in %s", txHash)
			}
			return s.parseTradeReceipt(ctx, receipt, txHash, amount, minOut)
		}
		if !errors.Is(err, ethereum.NotFound) {
			return nil, fmt.Errorf("onchain: receipt %s: %w", txHash, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *OnchainService) parseTradeReceipt(ctx context.Context, receipt *types.Receipt, txHash string, amount, minOut *big.Int) (*agentcore.ExecuteTradeOutput, error) {
	for _, log := range receipt.Logs {
		if event, err := s.contract.ParseTradeExecuted(*log); err == nil {
			tradeID := event.TradeId.Uint64()
			out := &agentcore.ExecuteTradeOutput{
				TxHash:         txHash,
				TradeID:        tradeID,
				BlockNumber:    receipt.BlockNumber.Int64(),
				LogIndex:       int(log.Index),
				Amount:         amount,
				ReceivedAmount: minOut,
				ProtocolFee:    big.NewInt(0),
			}
			if tradeRecord, err := s.contract.Trades(&bind.CallOpts{Context: ctx}, event.TradeId); err == nil {
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
	return nil, fmt.Errorf("onchain: executeTrade: TradeExecuted event not found in receipt %s", txHash)
}

func (s *OnchainService) currentFees(ctx context.Context) (*big.Int, *big.Int) {
	s.feeMu.Lock()
	defer s.feeMu.Unlock()

	if s.feeCap != nil && s.feeTip != nil && time.Since(s.feeAt) < s.feeTTL {
		return new(big.Int).Set(s.feeCap), new(big.Int).Set(s.feeTip)
	}
	tip, err := s.ethClient.SuggestGasTipCap(ctx)
	if err != nil {
		if s.feeCap != nil && s.feeTip != nil {
			return new(big.Int).Set(s.feeCap), new(big.Int).Set(s.feeTip)
		}
		return nil, nil
	}
	head, err := s.ethClient.HeaderByNumber(ctx, nil)
	if err != nil || head.BaseFee == nil {
		if s.feeCap != nil && s.feeTip != nil {
			return new(big.Int).Set(s.feeCap), new(big.Int).Set(s.feeTip)
		}
		return nil, nil
	}
	feeCap := new(big.Int).Add(new(big.Int).Mul(head.BaseFee, big.NewInt(2)), tip)
	s.feeCap, s.feeTip, s.feeAt = feeCap, tip, time.Now()
	return new(big.Int).Set(feeCap), new(big.Int).Set(tip)
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

func (s *OnchainService) TradePermissionRemaining(ctx context.Context, onChainTaskID uint) (string, error) {
	permission, err := s.contract.TradePermissions(&bind.CallOpts{Context: ctx}, new(big.Int).SetUint64(uint64(onChainTaskID)))
	if err != nil {
		return "", fmt.Errorf("onchain: tradePermissions: %w", err)
	}
	remaining := new(big.Int).Sub(permission.TotalBudget, permission.UsedBudget)
	return remaining.String(), nil
}
