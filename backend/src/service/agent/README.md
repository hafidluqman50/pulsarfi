# Agent service

Map of the agent layer: who owns what, and where to look when a turn
misbehaves.

## One turn, end to end

```text
WS  /agent/chats/:id/messages
 |
 +- chat.ChatService.HandleChatMessage            chat/chat_service.go
 |    resolveTurn       fresh | answer | side question | cancel
 |    persist the user message
 |    TurnRegistry      one turn per chat (core/turn_registry.go)
 |    runForMessage     detached from the request, 5-minute timeout
 |
 +- orchestrator.Orchestrator.Send                orchestrator/orchestrator_service.go
 |    dispatchTurn
 |      FreshTurn            -> runFreshTurn
 |      AnswerQuestions      -> resumeQuestionsTurn
 |      SideQuestionDuringArm-> runFreshTurn (arm pause stays recorded)
 |    consume                                     orchestrator/stream_service.go
 |      reads the Eino event stream, publishes reasoning/tool steps
 |    finish / finishInterrupt                    orchestrator/stream_service.go
 |      one pause  -> record the interrupt, keep the checkpoint
 |      no pause   -> clear checkpoint, persist the reply
 |
 +- chat persists the reply and publishes final / error
```

`ResumeExecute` (Arm Card) enters at `orchestrator_service.go` and runs the
same `consume` path.

## Where a pause lives

| Thing | Owner |
|---|---|
| Is this chat paused, and on what? | `agentcore.InterruptStore` (`app/core/agent/interrupt_store.go`) |
| The interrupt bytes | `checkpoint.PostgresCheckPointStore` (`checkpoint/checkpoint_service.go`) |
| What the user answered | `orchestrator` resume path |

`ask_user` and `await_arm` are the only two interrupt sources; both are in
`supervisor/tools_service.go`.

## Ownership

| Path | Owns | Does not own |
|---|---|---|
| `app/core/agent` | domain types, ports, pause policy, turn guard, instructions | Postgres, chain, HTTP |
| `app/helper` | reply classification, Nova verdict parsing, graceful tool, string helpers | any state |
| `config/onchain.go` | reading and validating the five chain settings | calling the chain |
| `service/external/onchain_service.go` | every chain call (RPC, signer, AgentTaskManager) | chat/turn state |
| `service/agent/orchestrator` | turn dispatch, resume, event stream, task workflow | chat persistence |
| `service/agent/chat` | chats, messages, turn intent, retry | prompt construction |
| `service/agent/task` | arm/disarm/pause/execute/reasoning, cooldown scheduler | LLM calls |
| `service/agent/subtask` | hash-chained sub-task rows (Postgres + chain port) | turn state |
| `service/agent/checkpoint` | Eino checkpoint persistence | pause policy |
| `supervisor` `analyzer` `executor` | the three agent roles and their tools | persistence, HTTP |

## Debugging recipes

| Symptom | Start here |
|---|---|
| Turn never finishes | `orchestrator/stream_service.go` `consume` + `finish` |
| Resume goes to the wrong place | `chat/chat_service.go` `resolveTurn`, then `InterruptStore.Pending` |
| "A turn is already running" | `agentcore.TurnRegistry` (`TryStart` / `Finish`) |
| No chart / wrong content type | `app/helper/reply_classifier.go` `ClassifyReply` |
| Nova verdict ignored | `app/helper/nova_verdict.go` `ParseNovaVerdict` |
| Tool failure swallowed | `app/helper/graceful_tool.go` (logs `tool=`, returns `tool_error`) |
| Sub-task chain broken | `subtask/recorder_service.go` `Record` |
| Pause survives a restart? | `checkpoint/checkpoint_service.go` |

Tests live in `backend/test/service/agent/` (per `AGENTS.md`).
