CREATE TABLE IF NOT EXISTS reactions (
    id UUID PRIMARY KEY,
    from_user_id UUID NOT NULL,
    to_user_id UUID NOT NULL,
    action VARCHAR(20) NOT NULL, -- 'like', 'dislike', 'skip'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_from_user FOREIGN KEY(from_user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_to_user FOREIGN KEY(to_user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(from_user_id, to_user_id)
);

CREATE INDEX idx_reactions_incoming_likes ON reactions (to_user_id) WHERE action = 'like';
CREATE INDEX idx_reactions_my_actions ON reactions (from_user_id, to_user_id);
CREATE INDEX idx_reactions_match_check ON reactions (from_user_id, to_user_id) WHERE action = 'like';
