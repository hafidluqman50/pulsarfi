package agent_test

import (
    "bytes"
    "context"
    "crypto/ecdsa"
    "encoding/json"
    "fmt"
    "io"
    "math/big"
    "net/http"
    "os"
    "strings"
    "testing"
    "time"

    "github.com/ethereum/go-ethereum"
    "github.com/ethereum/go-ethereum/accounts"
    "github.com/ethereum/go-ethereum/accounts/abi"
    "github.com/ethereum/go-ethereum/accounts/abi/bind"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/google/uuid"
    "github.com/horizonlabs/pulsarfi-backend/src/auth"
)

type scenarioStep struct {
    name    string
    message string
    hidden  bool
}

type scenarioClient struct {
    t       *testing.T
    baseURL string
    token   string
    http    *http.Client
}

func (client *scenarioClient) call(method, path string, body any) (map[string]any, int, time.Duration) {
    client.t.Helper()
    var reader io.Reader
    if body != nil {
        encoded, err := json.Marshal(body)
        if err != nil {
            client.t.Fatalf("encode body: %v", err)
        }

        reader = bytes.NewReader(encoded)
    }

    request, err := http.NewRequest(method, client.baseURL+path, reader)
    if err != nil {
        client.t.Fatalf("build request: %v", err)
    }

    request.Header.Set("Content-Type", "application/json")
    if client.token != "" {
        request.Header.Set("Authorization", "Bearer "+client.token)
    }

    started := time.Now()
    response, err := client.http.Do(request)
    if err != nil {
        client.t.Fatalf("%s %s: %v", method, path, err)
    }

    defer response.Body.Close()
    raw, _ := io.ReadAll(response.Body)
    elapsed := time.Since(started)

    decoded := map[string]any{}
    _ = json.Unmarshal(raw, &decoded)
    return decoded, response.StatusCode, elapsed
}

func (client *scenarioClient) login() string {
    client.t.Helper()
    rawKey := strings.TrimPrefix(strings.TrimSpace(os.Getenv("SCENARIO_PRIVATE_KEY")), "0x")
    if rawKey == "" {
        client.t.Fatal("SCENARIO_PRIVATE_KEY not set: the scenario signs in with the wallet it is given, never a generated one")
    }

    key, err := crypto.HexToECDSA(rawKey)
    if err != nil {
        client.t.Fatalf("parse SCENARIO_PRIVATE_KEY: %v", err)
    }

    address := crypto.PubkeyToAddress(key.PublicKey).Hex()
    nonceBody, status, _ := client.call(http.MethodGet, "/api/v1/auth/nonce?address="+address, nil)
    nonceData, _ := nonceBody["data"].(map[string]any)
    nonce, _ := nonceData["nonce"].(string)
    if status != http.StatusOK || nonce == "" {
        client.t.Fatalf("nonce failed: status=%d body=%v", status, nonceBody)
    }

    message := auth.BuildSIWEMessage("localhost", address, nonce, time.Now().UTC().Format(time.RFC3339))
    signature, err := crypto.Sign(accounts.TextHash([]byte(message)), key)
    if err != nil {
        client.t.Fatalf("sign: %v", err)
    }

    signature[64] += 27
    verifyBody, status, _ := client.call(http.MethodPost, "/api/v1/auth/verify", map[string]any{
        "address":   address,
        "message":   message,
        "signature": fmt.Sprintf("0x%x", signature),
        "nonce":     nonce,
    })
    verifyData, _ := verifyBody["data"].(map[string]any)
    token, _ := verifyData["access_token"].(string)
    if status != http.StatusOK || token == "" {
        client.t.Fatalf("verify failed: status=%d body=%v", status, verifyBody)
    }

    client.token = token
    return address
}

func TestChatScenarioLive_TradeFromChatThroughArmApproveAndExecute(t *testing.T) {
    baseURL := os.Getenv("SCENARIO_BASE_URL")
    if baseURL == "" {
        t.Skip("SCENARIO_BASE_URL not set, skipping live scenario")
    }

    client := &scenarioClient{t: t, baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 5 * time.Minute}}
    address := client.login()
    chatID := uuid.NewString()
    t.Logf("wallet %s chat %s", address, chatID)

    steps := []scenarioStep{
        {name: "greeting", message: "Hai halo"},
        {name: "trade intent", message: "Bisa lakukan trade? Ke BBRIP sih"},
        {name: "answers 1", message: "side: beli\nshape: scalp\nconsult_nova: langsung eksekusi", hidden: true},
        {name: "answers 2", message: "idrx_cap: 1000000", hidden: true},
    }

    var lastTaskID int64
    for _, step := range steps {
        started := time.Now().Format("15:04:05.000")
        body, status, elapsed := client.call(http.MethodPost, "/api/v1/agent/chats/"+chatID+"/messages", map[string]any{"message": step.message, "hidden": step.hidden})
        data, _ := body["data"].(map[string]any)
        reply, _ := data["reply"].(string)
        contentType, _ := data["content_type"].(string)
        taskID, _ := data["task_id"].(float64)
        if len(reply) > 70 {
            reply = reply[:70] + "..."
        }

        t.Logf("%-13s start=%s took=%6.1fs status=%d task=%d type=%s reply=%q", step.name, started, elapsed.Seconds(), status, int64(taskID), contentType, strings.ReplaceAll(reply, "\n", " "))
        if status != http.StatusOK {
            t.Fatalf("step %q failed: status=%d body=%v", step.name, status, body)
        }

        if taskID != 0 {
            lastTaskID = int64(taskID)
        }
    }

    if lastTaskID == 0 {
        t.Fatal("the chat never opened a Task, nothing to arm")
    }

    armApproveExecute(t, client, scenarioKey(t), lastTaskID)
}

func scenarioKey(t *testing.T) *ecdsa.PrivateKey {
    t.Helper()
    parsed, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(os.Getenv("SCENARIO_PRIVATE_KEY")), "0x"))
    if err != nil {
        t.Fatalf("parse SCENARIO_PRIVATE_KEY: %v", err)
    }

    return parsed
}

func requireScenarioEnv(t *testing.T, name string) string {
    t.Helper()
    value := strings.TrimSpace(os.Getenv(name))
    if value == "" {
        t.Fatalf("%s not set", name)
    }

    return value
}

func armApproveExecute(t *testing.T, client *scenarioClient, walletKey *ecdsa.PrivateKey, taskID int64) {
    t.Helper()
    rpcURL := requireScenarioEnv(t, "SCENARIO_RPC_URL")
    tokenAddress := requireScenarioEnv(t, "SCENARIO_TOKEN_ADDRESS")
    managerAddress := requireScenarioEnv(t, "SCENARIO_MANAGER_ADDRESS")
    budgetRaw := requireScenarioEnv(t, "SCENARIO_BUDGET_RAW")

    armBody, status, elapsed := client.call(http.MethodPost, fmt.Sprintf("/api/v1/agent/tasks/%d/arm", taskID), map[string]any{
        "total_budget":  budgetRaw,
        "duration_sec":  86400,
        "token_address": tokenAddress,
    })
    t.Logf("%-13s start=%s took=%6.1fs status=%d", "arm", time.Now().Add(-elapsed).Format("15:04:05.000"), elapsed.Seconds(), status)
    if status != http.StatusOK {
        t.Fatalf("arm failed: status=%d body=%v", status, armBody)
    }

    approveStarted := time.Now()
    txHash := sendScenarioApprove(t, rpcURL, walletKey, common.HexToAddress(tokenAddress), common.HexToAddress(managerAddress), budgetRaw)
    t.Logf("%-13s start=%s took=%6.1fs tx=%s", "approve", approveStarted.Format("15:04:05.000"), time.Since(approveStarted).Seconds(), txHash)

    executeBody, status, elapsed := client.call(http.MethodPost, fmt.Sprintf("/api/v1/agent/tasks/%d/execute", taskID), nil)
    data, _ := executeBody["data"].(map[string]any)
    trades, _ := data["trades"].([]any)
    t.Logf("%-13s start=%s took=%6.1fs status=%d result=%v trades=%d", "execute", time.Now().Add(-elapsed).Format("15:04:05.000"), elapsed.Seconds(), status, data["status"], len(trades))
    if status != http.StatusOK || len(trades) == 0 {
        t.Fatalf("execute did not produce a trade: status=%d body=%v", status, executeBody)
    }
}

func sendScenarioApprove(t *testing.T, rpcURL string, walletKey *ecdsa.PrivateKey, token, spender common.Address, amountRaw string) string {
    t.Helper()
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
    defer cancel()

    client, err := ethclient.DialContext(ctx, rpcURL)
    if err != nil {
        t.Fatalf("dial rpc: %v", err)
    }

    defer client.Close()
    amount, ok := new(big.Int).SetString(amountRaw, 10)
    if !ok {
        t.Fatalf("invalid budget %q", amountRaw)
    }

    erc20, err := abi.JSON(strings.NewReader(`[{"name":"approve","type":"function","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]}]`))
    if err != nil {
        t.Fatalf("parse abi: %v", err)
    }

    data, err := erc20.Pack("approve", spender, amount)
    if err != nil {
        t.Fatalf("pack approve: %v", err)
    }

    from := crypto.PubkeyToAddress(walletKey.PublicKey)
    chainID, err := client.ChainID(ctx)
    if err != nil {
        t.Fatalf("chain id: %v", err)
    }

    nonce, err := client.PendingNonceAt(ctx, from)
    if err != nil {
        t.Fatalf("nonce: %v", err)
    }

    tip, err := client.SuggestGasTipCap(ctx)
    if err != nil {
        t.Fatalf("gas tip: %v", err)
    }

    head, err := client.HeaderByNumber(ctx, nil)
    if err != nil {
        t.Fatalf("header: %v", err)
    }

    gas, err := client.EstimateGas(ctx, ethereum.CallMsg{From: from, To: &token, Data: data})
    if err != nil {
        t.Fatalf("estimate gas: %v", err)
    }

    feeCap := new(big.Int).Add(new(big.Int).Mul(head.BaseFee, big.NewInt(2)), tip)
    transaction := types.NewTx(&types.DynamicFeeTx{ChainID: chainID, Nonce: nonce, GasTipCap: tip, GasFeeCap: feeCap, Gas: gas * 12 / 10, To: &token, Data: data})
    signed, err := types.SignTx(transaction, types.LatestSignerForChainID(chainID), walletKey)
    if err != nil {
        t.Fatalf("sign approve: %v", err)
    }

    if err := client.SendTransaction(ctx, signed); err != nil {
        t.Fatalf("send approve: %v", err)
    }

    receipt, err := bind.WaitMined(ctx, client, signed)
    if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
        t.Fatalf("approve not mined successfully: %v %+v", err, receipt)
    }

    return signed.Hash().Hex()
}
