-- 025_add_horizon_notice_to_chat_messages.sql
-- Allow 'horizon_notice' in content_type and 'HorizonNoticeCard' in ui_component
-- for proactive H-1 horizon expiry notices dispatched by the scheduler.

ALTER TABLE agent_chat_messages ALTER COLUMN content_type TYPE VARCHAR(30);
ALTER TABLE agent_chat_messages DROP CONSTRAINT IF EXISTS agent_chat_messages_content_type_check;
ALTER TABLE agent_chat_messages ADD CONSTRAINT agent_chat_messages_content_type_check
    CHECK (content_type IN ('text', 'workflow_card', 'chart', 'news', 'horizon_notice'));

ALTER TABLE agent_chat_messages ALTER COLUMN ui_component TYPE VARCHAR(50);
ALTER TABLE agent_chat_messages DROP CONSTRAINT IF EXISTS agent_chat_messages_ui_component_check;
ALTER TABLE agent_chat_messages ADD CONSTRAINT agent_chat_messages_ui_component_check
    CHECK (ui_component IN ('plan_tracker', 'clarifying_questions', 'compiled_rule', 'HorizonNoticeCard'));
