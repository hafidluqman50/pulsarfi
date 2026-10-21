# Quasar Duplicate Reply Bubble

| | |
|---|---|
| **Version** | 1.4 |
| **Status** | Draft |
| **Date Created** | 2026-10-21 |
| **Last Updated** | 2026-10-21 |

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-10-21 | Initial draft. |
| 1.1 | 2026-10-21 | Added the new-chat half: §1 notes the empty fetch + skeleton, §2 gains a no-fetch rule, §3/§5 gate the messages query on the chat existing (or the first send), §8 adds the network check. |
| 1.2 | 2026-10-21 | Replaced the per-turn refetch with client-owned message state: the reply is appended from the send response, `endedAt` no longer invalidates `agent-chat-messages`, and the DB hydrates only on mount/refresh. §1, §2, §3, §5, §7 and §8 revised; `hasSent` removed, `optimisticUserMessage`/`replyMessage`/`appendMessages` added. |
| 1.3 | 2026-10-21 | Root fix is the API contract, not client-side row fabrication: `WorkflowCard` carries the rows the call created, `persistSupervisorReply` and `handleCancellation` return them, and the frontend appends them. §1, §2, §3, §5, §7 and §8 rewritten; `optimisticUserMessage`, `replyMessage` and the negative-id counter deleted. |
| 1.4 | 2026-10-21 | The response is the chat's state, so the client replaces instead of appending: `WorkflowCard.Messages` becomes the full chat after the turn (prior rows + the rows this call wrote), `includeUserRow` and `appendMessages` are removed, and the frontend does one `setQueryData` replace. §1–§3, §5, §7, §8 updated. |

## 1. Problem Statement

A finished turn showed Quasar's reply twice, and **+ new chat** showed a
loading skeleton.

Three things combined:

1. #53 removed the `isStreaming` guard from the live bubble, so it rendered as
   long as `streamingReplyText` was non-empty.
2. The send path refetched the chat after every turn. Between the refetched row
   landing and `clearTurnLive`, the persisted bubble and the live bubble
   rendered together.
3. The API returned only the reply text: the service wrote the user row and the
   supervisor row, then discarded both (`persistSupervisorReply` used
   `if _, err := Create(...)`). The browser therefore had to refetch (the
   blink/duplicate) or fabricate rows.

The new-chat half is separate: `useChatMessages` was always enabled, so a
client UUID with no row still issued a request and showed a skeleton.

The model we want is the plain chatbot one: while the user is in the chat, the
client owns the transcript; the DB is only read when the chat is opened or the
page is reloaded. The simplest contract for that is: **the send response is the
chat's current state** — the client replaces its list with it. No append rules,
no synthetic ids, no per-turn refetch.

## 2. Definition of Done

1. A send response carries the chat's full message list after the turn: prior
   rows + the rows this call wrote, with real `id`/`created_at`.
2. The frontend replaces its messages with that list (`setQueryData`), never
   refetching `agent-chat-messages` on a successful turn.
3. A reply renders exactly once: streaming grows live, then the live copy is
   dropped when the replaced list contains it.
4. A brand-new chat issues **no** messages request and shows the empty intro
   immediately.
5. Retry and cancellation return the same full-list shape.
6. Charts, cards and `ui_ref_task_id` render from the returned rows.
7. `go build`/`go vet`, the offline agent suite, `tsc --noEmit`, lint and the
   frontend build are green.

## 3. Feature Description

- `WorkflowCard.Messages` is the **full chat** after the turn, not just the new
  rows.
- `runForMessage` already holds `allMessages` (the history plus the user row for
  a send; the history for a retry). It appends the persisted supervisor row and
  puts the result in `card.Messages`.
- `handleCancellation` reads the history after writing the user row, then
  appends the confirmation row.
- If the supervisor write fails, the list is returned without it (it is logged)
  and the client picks the reply up on the next natural fetch.
- Frontend: `setQueryData(['agent-chat-messages', chatId], card.messages)` — a
  replace. `appendMessages` and `includeUserRow` are deleted.
- The messages query is enabled only when the chat exists, so a new chat never
  fetches. `agent-chats` is invalidated after the first message so the sidebar
  shows the chat.
- Error path: one messages refetch to recover the persisted user row for Retry;
  the happy path never refetches.

Trade-off, stated on purpose: the response carries the whole chat each turn.
Chats here are short trading conversations; a cursor/pagination contract is
out of scope until a chat can grow enough for this to matter.

## 4. Impacted Files

- `backend/src/app/core/agent/workflow_card.go`
- `backend/src/service/agent/chat/chat_service.go`
- `backend/test/service/agent/` (assertions for the full list)
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
	Messages    []model.AgentChatMessage `json:"messages,omitempty"` // full chat after the turn
}
```

```go
// runForMessage
replyRow, card := svc.persistSupervisorReply(ctx, chatID, result)
card.Messages = allMessages
if replyRow.ID != 0 {
	card.Messages = append(card.Messages, replyRow)
}
orchestrator.PublishChatEvent(chatID, "final", card)
return card, nil
```

```go
// handleCancellation
card := agentcore.WorkflowCard{Reply: cancelReply, ContentType: "text", Messages: existingMessages}
if replyRow.ID != 0 {
	card.Messages = append(card.Messages, replyRow)
}
```

Frontend:

```ts
export interface WorkflowCard {
  task_id?: number;
  reply: string;
  content_type: 'text' | 'chart' | 'news';
  ui_component?: string | null;
  ui_props?: unknown;
  messages?: AgentChatMessage[]; // full chat after the turn
}
```

```tsx
const card = await sendChatMessage(chatId, trimmed, hidden);
if (card.messages) queryClient.setQueryData(['agent-chat-messages', chatId], card.messages);
if (!chatExists) void queryClient.invalidateQueries({ queryKey: ['agent-chats'] });
markTurnEnded(chatId);
```

Deleted symbols: `includeUserRow`, `appendMessages`, `optimisticUserMessage`,
`replyMessage`, `nextOptimisticMessageId`.
New symbols: none.

Call flow:

```text
send -> service writes user + supervisor rows
     -> response: WorkflowCard{messages: [full chat]}
     -> setQueryData replaces the list -> clearTurnLive
open existing chat / reload -> GET messages once (DB)
new chat -> no GET
```

## 6. UI/UX Changes (Lo-Fi)

None. The reply appears in the same place, once.

## 7. Flowchart

```text
send ──► service writes rows
             │
             ▼
     response = full chat
             │
             ▼
   replace messages state (setQueryData)
             │
             ▼
   clearTurnLive + invalidate agent-tasks
             │
             ▼
   [one user bubble, one reply bubble]

open / reload ──► GET messages once
new chat      ──► no GET
```

## 8. Verification Plan

1. `cd backend && go build ./... && go vet ./...`.
2. Offline agent suite: a send returns the full list ending in the user row and
   the supervisor row (real ids); a retry returns the full list ending in one
   new supervisor row; cancellation returns both new rows.
3. `cd frontend && npx tsc --noEmit && npm run lint && npm run build`.
4. Manual: send `halo` — one bubble; no `GET /agent/chats/<id>/messages` after
   the turn; `+ new chat` issues no request.
5. Manual: chart answer renders one chart card; a second send keeps the first
   turn visible (the response replaces the list with both turns).
