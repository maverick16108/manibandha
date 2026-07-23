-- Соглашение в разделе «Вопросы гуру»: текст хранится в app_settings (расширяем value до text),
-- подтверждения пользователей — в отдельной таблице (по версии текста).
ALTER TABLE app_settings ALTER COLUMN value TYPE text;

CREATE TABLE IF NOT EXISTS question_agreement_acks (
  user_id integer PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  version integer NOT NULL DEFAULT 0
);

INSERT INTO app_settings (key, value) VALUES ('qagr_enabled', '1') ON CONFLICT (key) DO NOTHING;
INSERT INTO app_settings (key, value) VALUES ('qagr_version', '1') ON CONFLICT (key) DO NOTHING;
INSERT INTO app_settings (key, value) VALUES ('qagr_text',
'Дорогие преданные!

Гуру Махарадж искренне рад возможности поддерживать общение с каждым учеником и всегда ценит ваши вопросы, размышления и стремление получать наставления.

Вместе с тем, в связи с большим объемом служения, он не всегда имеет возможность отвечать на каждое сообщение лично. Поэтому часть вопросов могут рассматривать и отвечать на них квалифицированные наставники, которым Гуру Махарадж доверил это служение.

Если же вопрос требует его личного участия, касается важных духовных решений или обстоятельств, в которых необходимо непосредственное наставление Духовного Учителя, Гуру Махарадж обязательно ответит сам.

Благодарим вас за понимание, доверие и уважение к этому порядку общения.')
ON CONFLICT (key) DO NOTHING;
