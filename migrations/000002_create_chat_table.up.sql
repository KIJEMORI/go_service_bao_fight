CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY,
    title VARCHAR(255),               -- Название для групп, NULL для диалогов
    is_group BOOLEAN DEFAULT FALSE,   -- Флаг: группа это или личка
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
