-- up
CREATE TABLE IF NOT EXISTS users (
                                     id BIGINT PRIMARY KEY,
                                     created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS calculations (
                                            id SERIAL PRIMARY KEY,
                                            user_id BIGINT NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMP NOT NULL,
    metadata JSONB
    );

CREATE INDEX ON calculations (user_id, type, created_at DESC);

-- down
DROP TABLE calculations;
DROP TABLE users;