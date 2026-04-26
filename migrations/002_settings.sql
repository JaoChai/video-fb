CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO settings (key, value) VALUES
    ('openrouter_api_key', ''),
    ('default_model', 'openai/gpt-4.1'),
    ('kie_api_key', ''),
    ('elevenlabs_voice', 'Adam'),
    ('zernio_api_key', '')
ON CONFLICT (key) DO NOTHING;
