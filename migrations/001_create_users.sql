-- +migrate Up
CREATE TABLE users (
                       id BIGSERIAL PRIMARY KEY,
                       telegram_id BIGINT UNIQUE NOT NULL,
                       username VARCHAR(255),
                       first_name VARCHAR(255),
                       last_name VARCHAR(255),
                       language_code VARCHAR(10),
                       is_bot BOOLEAN DEFAULT FALSE,
                       role VARCHAR(50) DEFAULT 'user',
                       created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                       updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE user_sessions (
                               id BIGSERIAL PRIMARY KEY,
                               user_id BIGINT REFERENCES users(telegram_id),
                               state VARCHAR(100),
                               data JSONB,
                               created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                               updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_users_telegram_id ON users(telegram_id);
CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);

-- +migrate Down
DROP TABLE user_sessions;
DROP TABLE users;