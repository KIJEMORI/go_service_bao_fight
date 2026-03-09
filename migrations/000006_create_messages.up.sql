CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY,
    chat_id UUID NOT NULL,          -- ID диалога или группы
    sender_id UUID NOT NULL,        -- Кто отправил
    text TEXT NOT NULL,
    payload JSONB,                  -- Для фото, вложений, реакций
    sequence_number BIGSERIAL,      -- Порядковый номер внутри чата (для контроля пропусков)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_message_chat FOREIGN KEY(chat_id) REFERENCES chats(id) ON DELETE CASCADE,
    CONSTRAINT fk_message_sender FOREIGN KEY(sender_id) REFERENCES users(id)
);

-- КРИТИЧЕСКИЙ ИНДЕКС: Поиск истории по чату с сортировкой по времени
CREATE INDEX idx_messages_chat_history ON messages (chat_id, created_at DESC);
