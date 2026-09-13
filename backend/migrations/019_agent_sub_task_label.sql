-- agent_sub_tasks.label: a short, human-readable description of this step,
-- authored by the calling model in whichever language the conversation is
-- in (e.g. "Meneruskan ke Analyzer untuk cek tren VKTR" vs "Routing to
-- Analyzer to check VKTR's trend") — step_name stays a fixed, technical,
-- language-independent identifier for the hash chain and audit trail;
-- label is purely what the UI displays. Nullable: existing rows never had
-- one, the UI falls back to a humanized step_name for those.
ALTER TABLE agent_sub_tasks ADD COLUMN IF NOT EXISTS label TEXT;
