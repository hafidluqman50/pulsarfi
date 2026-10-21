# Quasar Duplicate Reply Bubble

| | |
|---|---|
| **Version** | 1.0 |
| **Status** | Draft |
| **Date Created** | 2026-10-21 |
| **Last Updated** | 2026-10-21 |

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-10-21 | Initial draft. |

## 1. Problem Statement

A single-word message ("halo") renders Quasar's reply twice. This is a
regression from #53 (`chatSession`): before it, the live streaming bubble was
guarded at render time by `isStreaming && streamingReplyText`; after it, the
guard is only `streamingReplyText` (`ChatThread.tsx:408`).

The persisted supervisor row and the live bubble are the same text. When a turn
ends:

1. the backend persists the reply, then publishes `final`;
2. `markTurnEnded` sets `endedAt`;
3. the `endedAt` effect refetches the chat messages, then calls
   `clearTurnLive` — only after **two** `invalidateQueries` promises resolve.

Between step 3's refetch and `clearTurnLive`, `messages` already contains the
persisted reply while `streamingReplyText` still holds the streamed copy, so
`MessageList` renders both: one live bubble, one persisted bubble.

Short replies make the window obvious (the whole reply is one chunk); long
replies hide it behind the scrolling.

## 2. Definition of Done

1. A normal turn renders the reply once, from stream to persisted, with no
   blink and no moment where both exist.
2. Streaming still grows live while the turn runs.
3. No behaviour change for charts, cards, hidden messages, retry, or
   `clearTurnLive`.
4. Frontend typecheck/lint/build green.

## 3. Feature Description

Derive, at render time, whether the live text has already landed in the
persisted history. If it has, pass an empty streaming text to `MessageList`;
otherwise keep passing it (no blink while the refetch is in flight).

Equality is exact: the backend persists `result.Reply`, which is the same
string the stream accumulated in `outcome.reply`.

## 4. Impacted Files

- `frontend/components/agent/ChatThread.tsx`
- `docs/plans/fix-quasar-duplicate-reply-bubble.md` (this document)

## 5. Code Shape

Before:

```tsx
{streamingReplyText && (
  <div className="rise" style={{ maxWidth: '92%' }}>
    ...
    <MessageMarkdown content={streamingReplyText} fontSize={15} />
  </div>
)}
```

After (ChatThread derives the prop; MessageList is unchanged):

```tsx
const liveReplyText = session.streamingReplyText;
const liveReplyPersisted =
  liveReplyText !== '' &&
  messages.some((message) =>
    message.sender !== 'user' && !isHiddenMessage(message) && message.content === liveReplyText,
  );
```

```tsx
streamingReplyText={liveReplyPersisted ? '' : liveReplyText}
```

New symbols: `liveReplyText`, `liveReplyPersisted` (locals).
Deleted symbols: none.

Call flow after the change:

```text
WS reply_delta -> session.streamingReplyText grows -> live bubble renders
turn ends -> final -> endedAt -> refetch messages
  persisted reply found with the same content -> liveReplyPersisted = true
  -> live bubble hidden, persisted bubble renders
```

## 6. UI/UX Changes (Lo-Fi)

None. The reply appears in the same place, once.

## 7. Flowchart

```text
reply_delta ──► session.streamingReplyText ──► [live bubble]
final ──► endedAt ──► refetch ──► messages contains reply?
                                        │ yes                 │ no
                                        ▼                     ▼
                              hide live bubble        keep live bubble
                              (persisted renders)     (no blink yet)
```

## 8. Verification Plan

1. `cd frontend && npx tsc --noEmit` — no type errors.
2. `cd frontend && npm run lint`.
3. `cd frontend && npm run build`.
4. Manual, dev server: open Quasar, send `halo`.
   - While streaming: exactly one bubble, growing.
   - After the turn: exactly one bubble (the persisted one), no second copy.
5. Manual, long answer: streaming still grows, then swaps to the persisted
   message without a blank frame and without a duplicate.
6. Manual, chart answer: exactly one chart card, no duplicate text.
