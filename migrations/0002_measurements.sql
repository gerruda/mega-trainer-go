-- Body measurements: the data behind /chart, /history and /export.

CREATE TABLE IF NOT EXISTS measurements (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    weight_kg     DOUBLE PRECISION NOT NULL CHECK (weight_kg > 0),
    body_fat_pct  DOUBLE PRECISION,
    neck_cm       DOUBLE PRECISION,
    waist_cm      DOUBLE PRECISION,
    hips_cm       DOUBLE PRECISION,
    measured_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_measurements_user_measured
    ON measurements (user_id, measured_at DESC);
