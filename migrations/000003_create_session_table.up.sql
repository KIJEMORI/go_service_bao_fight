CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    refresh_token TEXT UNIQUE NOT NULL,
    user_agent TEXT,
    client_ip TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Индекс для очистки старых сессий
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
-- Индекс для поиска сессий конкретного пользователя
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
