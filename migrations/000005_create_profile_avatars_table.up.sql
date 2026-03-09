CREATE TABLE IF NOT EXISTS profile_avatars (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    url VARCHAR(500) NOT NULL,
    is_current BOOLEAN DEFAULT FALSE, -- Текущая активная аватарка
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- При удалении юзера все его аватарки удаляются каскадно
    CONSTRAINT fk_user_avatars FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Индекс для быстрого получения всей истории аватарок пользователя (для листалки)
CREATE INDEX idx_profile_avatars_user_id ON profile_avatars(user_id);

-- Частичный индекс для мгновенного поиска текущей аватарки
CREATE INDEX idx_profile_avatars_current ON profile_avatars(user_id) WHERE is_current = TRUE;
