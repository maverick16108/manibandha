-- Пригласительные ссылки групп, тип (публичная/частная), скрытая история для новых участников.
ALTER TABLE public.chats ADD COLUMN IF NOT EXISTS invite_token text;
ALTER TABLE public.chats ADD COLUMN IF NOT EXISTS is_public boolean NOT NULL DEFAULT false;
ALTER TABLE public.chats ADD COLUMN IF NOT EXISTS hide_history boolean NOT NULL DEFAULT false;
-- seq на момент вступления участника: при скрытой истории он видит только сообщения после этого seq.
ALTER TABLE public.chat_members ADD COLUMN IF NOT EXISTS joined_seq bigint NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS chats_invite_token_uq ON public.chats (invite_token) WHERE invite_token IS NOT NULL;
