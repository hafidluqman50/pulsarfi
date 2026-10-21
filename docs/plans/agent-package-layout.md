# Agent Package Layout

| | |
|---|---|
| **Version** | 1.1 |
| **Status** | Implemented |
| **Date Created** | 2026-10-20 |
| **Last Updated** | 2026-10-20 |

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-10-20 | Initial draft. |
| 1.1 | 2026-10-20 | Marked Implemented. Target Tree updated to the files that shipped (execute_result.go, turn_intent.go, chat_history.go, helper/strings.go); Staging notes the four commits and their verification. |

## 1. Problem Statement

`backend/src/service/agent` is a flat package of ~3,500 lines across 16 files. It mixes domain types, per-turn runtime state, pure helpers, and services that touch Postgres and the chain. The maintainer knows the rest of the codebase but not this layer, and the flat package gives no scope to open.

## 2. Rules

1. **Anything that talks to the chain lives in `external`, named for what it is.** `client_service.go` faces the chain (RPC, signer, `AgentTaskManager` contract). It is the app's on-chain service, so it moves to `service/external/onchain_service.go` as `OnchainService` / `NewOnchainService`. No agent-specific name: `external` must not be tied to the agent domain.
2. **`agent` (core) holds no I/O.** `app/core/agent` contains types, interfaces (ports) and pure/in-memory logic. No `repository`, `gorm`, `pgx`, `go-ethereum`, no HTTP.
3. **Domain services keep their persistence.** `SubTaskRecorder` writes Postgres and calls the chain through a port; that is domain work, so it stays under `service/agent/`.

There is no `external/onchain` folder, no `agent/chain`, and no `agent_task_client` name. One on-chain service, one place: `external/onchain_service.go`.

## 3. Consequences for `RunContext` (core purity)

| Today | After |
|---|---|
| `RunContext.Recorder *SubTaskRecorder` | `RunContext.Recorder SubTaskWriter` — interface in core; implementation in `service/agent/subtask/` |
| `RunContext.Stream *turnStream` | `RunContext.Stream TurnStream` — interface in core; `turnStream`/`liveSubTasks` in `service/agent/orchestrator/` |
| `ExtendedCheckPointStore` + Postgres impl in one file | interface in `ports.go`; Postgres impl in `service/agent/checkpoint/` |

Call sites keep the same method names, so tools and the workflow compile unchanged.

## 4. Target Tree

```text
src/
  app/
    app.go, bootstrap.go, helpers.go        # package app, unchanged
    core/
      agent/                                # package agentcore — NO repository/gorm/eth
        errors.go
        decision.go                         # RouteDecision
        interrupts.go                       # QuestionsInterrupt, ArmInterrupt
        workflow_card.go                    # WorkflowCard
        execute_output.go                   # ExecuteTradeOutput
        execute_result.go                   # ExecuteTaskResult (orchestrator + task share it)
        turn_intent.go                      # TurnIntent, TurnKind, FreshTurnIntent
        chat_history.go                     # ChatHistory (model rows -> eino messages)
        ports.go                            # SubTaskWriter, TurnStream, ChainSubTaskRecorder, ExtendedCheckPointStore
        run_context.go                      # RunContext, WithRunContext, RunContextFrom
        interrupt_store.go                  # InterruptStore (policy over the store port)
        turn_registry.go                    # TurnRegistry (in-memory)
        instructions.go                     # GlobalInstructions, CurrentTimeContext
    helper/                                 # package helper — pure
      reply_classifier.go
      nova_verdict.go
      graceful_tool.go
      strings.go
  config/
    database.go, env.go                     # unchanged
    onchain.go                              # OnchainConfig + LoadOnchain()
  service/
    external/
      onchain_service.go                    # moved from service/agent/client_service.go — the on-chain service
      deepseek_service.go, email_service.go, price_service.go, storage_service.go,
      chain_verify_service.go, stream_service.go, broadcaster.go   # unchanged
    agent/
      orchestrator/
        orchestrator_service.go             # Send/dispatch/resume, ResumeExecute
        stream_service.go                   # consume, turnStream/liveSubTasks, publish, finish*
        workflow_service.go                 # CommitTask/UpdateTask, on-chain calls, trigger
      chat/
        chat_service.go                     # ChatService, resolveTurn
      task/
        task_service.go                     # TaskService: arm/disarm/pause/resume/execute/reasoning
        scheduler_service.go                # TaskScheduler
      subtask/
        recorder_service.go                 # SubTaskRecorder (Postgres + chain via port)
      checkpoint/
        checkpoint_service.go               # PostgresCheckPointStore
      supervisor/ analyzer/ executor/       # unchanged location; imports updated
      README.md                             # flow map + "which file owns what"
```

### Why `subtask/` is not inside `task/`

They are complementary in the domain, but not in Go: `task` imports `orchestrator` (for `ResumeExecute`), and `orchestrator` imports the recorder (its workflow builds it). Putting the recorder inside `task` would make `orchestrator → task → orchestrator` a cycle. `subtask/` is therefore its own package: `orchestrator → subtask`, `task → orchestrator`. Nothing about it is chain-specific — it uses the `ChainSubTaskRecorder` port, whose implementation is `external.OnchainService`.

Dependency direction (one-way, acyclic):

```text
config            <- external, service (registry)
agentcore         <- helper, orchestrator, chat, task, subtask, checkpoint, tools, external
helper            <- orchestrator, tools
subtask           <- orchestrator
orchestrator      <- chat, task, supervisor (as TaskOpener)
checkpoint        <- orchestrator, chat
external          <- service (registry)
```

`agentcore` imports only `model` (value types in ports), `contracts`, `uuid`, `sync`, `time`.

## 5. On-chain Configuration

`src/config/onchain.go`:

```go
type OnchainConfig struct {
    RPCURL                string
    AgentWalletPrivateKey string
    AgentTaskManager      string
    PulsarProtocol        string
    IDRXAddress           string
}

func LoadOnchain() (OnchainConfig, error) // reads + validates ALCHEMY_RPC_URL,
                                           // AGENT_WALLET_PRIVATE_KEY, AGENT_TASK_MANAGER_ADDRESS;
                                           // PULSAR_PROTOCOL and IDRX_ADDRESS are optional
```

`external.NewOnchainService(ctx, cfg config.OnchainConfig)` takes it; the two adapters in `agent_registry.go` take the same struct. `os.Getenv` disappears from agent/external.

## 6. Staging (one branch, one commit per phase, each green)

| Phase | Content | Risk |
|---|---|---|
| 1 | `config/onchain.go`; `app/core/agent/execute_output.go` (the DTO, so the moved service can return it); `client_service.go` → `external/onchain_service.go` (`OnchainService`) | Low |
| 2 | `app/helper/` (reply classifier, Nova verdict, graceful) | Low |
| 3 | `app/core/agent/` (remaining types + ports); `RunContext` fields → interfaces; `subtask/`, `checkpoint/` | Medium — field types change, call sites do not |
| 4 | `service/agent/{orchestrator,chat,task}/`; handlers, registry, tests; `README.md` | Medium |

Delivered as four commits on `refactor/agent-package-layout`, each built from
`main` and verified before the next one started:

1. `refactor(agent): move the on-chain service to external`
2. `refactor(agent): move the pure helpers to app/helper`
3. `refactor(agent): make core I/O-free with ports`
4. `refactor(agent): split orchestrator, chat and task into folders`

## 7. Impacted Files

- **Moved**: `client_service.go` → `service/external/onchain_service.go`; `sub_task_recorder.go` → `service/agent/subtask/`; `checkpoint_store_service.go` → `service/agent/checkpoint/`; the rest of `service/agent/*.go` into `app/core/agent/`, `app/helper/`, `service/agent/{orchestrator,chat,task}/`.
- **New**: `config/onchain.go`, `app/core/agent/execute_output.go`, `app/core/agent/ports.go`, `service/agent/README.md`.
- **Import-only**: `agent_registry.go`, `service/index.go`, all `http/handlers/agent/*.go`, `service/agent/{supervisor,analyzer,executor}/*.go`, `backend/test/service/agent/*.go`.
- **Untouched**: behaviour, schema, frontend, other `external` services.

## 8. Verification

1. After each phase: `cd backend && go build ./... && go vet ./...`.
2. Offline suite: `go test ./test/service/agent/ -race -run 'TestTurnRegistry|TestChatTurn_SurvivesACancelledRequest|TestChatRetry|TestToolBudget|TestFlow|TestEinoProof|TestChatService|TestPostgresCheckPointStore|TestTaskService_ExecuteTaskValidation|TestScalpRefactor|TestTrustedSources|TestStockRepository|TestVerifyTickerTool|TestReadArticleTool|TestNewNewsTools_Construction|TestStockChartTool|TestSearchTool'`.
3. Purity checks (all empty):
   - `grep -rn "src/repository\|gorm\|pgx\|go-ethereum" src/app/core/agent`
   - `grep -rn "ethclient\|abi/bind\|abi\." src/service/agent src/app/core/agent` — empty. `common.Address` (address value) and `crypto.Keccak256Hash` (hashing) are value-level uses at the contract boundary and are allowed.
   - `grep -rn "os.Getenv" src/service/agent src/service/external/onchain_service.go src/app/core/agent`
4. Boundary check: `grep -rniE "agent\." src/service/external/onchain_service.go` → no import of the agent packages (the `subTask*` helpers and `model.AgentSubTask` are the contract's own vocabulary, not the agent domain).
5. Import cycles are caught by `go build`.
6. Result after phase 4 (2026-10-20): `go build ./...` and `go vet ./...` green;
   the offline suite PASS (53s); `grep -rn 'src/service/agent"' src test` empty;
   `grep -rn 'os.Getenv' src/service/agent src/app/core/agent` empty.

## 9. Risks

- **Churn**: ~3,500 lines change package. Mechanical; staging keeps every commit green.
- **Interface indirection**: `RunContext.Recorder`/`.Stream` become interfaces; the concrete `*SubTaskRecorder` is no longer reachable from tools — that is the point.
- **Not worth moving** `supervisor/analyzer/executor`: their location is right; only imports change.
