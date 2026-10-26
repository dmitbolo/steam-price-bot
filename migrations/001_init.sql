CREATE TABLE IF NOT EXISTS subscriptions (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    app_id INT NOT NULL,
    game_name TEXT NOT NULL,
    target_price INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chat_id, app_id)
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_app_id ON subscriptions(app_id);