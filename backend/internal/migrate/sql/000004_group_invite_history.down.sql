ALTER TABLE public.chat_members DROP COLUMN IF EXISTS joined_seq;
DROP INDEX IF EXISTS public.chats_invite_token_uq;
ALTER TABLE public.chats DROP COLUMN IF EXISTS hide_history;
ALTER TABLE public.chats DROP COLUMN IF EXISTS is_public;
ALTER TABLE public.chats DROP COLUMN IF EXISTS invite_token;
