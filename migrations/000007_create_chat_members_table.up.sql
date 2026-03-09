CREATE TABLE IF NOT EXISTS chat_members (
    chat_id UUID NOT NULL,
    user_id UUID NOT NULL,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (chat_id, user_id), -- Составной ключ для уникальности
    CONSTRAINT fk_chat FOREIGN KEY(chat_id) REFERENCES chats(id) ON DELETE CASCADE,
    CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_members_user_id ON chat_members(user_id); -- Чтобы быстро найти все чаты юзера
