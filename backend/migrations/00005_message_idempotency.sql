-- +goose Up
-- Retried sends reuse the first message instead of creating a duplicate (#19).
-- The key lives as long as its message, so it follows the message retention policy.
ALTER TABLE messages ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX messages_idempotency ON messages(participant_id,sender_id,idempotency_key) WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP INDEX messages_idempotency;
ALTER TABLE messages DROP COLUMN idempotency_key;
