BEGIN;

TRUNCATE TABLE agent_trades, agent_sub_tasks, agent_tasks, agent_chat_messages, agent_chats CASCADE;

ALTER TABLE agent_chat_messages DROP CONSTRAINT agent_chat_messages_chat_id_fkey;

ALTER TABLE agent_chats DROP CONSTRAINT agent_chats_pkey;
ALTER TABLE agent_chats DROP COLUMN id;
ALTER TABLE agent_chats ADD COLUMN id UUID PRIMARY KEY;

ALTER TABLE agent_chat_messages DROP COLUMN chat_id;
ALTER TABLE agent_chat_messages ADD COLUMN chat_id UUID NOT NULL;

ALTER TABLE agent_chat_messages
    ADD CONSTRAINT agent_chat_messages_chat_id_fkey FOREIGN KEY (chat_id) REFERENCES agent_chats(id);

CREATE INDEX idx_agent_chat_messages_chat_id ON agent_chat_messages(chat_id, created_at);

COMMIT;
