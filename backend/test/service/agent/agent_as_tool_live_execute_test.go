package agent_test

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/realtime"
	"github.com/joho/godotenv"
)

// Does what the Arm button does, for one Task whose Arm pause is waiting, with
// the demo retail wallet's own key: ArmTask (grantTradePermission), the
// wallet's ERC20 approve, then ExecuteTask, which resumes the pause and lets
// Comet trade for real on Arbitrum Sepolia. It runs only when
// LIVE_ARM_TASK_ID names the Task on purpose, and it refuses to sign unless
// the key really belongs to the wallet that owns that Task.
const approveABI = `[{"name":"approve","type":"function","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]}]`

func TestLiveAgentAsTool_ArmAndExecute(t *testing.T) {
	taskIDText := os.Getenv("LIVE_ARM_TASK_ID")
	if os.Getenv("LIVE_AGENT_FLOW") != "1" || taskIDText == "" {
		t.Skip("set LIVE_AGENT_FLOW=1 and LIVE_ARM_TASK_ID=<task id>: this test signs and sends real testnet transactions")
	}
	taskID, err := strconv.ParseInt(taskIDText, 10, 64)
	if err != nil {
		t.Fatalf("LIVE_ARM_TASK_ID: %v", err)
	}
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Logf("no .env loaded (%v), relying on already-exported environment", err)
	}
	ctx := context.Background()

	// The key is read from the file into memory only, never into the process environment.
	signerEnv, err := godotenv.Read("../../../../smart-contract/.env")
	if err != nil {
		t.Fatalf("read smart-contract/.env: %v", err)
	}
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(signerEnv["DEMO_INVESTOR_RETAIL_PRIVATE_KEY"]), "0x"))
	if err != nil {
		t.Fatalf("DEMO_INVESTOR_RETAIL_PRIVATE_KEY is missing or malformed")
	}
	signer := crypto.PubkeyToAddress(privateKey.PublicKey)

	db, err := config.NewDatabase(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	repos := repository.NewRegistry(db)
	registry := service.NewRegistry(service.Config{Repos: repos})
	if registry.AgentTask == nil || registry.AgentChat == nil {
		t.Fatal("agent services disabled: check the DeepSeek, Alchemy, agent wallet and task manager settings in .env")
	}

	task, found, err := repos.AgentTask.FindByID(ctx, taskID)
	if err != nil || !found {
		t.Fatalf("task %d not found: %v", taskID, err)
	}
	if !strings.EqualFold(task.WalletAddress, signer.Hex()) {
		t.Fatalf("refusing to sign: task %d belongs to %s, the key belongs to %s", taskID, task.WalletAddress, signer.Hex())
	}
	if !task.IsActionable || task.ArmedAt != nil || task.OnChainTaskID == nil {
		t.Fatalf("task %d is not an unarmed, actionable, on-chain Task", taskID)
	}
	var trigger struct {
		Side       string `json:"side"`
		BudgetIDRX string `json:"confirmed_budget_idrx"`
	}
	if err := json.Unmarshal([]byte(*task.TriggerDescription), &trigger); err != nil || trigger.Side != "buy" || trigger.BudgetIDRX == "" {
		t.Fatalf("this test only arms a buy Task with a confirmed IDRX budget, got side=%q budget=%q", trigger.Side, trigger.BudgetIDRX)
	}
	budgetIDRX, ok := new(big.Int).SetString(trigger.BudgetIDRX, 10)
	if !ok {
		t.Fatalf("budget %q is not a number", trigger.BudgetIDRX)
	}
	rawBudget := new(big.Int).Mul(budgetIDRX, big.NewInt(100)) // IDRX has 2 decimals

	chatID := chatOfTask(t, repos, taskID)
	stored, hasPause, _ := registry.AgentChat.CheckPointStore.GetInterruptID(ctx, chatID.String())
	if !hasPause || !strings.HasPrefix(stored, "arm|") {
		t.Fatalf("task %d has no Arm pause waiting (%q): ExecuteTask could not resume it", taskID, stored)
	}

	conn := newCaptureConn()
	go realtime.Default().Serve(conn, &auth.Claims{WalletAddress: signer.Hex(), Role: "user"})
	defer close(conn.incoming)
	conn.subscribe(agentsvc.ChatStreamTopic(chatID))

	idrx := common.HexToAddress(os.Getenv("IDRX_ADDRESS"))
	manager := common.HexToAddress(os.Getenv("AGENT_TASK_MANAGER_ADDRESS"))

	// 1. ArmTask: the server grants the on-chain trade permission.
	arm, err := registry.AgentTask.ArmTask(ctx, taskID, signer.Hex(), agentsvc.ArmTaskInput{
		TotalBudget: rawBudget.String(), DurationSec: 24 * 60 * 60, TokenAddress: idrx.Hex(),
	})
	if err != nil {
		t.Fatalf("ArmTask: %v", err)
	}
	t.Logf("[arm] granted trade permission: on_chain_task_id=%d total_budget_raw=%s", arm.OnChainTaskID, arm.TotalBudget)

	// 2. The wallet's own approve(IDRX -> AgentTaskManager, budget).
	client, err := ethclient.DialContext(ctx, os.Getenv("ALCHEMY_RPC_URL"))
	if err != nil {
		t.Fatalf("dial rpc: %v", err)
	}
	approveHash := sendApprove(t, ctx, client, privateKey, signer, idrx, manager, rawBudget)
	t.Logf("[approve] confirmed on-chain: https://sepolia.arbiscan.io/tx/%s", approveHash)

	// 3. ExecuteTask resumes the Arm pause; Comet trades.
	started := time.Now()
	result, err := registry.AgentTask.ExecuteTask(ctx, taskID, signer.Hex())
	if err != nil {
		t.Fatalf("ExecuteTask: %v", err)
	}
	summary := summarize(conn.drain())
	t.Logf("[execute] took %s, status=%s, trades=%d", time.Since(started).Round(time.Second), result.Status, len(result.Trades))
	t.Logf("[execute] reply: %s", strings.TrimSpace(result.Reply))
	t.Logf("[execute] live events after Arm: %v | reply_delta chunks=%d | thinking=%v", summary.eventTypes, summary.replyDeltas, summary.thinking)
	t.Logf("[execute] tool_calls: %v", summary.toolCalls)
	t.Logf("[execute] sub_tasks (last snapshot): %v", summary.subTasks)
	for _, trade := range result.Trades {
		t.Logf("[execute] trade: %s", fmt.Sprintf("%+v", trade))
	}

	rows, _ := repos.AgentSubTask.FindByTaskID(ctx, taskID)
	var steps []string
	for _, row := range rows {
		steps = append(steps, row.Agent+"/"+row.StepName)
	}
	t.Logf("[execute] persisted sub task chain: %v", steps)

	if result.Status != "executed" || len(result.Trades) == 0 {
		t.Errorf("expected an executed trade, got status=%q trades=%d", result.Status, len(result.Trades))
	}
	if summary.thinking["executor"] == 0 {
		t.Errorf("expected Comet's thinking to stream live after Arm, got %v", summary.thinking)
	}
	if found, _ := registry.AgentChat.CheckPointStore.Has(ctx, chatID.String()); found {
		t.Errorf("the checkpoint must be gone after the execution")
	}
}

func chatOfTask(t *testing.T, repos *repository.Registry, taskID int64) uuid.UUID {
	t.Helper()
	message, found, err := repos.AgentChatMessage.FindByUIRefTaskID(context.Background(), taskID)
	if err != nil || !found {
		t.Fatalf("no chat message references task %d: %v", taskID, err)
	}
	return message.ChatID
}

func sendApprove(t *testing.T, ctx context.Context, client *ethclient.Client, key *ecdsa.PrivateKey, from, token, spender common.Address, amount *big.Int) string {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(approveABI))
	if err != nil {
		t.Fatalf("abi: %v", err)
	}
	data, err := parsed.Pack("approve", spender, amount)
	if err != nil {
		t.Fatalf("pack approve: %v", err)
	}
	chainID, err := client.ChainID(ctx)
	if err != nil || chainID.Int64() != 421614 {
		t.Fatalf("refusing to send: expected Arbitrum Sepolia (421614), got %v (%v)", chainID, err)
	}
	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		t.Fatalf("tip: %v", err)
	}
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	feeCap := new(big.Int).Add(new(big.Int).Mul(head.BaseFee, big.NewInt(2)), tip)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{From: from, To: &token, Data: data})
	if err != nil {
		t.Fatalf("estimate gas (the approve would revert): %v", err)
	}
	transaction := types.NewTx(&types.DynamicFeeTx{
		ChainID: chainID, Nonce: nonce, GasTipCap: tip, GasFeeCap: feeCap, Gas: gas * 12 / 10, To: &token, Data: data,
	})
	signed, err := types.SignTx(transaction, types.LatestSignerForChainID(chainID), key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := client.SendTransaction(ctx, signed); err != nil {
		t.Fatalf("send approve: %v", err)
	}
	receipt, err := bind.WaitMined(ctx, client, signed)
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("approve did not succeed on-chain: %v", err)
	}
	return signed.Hash().Hex()
}
