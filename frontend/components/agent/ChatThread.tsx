'use client';

import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { streamChatMessage, retryLastMessage } from '@/http/agent/chatApi';
import type { AgentSubTask } from '@/http/agent/taskApi';
import { PlanCard } from './PlanCard';
import { PortfolioChart } from './PortfolioChart';
import { statusColor, StructuredOrProse } from './SubTaskReasoning';
import { useChatMessages } from '@/http/agent/hooks';

type ChatThreadProps = {
  chatId: string;
};

function LiveSubTasks({ subTasks }: { subTasks: AgentSubTask[] }) {
  const [openRow, setOpenRow] = useState<number | null>(null);
  if (subTasks.length === 0) return null;
  return (
    <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '10px 13px', display: 'flex', alignItems: 'center', gap: 9 }}>
        <span className="pulsar" />
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>Working</span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--hairline-strong)' }}>{subTasks.length} sub tasks so far</span>
      </div>
      {subTasks.map((subTask) => {
        const color = statusColor(subTask.status);
        const isOpen = openRow === subTask.id;
        return (
          <div key={subTask.id} className="rise" style={{ borderBottom: '1px solid var(--hairline)' }}>
            <button
              onClick={() => setOpenRow(isOpen ? null : subTask.id)}
              style={{ width: '100%', appearance: 'none', border: 0, cursor: 'pointer', background: 'transparent', padding: '11px 13px', textAlign: 'left', display: 'block' }}
            >
              <span style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--ticker)', flex: 'none' }}>
                  {String(subTask.step_order).padStart(2, '0')}
                </span>
                <span style={{ fontSize: 14, fontWeight: 600, lineHeight: 1.3, flex: '1 1 120px', minWidth: 0 }}>{subTask.step_name}</span>
                <span style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 7, flex: 'none' }}>
                  <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)' }}>{subTask.agent}</span>
                  <span style={{ font: '600 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color, border: `1px solid ${color}`, padding: '3px 4px', whiteSpace: 'nowrap' }}>
                    {subTask.status.toUpperCase()}
                  </span>
                </span>
              </span>
            </button>
            {isOpen && (
              <div style={{ margin: '0 13px 12px 35px', borderLeft: '1px solid var(--hairline)', paddingLeft: 12 }}>
                <div style={{ font: '600 8.5px/1.2 var(--font-sans)', letterSpacing: '.12em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 6 }}>
                  Reasoning · {subTask.agent}
                </div>
                <div style={{ marginBottom: subTask.output ? 14 : 0 }}>
                  <StructuredOrProse raw={subTask.reasoning} />
                </div>
                {subTask.output && (
                  <>
                    <div style={{ font: '600 8.5px/1.2 var(--font-sans)', letterSpacing: '.12em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 8 }}>
                      Output
                    </div>
                    <StructuredOrProse raw={subTask.output} />
                  </>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function MessageMarkdown({ content }: { content: string }) {
  return (
    <div style={{ fontSize: 14.5, lineHeight: 1.55 }}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }) => <p style={{ margin: '0 0 10px' }}>{children}</p>,
          strong: ({ children }) => <strong style={{ fontWeight: 600, color: 'var(--ink)' }}>{children}</strong>,
          ul: ({ children }) => <ul style={{ margin: '0 0 10px', paddingLeft: 20 }}>{children}</ul>,
          ol: ({ children }) => <ol style={{ margin: '0 0 10px', paddingLeft: 20 }}>{children}</ol>,
          li: ({ children }) => <li style={{ marginBottom: 4 }}>{children}</li>,
          table: ({ children }) => (
            <div style={{ overflowX: 'auto', margin: '0 0 10px' }}>
              <table style={{ borderCollapse: 'collapse', width: '100%', fontSize: 13 }}>{children}</table>
            </div>
          ),
          thead: ({ children }) => <thead style={{ borderBottom: '1px solid var(--hairline-strong)' }}>{children}</thead>,
          th: ({ children }) => <th style={{ textAlign: 'left', padding: '6px 10px', fontWeight: 600, color: 'var(--ink)' }}>{children}</th>,
          td: ({ children }) => <td style={{ padding: '6px 10px', borderTop: '1px solid var(--hairline)' }}>{children}</td>,
          code: ({ children }) => (
            <code style={{ fontFamily: 'var(--font-mono)', fontSize: 12.5, background: 'var(--canvas-soft)', padding: '1px 4px' }}>{children}</code>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}

function ChartCard({ uiProps }: { uiProps: unknown }) {
  const props = uiProps as { lens?: string; ticker?: string; lensNote?: string; data?: unknown } | undefined;
  if (!props?.lens) return null;
  return (
    <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', marginBottom: 10, paddingBottom: 9, borderBottom: '1px solid var(--canvas-soft)' }}>
        <span style={{ font: '700 9px/1.3 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--merah)', border: '1px solid var(--merah)', padding: '3px 5px', whiteSpace: 'nowrap' }}>
          {props.lens}
        </span>
        {props.lensNote && <span style={{ fontSize: 11.5, color: 'var(--body)', lineHeight: 1.4 }}>{props.lensNote}</span>}
      </div>
      <PortfolioChart payload={{ lens: props.lens, ticker: props.ticker, lensNote: props.lensNote, data: props.data }} />
    </div>
  );
}

export function ChatThread({ chatId }: ChatThreadProps) {
  const { data: messages = [], isLoading } = useChatMessages(chatId);
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState('');
  const [pendingText, setPendingText] = useState<string | null>(null);
  const [failedMessage, setFailedMessage] = useState<{ text: string; description: string } | null>(null);
  const [liveSubTasks, setLiveSubTasks] = useState<AgentSubTask[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const isSendingRef = useRef(false);
  const threadRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = threadRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [messages, pendingText, liveSubTasks, isStreaming]);

  useEffect(() => {
    if (!pendingText) return;
    const alreadyPersisted = messages.some((m) => m.sender === 'user' && m.content === pendingText);
    if (alreadyPersisted) setPendingText(null);
  }, [messages, pendingText]);

  async function handleSend(overrideText?: string) {
    const trimmed = (overrideText ?? draft).trim();
    if (!trimmed || isSendingRef.current) return;
    isSendingRef.current = true;
    setIsStreaming(true);
    setPendingText(trimmed);
    setFailedMessage(null);
    setLiveSubTasks([]);
    if (!overrideText) setDraft('');
    try {
      await streamChatMessage(chatId, trimmed, (event) => {
        if (event.type === 'sub_task') {
          setLiveSubTasks((prev) => [...prev, event.data]);
        }
      });
      queryClient.invalidateQueries({ queryKey: ['agent-chat-messages', chatId] });
      queryClient.invalidateQueries({ queryKey: ['agent-tasks'] });
    } catch (error: unknown) {
      const description = error instanceof Error ? error.message : 'Something went wrong';
      setFailedMessage({ text: trimmed, description });
      toast.error('Message failed to send', { description });
    } finally {
      isSendingRef.current = false;
      setPendingText(null);
      setLiveSubTasks([]);
      setIsStreaming(false);
    }
  }

  async function handleRetry() {
    if (isSendingRef.current) return;
    isSendingRef.current = true;
    setIsStreaming(true);
    setFailedMessage(null);
    setLiveSubTasks([]);
    try {
      await retryLastMessage(chatId, (event) => {
        if (event.type === 'sub_task') {
          setLiveSubTasks((prev) => [...prev, event.data]);
        }
      });
      queryClient.invalidateQueries({ queryKey: ['agent-chat-messages', chatId] });
      queryClient.invalidateQueries({ queryKey: ['agent-tasks'] });
    } catch (error: unknown) {
      const description = error instanceof Error ? error.message : 'Something went wrong';
      setFailedMessage({ text: messages[messages.length - 1]?.content ?? '', description });
      toast.error('Message failed to send', { description });
    } finally {
      isSendingRef.current = false;
      setLiveSubTasks([]);
      setIsStreaming(false);
    }
  }

  return (
    <>
      <div ref={threadRef} className="thread" style={{ flex: 1, overflowY: 'auto', padding: '16px 14px', display: 'flex', flexDirection: 'column', gap: 14 }}>
        {isLoading && <div className="skeleton" style={{ height: 80, width: '100%' }} />}

        {messages.map((message, index) => {
          const isLast = index === messages.length - 1;
          const needsRetry = isLast && message.sender === 'user' && !isStreaming && !pendingText;
          const retryDescription = needsRetry && failedMessage?.text === message.content ? failedMessage.description : 'No reply received for this message yet.';

          return message.sender === 'user' ? (
            <div key={message.id} className="rise" style={{ background: 'var(--merah-soft)', border: `1px solid ${needsRetry ? 'var(--negative)' : 'var(--merah-line)'}`, borderRight: `2px solid ${needsRetry ? 'var(--negative)' : 'var(--merah)'}`, padding: '13px 15px', marginLeft: 'clamp(18px,6vw,34px)' }}>
              <div style={{ fontSize: 14.5, lineHeight: 1.55, color: 'var(--ink-soft)' }}>{message.content}</div>
              {needsRetry && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 9, paddingTop: 9, borderTop: '1px solid var(--merah-line)' }}>
                  <span style={{ fontSize: 11.5, color: 'var(--negative)' }}>{retryDescription}</span>
                  <button
                    onClick={handleRetry}
                    style={{ marginLeft: 'auto', appearance: 'none', cursor: 'pointer', border: '1px solid var(--negative)', background: 'transparent', color: 'var(--negative)', font: '600 11px/1 var(--font-mono)', padding: '6px 10px', flex: 'none' }}
                  >
                    Retry
                  </button>
                </div>
              )}
            </div>
          ) : (
            <div key={message.id} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {message.ui_ref_task_id != null && <PlanCard taskId={message.ui_ref_task_id} chatId={chatId} />}
              <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
                <MessageMarkdown content={message.content} />
              </div>
              {message.content_type === 'chart' && <ChartCard uiProps={message.ui_props} />}
            </div>
          );
        })}

        {pendingText && (
          <div className="rise" style={{ background: 'var(--merah-soft)', border: '1px solid var(--merah-line)', borderRight: '2px solid var(--merah)', padding: '13px 15px', marginLeft: 'clamp(18px,6vw,34px)', opacity: 0.6 }}>
            <div style={{ fontSize: 14.5, lineHeight: 1.55, color: 'var(--ink-soft)' }}>{pendingText}</div>
          </div>
        )}

        {isStreaming && liveSubTasks.length === 0 && (
          <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--canvas)', padding: '13px 15px', display: 'flex', alignItems: 'center', gap: 10 }}>
            <span className="pulsar" />
            <span style={{ fontSize: 12.5, color: 'var(--body)', fontFamily: 'var(--font-mono)' }}>Quasar is thinking…</span>
          </div>
        )}

        {isStreaming && <LiveSubTasks subTasks={liveSubTasks} />}
      </div>

      <div style={{ flex: 'none', borderTop: '1px solid var(--hairline)', background: 'var(--canvas)', padding: '10px 12px 12px' }}>
        <div style={{ border: '1px solid var(--ink)', background: 'var(--putih)', display: 'flex', alignItems: 'flex-end', gap: 0 }}>
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSend();
              }
            }}
            rows={2}
            placeholder="Ask or instruct Quasar…"
            style={{ flex: 1, border: 0, background: 'transparent', padding: '11px 12px', font: '400 14px/1.5 var(--font-sans)', color: 'var(--ink)', outline: 'none', resize: 'none' }}
          />
          <button
            onClick={() => handleSend()}
            disabled={!draft.trim() || isStreaming || pendingText != null}
            style={{ appearance: 'none', border: 0, cursor: 'pointer', background: 'var(--merah)', color: 'var(--putih)', font: '600 12px/1 var(--font-sans)', padding: '14px 13px', flex: 'none', alignSelf: 'stretch' }}
          >
            Send
          </button>
        </div>
        <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--ticker)', marginTop: 7 }}>only this box is treated as a command</div>
      </div>
    </>
  );
}
