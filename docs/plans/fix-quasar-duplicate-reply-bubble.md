# Quasar Duplicate Reply Bubble

| | |
|---|---|
| **Version** | 1.3 |
| **Status** | Draft |
| **Date Created** | 2026-10-21 |
| **Last Updated** | 2026-10-21 |

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-10-21 | Initial draft. |
| 1.1 | 2026-10-21 | Added the new-chat half: §1 notes the empty fetch + skeleton, §2 gains a no-fetch rule, §3/§5 gate the messages query on the chat existing (or the first send), §8 adds the network check. |
| 1.2 | 2026-10-21 | Replaced the per-turn refetch with client-owned message state: the reply is appended from the send response, `endedAt` no longer invalidates `agent-chat-messages`, and the DB hydrates only on mount/refresh. §1, §2, §3, §5, §7 and §8 revised; `hasSent` removed, `optimisticUserMessage`/`replyMessage`/`appendMessages` added. |
| 1.3 | 2026-10-21 | Root fix is the API contract, not client-side row fabrication: `WorkflowCard` now carries the rows the call created, `persistSupervisorReply` and `handleCancellation` return them, and the frontend appends them. §1, §2, §3, §5, §7 and §8 rewritten; `optimisticUserMessage`, `replyMessage` and the negative-id counter deleted. |

## 1. Problem Statement

A finished turn shows Quasar's reply twice, and **+ new chat** shows a loading
skeleton for a chat with nothing to load.

Three things combine:

1. #53 removed the `isStreaming` guard from the live bubble, so it renders as
   long as `streamingReplyText` is non-empty.
2. The send path refetches the chat after every turn. Between the refetched row
   landing and `clearTurnLive`, the persisted bubble and the live bubble render
   together.
3. The API hands the browser only the reply text. The service writes the user
   row and the supervisor row, then drops both:

   | Where | What happens |
   |---|---|
   | `agent_chat_message_repository.go:54` | `Create` returns the full row (`id`, `created_at`) |
   | `chat_service.go:140` | the user row is kept in memory |
   | `chat_service.go:285` | `runForMessage` returns only `WorkflowCard` |
   | `chat_service.go:326` | `persistSupervisorReply` writes the row with `if _, err := ...` and discards it |
   | `workflow_card.go` | the card has `TaskID`, `Reply`, `ContentType`, `UIComponent`, `UIProps` — no message row |
   | `chats.go:83` | the handler returns that card as-is |

   Because the response carries no row, the browser has to either refetch (the
   blink/duplicate above) or fabricate rows with synthetic ids (the v1.2
   workaround: `optimisticUserMessage`, `replyMessage`, a negative-id counter).

The fix is the missing half of the contract: the call returns the rows it
created, and the browser appends them. No refetch on the happy path, no
synthetic ids, no blink.

The new-chat half is separate: `useChatMessages` was always enabled, so a
client UUID with no row still issued a request and showed a skeleton.

## 2. Definition of Done

1. A send response carries the rows created by that call: the user row (unless
   hidden) and the supervisor row, with real `id`/`created_at`.
2. The frontend appends those rows and does **no** messages refetch after a
   successful turn.
3. Streaming still grows live, and the reply renders exactly once — no blink,
   no duplicate.
4. A brand-new chat issues **no** messages request and shows the empty intro
   immediately.
5. Retry returns only the supervisor row; cancellation returns both rows.
6. Charts, cards and `ui_ref_task_id` render from the returned row.
7. `go build`/`go vet`, the offline agent suite, `tsc --noEmit`, lint and the
   frontend build are green.

## 3. Feature Description

- `WorkflowCard` gains `Messages []model.AgentChatMessage` (JSON `messages`,
  omitempty). The `final` WebSocket event carries the same card, so both
  transports agree.
- `persistSupervisorReply` returns the row `Create` produced.
- `runForMessage` takes `includeUserRow`: true for a normal send, false for
  retry (the user row already exists client-side). It fills `card.Messages`.
- `handleCancellation` returns the user row and the confirmation row.
- If the row write fails, it is logged and omitted from `card.Messages`; the
  client still shows the reply text and picks the row up on the next fetch.
- Frontend: `appendMessages(queryClient, chatId, card.messages)`; the query is
  enabled only when the chat exists; the live bubble is hidden once `messages`
  contains the same reply. On an error the client invalidates messages once to
  recover the persisted user row (and Retry).

## 4. Impacted Files

- `backend/src/app/core/agent/workflow_card.go`
- `backend/src/service/agent/chat/chat_service.go`
- `backend/test/service/agent/` (assertion for the returned rows)
- `frontend/http/agent/chatApi.ts`
- `frontend/components/agent/ChatThread.tsx`
- `docs/plans/fix-quasar-duplicate-reply-bubble.md` (this document)

## 5. Code Shape

Backend:

```go
type WorkflowCard struct {
	TaskID      int64                    `json:"task_id,omitempty"`
	Reply       string                   `json:"reply"`
	ContentType string                   `json:"content_type"`
	UIComponent *string                  `json:"ui_component,omitempty"`
	UIProps     json.RawMessage          `json:"ui_props,omitempty"`
	Messages    []model.AgentChatMessage `json:"messages,omitempty"`
}
```

```go
func (svc *ChatService) runForMessage(..., intent agentcore.TurnIntent, includeUserRow bool) (agentcore.WorkflowCard, error)
func (svc *ChatService) persistSupervisorReply(...) (model.AgentChatMessage, agentcore.WorkflowCard)
```

```go
card.Messages = []model.AgentChatMessage{replyRow}
if includeUserRow {
    card.Messages = []model.AgentChatMessage{userMessage, replyRow}
}
orchestrator.PublishChatEvent(chatID, "final", card)
```

Frontend:

```ts
export interface WorkflowCard {
  task_id?: number;
  reply: string;
  content_type: 'text' | 'chart' | 'news';
  ui_component?: string | null;
  ui_props?: unknown;
  messages?: AgentChatMessage[];
}
```

```tsx
const card = await sendChatMessage(chatId, trimmed, hidden);
if (card.messages?.length) appendMessages(queryClient, chatId, card.messages);
if (!chatExists) void queryClient.invalidateQueries({ queryKey: ['agent-chats'] });
markTurnEnded(chatId);
```

Deleted symbols: `optimisticUserMessage`, `replyMessage`,
`nextOptimisticMessageId`, `optimisticMessageId`, `hasSent`.
New symbols: `WorkflowCard.Messages`, `appendMessages` (kept).

Call flow:

```text
send -> POST /agent/chats/:id/messages
     -> service writes user row + reply row
     -> response: WorkflowCard{messages:[user, reply]}
     -> append to agent-chat-messages -> clearTurnLive
open existing chat / reload -> GET messages once (DB)
```

## 6. UI/UX Changes (Lo-Fi)

None. The reply appears in the same place, once.

## 7. Flowchart

```text
send ──► service writes rows ──► response carries rows
                                      │
                                      ▼
                        append to messages cache
                                      │
                                      ▼
                     clearTurnLive + invalidate agent-tasks
                                      │
                                      ▼
                       [one user bubble, one reply bubble]

open / reload ──► GET messages once ──► [DB rows]
new chat      ──► no GET (chat has no row yet)
```

## 8. Verification Plan

1. `cd backend && go build ./... && go vet ./...`.
2. Offline agent suite; add an assertion that a send returns two rows
   (user + supervisor) and a retry returns one.
3. `cd frontend && npx tsc --noEmit && npm run lint && npm run build`.
4. Manual: send `halo` — one bubble, no second copy; DevTools: no
   `GET /agent/chats/<id>/messages` after the turn, no request at all when
   clicking "+ new chat".
5. Manual: chart answer renders one chart card from the returned row; retry
   after a failed turn shows the user row and one reply.
