'use client';

import { useRef, useState } from 'react';
import { useSendChatMessage } from '@/http/agent/hooks';
import { type CardContract, parseCardContract } from './cardContract';

type IntakeField = {
  key: string;
  question: string;
  why?: string;
  options?: string[];
};

type ClarifyingQuestionsProps = {
  uiProps: unknown;
  chatId: string;
};

function formatNumberWithDots(val: string | number): string {
  const digits = String(val).replace(/\D/g, '');
  if (!digits) return '';
  return Number(digits).toLocaleString();
}

function formatBudgetHumanReadable(val: string | number): string {
  const digits = String(val).replace(/\D/g, '');
  const num = Number(digits);
  if (!num || isNaN(num) || num <= 0) return '';
  return `${num.toLocaleString()} IDRX`;
}

function isAmountQuestion(q: IntakeField): boolean {
  const key = q.key.toLowerCase();
  if (key === 'idrx_cap' || key === 'budget' || key === 'budget_idrx') return true;
  return /idrx|budget|anggaran|modal|dana|nominal|how much/i.test(q.question);
}

export function ClarifyingQuestions({ uiProps, chatId }: ClarifyingQuestionsProps) {
  const parsed = uiProps as { questions?: IntakeField[]; card?: CardContract } | undefined;
  const questions = parsed?.questions ?? [];
  const contract = parsed?.card ?? parseCardContract(null);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const sendMessage = useSendChatMessage(chatId);
  const isSendingRef = useRef(false);

  if (questions.length === 0) return null;

  const answeredCount = questions.filter((q) => (draft[q.key] ?? '').trim() !== '').length;
  const allAnswered = answeredCount === questions.length;

  const [isCancelled, setIsCancelled] = useState(false);

  if (isCancelled) {
    return (
      <div className="rise" style={{ border: '1px solid var(--hairline-strong)', background: 'var(--canvas-subtle, #f6f6f6)' }}>
        <div
          style={{
            background: 'var(--hairline-strong, #888)',
            color: 'var(--putih)',
            padding: '8px 13px',
            display: 'flex',
            alignItems: 'center',
            gap: 9,
          }}
        >
          <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>
            {contract.ledger.disarmed_title.replace('· Task T-{id}', '').trim()}
          </span>
        </div>
        <div style={{ padding: '11px 13px', fontSize: 12.5, color: 'var(--body)' }}>
          {contract.ledger.disarmed_desc}
        </div>
      </div>
    );
  }

  function cancelPlan() {
    if (isSendingRef.current || sendMessage.isPending) return;
    isSendingRef.current = true;
    setIsCancelled(true);
    sendMessage.mutate(contract.ledger.disarm_button, {
      onSettled: () => {
        isSendingRef.current = false;
      },
    });
  }

  function send() {
    if (!allAnswered || isSendingRef.current) return;
    isSendingRef.current = true;
    const body = questions.map((q) => `${q.question}: ${draft[q.key].trim()}`).join('\n');
    sendMessage.mutate(body, {
      onSettled: () => {
        isSendingRef.current = false;
      },
    });
  }

  return (
    <div className="rise" style={{ border: '1px solid var(--merah)', background: 'var(--putih)' }}>
      <div
        style={{
          background: 'var(--merah)',
          color: 'var(--putih)',
          padding: '10px 13px',
          display: 'flex',
          alignItems: 'center',
          gap: 9,
        }}
      >
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>
          {contract.arm_title_ready}
        </span>
        <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: 11 }}>
          {answeredCount} / {questions.length}
        </span>
      </div>

      {questions.map((q, i) => {
        const value = draft[q.key] ?? '';
        const isDone = Boolean(value);
        return (
          <div key={q.key} style={{ borderBottom: '1px solid var(--hairline)', padding: '11px 13px' }}>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--ticker)', flex: 'none' }}>
                {String(i + 1).padStart(2, '0')}
              </span>
              <span style={{ fontSize: 14, fontWeight: 600, lineHeight: 1.35, flex: 1, minWidth: 0 }}>{q.question}</span>
              <span
                style={{
                  font: '600 9px/1.3 var(--font-sans)',
                  letterSpacing: '.1em',
                  textTransform: 'uppercase',
                  color: isDone ? 'var(--positive)' : 'var(--merah)',
                  border: `1px solid ${isDone ? 'var(--positive)' : 'var(--merah)'}`,
                  padding: '3px 4px',
                  whiteSpace: 'nowrap',
                  flex: 'none',
                }}
              >
                {isDone ? (contract.status_labels['done'] || 'DONE') : (contract.status_labels['needs_input'] || 'NEEDS INPUT')}
              </span>
            </div>

            {q.why && (
              <div style={{ fontSize: 11.5, color: 'var(--body)', lineHeight: 1.45, margin: '5px 0 0 20px' }}>{q.why}</div>
            )}

            <div style={{ margin: '9px 0 0 20px', display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {(q.options ?? []).map((opt) => (
                <button
                  key={opt}
                  onClick={() => setDraft((prev) => ({ ...prev, [q.key]: opt }))}
                  style={{
                    appearance: 'none',
                    cursor: 'pointer',
                    border: `1px solid ${value === opt ? 'var(--ink)' : 'var(--hairline-strong)'}`,
                    background: value === opt ? 'var(--ink)' : 'transparent',
                    color: value === opt ? 'var(--canvas)' : 'var(--ink)',
                    font: '500 12px/1 var(--font-sans)',
                    padding: '7px 10px',
                  }}
                >
                  {opt}
                </button>
              ))}
              {(q.options ?? []).length === 0 && (
                isAmountQuestion(q) ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 7, flex: 1, minWidth: 200 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <input
                        value={formatNumberWithDots(value)}
                        onChange={(e) => {
                          const raw = e.target.value.replace(/\D/g, '');
                          setDraft((prev) => ({ ...prev, [q.key]: raw ? Number(raw).toLocaleString() : '' }));
                        }}
                        placeholder={contract.budget_placeholder}
                        style={{
                          flex: 1,
                          minWidth: 160,
                          appearance: 'none',
                          border: '1px solid var(--hairline-strong)',
                          background: 'var(--putih)',
                          color: 'var(--ink)',
                          font: '600 13px/1.35 var(--font-mono)',
                          padding: '8px 10px',
                        }}
                      />
                      <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--body)', flex: 'none' }}>IDRX</span>
                    </div>

                    {value && (
                      <div style={{ fontSize: 11.5, fontFamily: 'var(--font-mono)', color: 'var(--positive)', background: 'var(--canvas-soft)', border: '1px solid var(--hairline)', padding: '4px 8px', alignSelf: 'flex-start' }}>
                        ✓ {formatBudgetHumanReadable(value)}
                      </div>
                    )}

                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 2 }}>
                      <span style={{ fontSize: 9.5, color: 'var(--ticker)', alignSelf: 'center', marginRight: 2, fontFamily: 'var(--font-sans)', textTransform: 'uppercase', letterSpacing: '.08em' }}>
                        {contract.preset_label}
                      </span>
                      {contract.presets.map((p) => (
                        <button
                          key={p.value}
                          type="button"
                          onClick={() => setDraft((prev) => ({ ...prev, [q.key]: p.value }))}
                          style={{
                            appearance: 'none',
                            cursor: 'pointer',
                            border: `1px solid ${value === p.value ? 'var(--ink)' : 'var(--hairline-strong)'}`,
                            background: value === p.value ? 'var(--ink)' : 'transparent',
                            color: value === p.value ? 'var(--canvas)' : 'var(--body)',
                            font: '500 10.5px/1 var(--font-mono)',
                            padding: '4px 7px',
                          }}
                        >
                          {p.label}
                        </button>
                      ))}
                    </div>
                  </div>
                ) : (
                  <input
                    value={value}
                    onChange={(e) => setDraft((prev) => ({ ...prev, [q.key]: e.target.value }))}
                    placeholder={contract.needs_input.placeholder}
                    style={{
                      flex: 1,
                      minWidth: 160,
                      appearance: 'none',
                      border: '1px solid var(--hairline-strong)',
                      background: 'var(--putih)',
                      color: 'var(--ink)',
                      font: '500 12.5px/1.35 var(--font-sans)',
                      padding: '8px 10px',
                    }}
                  />
                )
              )}
            </div>
          </div>
        );
      })}

      <div style={{ padding: '11px 13px' }}>
        <div style={{ fontSize: 11.5, color: 'var(--body)', lineHeight: 1.5, marginBottom: 10 }}>
          {contract.no_trade_description}
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button
            onClick={send}
            disabled={!allAnswered || sendMessage.isPending}
            style={{
              appearance: 'none',
              border: 0,
              cursor: !allAnswered || sendMessage.isPending ? 'not-allowed' : 'pointer',
              flex: 1,
              background: !allAnswered || sendMessage.isPending ? 'var(--hairline-strong)' : 'var(--merah)',
              color: 'var(--putih)',
              font: '600 13px/1 var(--font-sans)',
              padding: 12,
            }}
          >
            {sendMessage.isPending
              ? contract.button_labels.executing
              : allAnswered
              ? contract.needs_input.button
              : `${questions.length - answeredCount} ${contract.subtask_unit}`}
          </button>
          <button
            type="button"
            onClick={cancelPlan}
            disabled={sendMessage.isPending}
            style={{
              appearance: 'none',
              border: '1px solid var(--hairline-strong)',
              cursor: sendMessage.isPending ? 'not-allowed' : 'pointer',
              background: 'transparent',
              color: 'var(--body)',
              font: '500 12.5px/1 var(--font-sans)',
              padding: '12px 14px',
              whiteSpace: 'nowrap',
            }}
          >
            {contract.ledger.disarm_button}
          </button>
        </div>
      </div>
    </div>
  );
}
