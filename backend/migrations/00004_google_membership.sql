-- +goose Up
CREATE TABLE departments (
 id TEXT PRIMARY KEY NOT NULL,
 name TEXT NOT NULL UNIQUE CHECK(length(trim(name)) BETWEEN 1 AND 50),
 is_hr INTEGER NOT NULL DEFAULT 0 CHECK(is_hr IN (0,1))
);
CREATE UNIQUE INDEX one_hr_department ON departments(is_hr) WHERE is_hr=1;
INSERT INTO departments(id,name,is_hr) VALUES('hr','인사팀',1);
ALTER TABLE users ADD COLUMN department_id TEXT REFERENCES departments(id);
ALTER TABLE users ADD COLUMN membership_status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK(membership_status IN ('PROFILE_REQUIRED','PENDING','ACTIVE','REJECTED'));
ALTER TABLE users ADD COLUMN reviewed_at TEXT;
ALTER TABLE users ADD COLUMN reviewed_by TEXT REFERENCES users(id);
CREATE TABLE oauth_flows (
 state_hash TEXT PRIMARY KEY NOT NULL,
 binding_hash TEXT NOT NULL,
 nonce TEXT NOT NULL,
 verifier TEXT NOT NULL,
 expires_at TEXT NOT NULL
);
-- +goose Down
DROP TABLE oauth_flows;
ALTER TABLE users DROP COLUMN reviewed_by;
ALTER TABLE users DROP COLUMN reviewed_at;
ALTER TABLE users DROP COLUMN membership_status;
ALTER TABLE users DROP COLUMN department_id;
DROP TABLE departments;
