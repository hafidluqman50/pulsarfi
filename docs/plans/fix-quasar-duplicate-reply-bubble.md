# Quasar Duplicate Reply Bubble

| | |
|---|---|
| **Version** | 1.2 |
| **Status** | Draft |
| **Date Created** | 2026-10-21 |
| **Last Updated** | 2026-10-21 |

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-10-21 | Initial draft. |
| 1.1 | 2026-10-21 | Added the new-chat half: §1 notes the empty fetch + skeleton, §2 gains a no-fetch rule, §3/§5 gate the messages query on the chat existing (or the first send), §8 adds the network check. |
| 1.2 | 2026-10-21 | Replaced the per-turn refetch with client-owned message state: the reply is appended from the send response, `endedAt` no longer invalidates `agent-chat-messages`, and the DB hydrates only on mount/refresh. §1, §2, §3, §5, §7 and §8 revised; `hasSent` removed, `optimisticUserMessage`/`replyMessage`/`appendMessages` added. |

## 1. Problem Statement

A single-word message ("halo") renders Quasar's reply twice, and **+ new chat**
shows a loading skeleton for a chat that has nothing to load.

The duplicate is a regression from #53 (`chatSession`): before it, the live
streaming bubble was guarded by `isStreaming && streamingReplyText`; after it,
only by `streamingReplyText` (`ChatThread.tsx`). But the deeper cause is the
model, not the guard: the send path waits for the reply to reach Postgres and
then refetches it.

```text
turn ends -> markTurnEnded -> endedAt
  -> invalidate(agent-chat-messages) -> refetch lands
  -> invalidate(agent-tasks) -> clearTurnLive
```

Between "refetch lands" and `clearTurnLive`, `messages` contains the persisted
reply while `streamingReplyText` still holds the streamed copy, so both render.
A chatbot should not know or ask "has my message reached the database yet": it
appends the user's message, appends the reply it already has, and lets the DB
be the source only when the chat is opened or the page is reloaded.

The same mount always fetched messages: "+ new chat" is a client UUID with no
row, `useChatMessages` was always enabled, and the backend answers 200 `[]` —
but the request is in flight, so the thread shows a skeleton.

## 2. Definition of Done

1. A normal turn renders the reply once, with no blink.
2. Streaming still grows live while the turn runs.
3. A brand-new chat issues **no** messages request and shows the empty intro
   immediately; there is no loading skeleton.
4. A finished turn issues **no** messages refetch. The DB is consulted when an
   existing chat is opened and after a reload.
5. Charts, cards, hidden messages and retry behave as before.
6. Frontend typecheck/lint/build green.

## 3. Feature Description

- **Send** — on success, append the user row (unless the send is hidden) and
  the reply row built from the response's `WorkflowCard` to the
  `agent-chat-messages` cache. No messages invalidation.
- **First message of a new chat** — invalidate `agent-chats` so the sidebar
  shows the new chat; the messages query then enables and reconciles with the
  DB rows, which replace the optimistic ones.
- **Retry** — append the reply only; the user row is already there.
- **Turn end** (`endedAt`) — clear the live chrome and invalidate `agent-tasks`
  (PlanCard state). Never `agent-chat-messages`.
- **History** — `useChatMessages` is enabled only when the chat exists in the
  shared `agent-chats` cache, so a new chat never fetches.
- **Live bubble** — hidden while `messages` already contains that exact reply.
  This covers the frame between the append and `clearTurnLive`, and any later
  fetch that returns the same row.
- **Failure** — keep the user row in the cache so Retry stays available (the
  server persists it before running the turn; validation failures reconcile on
  the next natural fetch).

Optimistic rows carry negative ids; the next fetch replaces them with the
canonical DB rows, so React keys stay unique in the meantime.

## 4. Impacted Files

- `frontend/components/agent/ChatThread.tsx`
- `docs/plans/fix-quasar-duplicate-reply-bubble.md` (this document)

## 5. Code Shape

New helpers (module level):

```tsx
function optimisticUserMessage(chatId: string, content: string): AgentChatMessage
function replyMessage(chatId: string, card: WorkflowCard): AgentChatMessage
function appendMessages(queryClient: QueryClient, chatId: string, rows: AgentChatMessage[]): void
```

The send path:

```tsx
const card = await sendChatMessage(chatId, trimmed, hidden);
appendMessages(queryClient, chatId, hidden
  ? [replyMessage(chatId, card)]
  : [optimisticUserMessage(chatId, trimmed), replyMessage(chatId, card)]);
if (!chatExists) void queryClient.invalidateQueries({ queryKey: ['agent-chats'] });
markTurnEnded(chatId);
```

The retry path appends `replyMessage` only. The failure path appends the user
row only.

Turn end:

```tsx
useEffect(() => {
  if (!endedAt) return;
  clearTurnLive(chatId);
  void queryClient.invalidateQueries({ queryKey: ['agent-tasks'] });
}, [endedAt, chatId, queryClient]);
```

Query gate and live-bubble guard:

```tsx
const { data: chats = [] } = useAgentChats();
const chatExists = chats.some((chat) => chat.id === chatId);
const { data: messages = [], isLoading } = useChatMessages(chatId, { enabled: chatExists });

const liveReplyPersisted =
  liveReplyText !== '' &&
  messages.some((m) => m.sender !== 'user' && !isHiddenMessage(m) && m.content === liveReplyText);
```

New symbols: `optimisticUserMessage`, `replyMessage`, `appendMessages`,
`nextOptimisticMessageId`, `chatExists`, `liveReplyText`, `liveReplyPersisted`.
Deleted symbols: `hasSent`, `setHasSent`.

Call flow after the change:

```text
send -> pending bubble
     -> stream (reply_delta) -> live bubble
     -> response WorkflowCard
     -> append [user, reply] to agent-chat-messages
     -> markTurnEnded -> clearTurnLive + invalidate agent-tasks
open existing chat / reload -> fetch messages once (DB is the source)
```

## 6. UI/UX Changes (Lo-Fi)

None. The reply appears in the same place, once.

## 7. Flowchart

```text
send ──► [optimistic user bubble]
          │
          ├─ reply_delta ──► [live bubble]
          │
          └─ response ──► append user + reply to cache
                          │
                          ▼
                       clearTurnLive
                          │
                          ▼
             [one user bubble, one reply bubble]

open / reload ──► fetch history once ──► [DB rows]
```

## 8. Verification Plan

1. `cd frontend && npx tsc --noEmit` — no type errors.
2. `cd frontend && npm run lint`.
3. `cd frontend && npm run build`.
4. Manual, dev server: open Quasar, send `halo`.
   - While streaming: exactly one bubble, growing.
   - After the turn: exactly one bubble, no second copy, no blank frame.
5. Manual, long answer and chart answer: one reply / one chart card.
6. Manual, retry after a failed turn: the user row stays, Retry produces one
   reply.
7. DevTools Network:
   - click "+ new chat" → **no** `GET /agent/chats/<uuid>/messages`;
   - send `halo` → no messages refetch after the response;
   - open an existing chat or reload → exactly one messages fetch.
