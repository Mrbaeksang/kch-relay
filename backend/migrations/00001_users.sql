-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY NOT NULL,
    google_subject TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    real_name TEXT NOT NULL,
    department TEXT NOT NULL,
    job_level TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'MEMBER' CHECK (role IN ('MEMBER', 'HR_ADMIN')),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- +goose Down
DROP TABLE users;
