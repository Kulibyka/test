CREATE TABLE IF NOT EXISTS mails (
    id UUID PRIMARY KEY,
    input TEXT NOT NULL,
    from_email TEXT NOT NULL,
    to_email TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    classification TEXT,
    model_answer JSONB,
    assistant_response JSONB,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    is_approved BOOLEAN NOT NULL DEFAULT FALSE,
    failed_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mails_processed ON mails (processed);
CREATE INDEX IF NOT EXISTS idx_mails_status ON mails (status);
CREATE INDEX IF NOT EXISTS idx_mails_received_at ON mails (received_at);
