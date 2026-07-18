-- Закреплённое сообщение в чате (одно на чат).
ALTER TABLE public.chats ADD COLUMN IF NOT EXISTS pinned_message_id bigint;
