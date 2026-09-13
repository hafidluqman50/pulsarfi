-- agent_chat_messages.content_type gains "news" — a reply whose real
-- driver was sourced, dated evidence (analyzer_agent's own evidence array)
-- gets its own card, not just Supervisor's prose summary of it.
ALTER TABLE agent_chat_messages DROP CONSTRAINT IF EXISTS agent_chat_messages_content_type_check;
ALTER TABLE agent_chat_messages ADD CONSTRAINT agent_chat_messages_content_type_check
    CHECK (content_type IN ('text', 'workflow_card', 'chart', 'news'));
