-- Порядок закреплённых чатов (перетаскивание в списке), на участника.
ALTER TABLE public.chat_members ADD COLUMN IF NOT EXISTS pin_order integer NOT NULL DEFAULT 0;
