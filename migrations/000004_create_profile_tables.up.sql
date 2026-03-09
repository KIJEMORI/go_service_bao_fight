CREATE TABLE IF NOT EXISTS profiles(
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL UNIQUE,
    nick_name VARCHAR(100) NOT NULL UNIQUE,
    user_name VARCHAR(100) NOT NULL,
    age INTEGER NOT NULL,
    gender VARCHAR(20) NOT NULL,
    sport VARCHAR(50),
    level VARCHAR(20),
    experience TEXT,
    photo_url VARCHAR(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_user_profile FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_profiles_user_id ON profiles(user_id);
CREATE INDEX idx_profiles_nick_name ON profiles(nick_name);
