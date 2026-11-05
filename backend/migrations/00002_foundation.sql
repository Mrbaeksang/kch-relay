-- +goose Up
-- Forward-only foundation migration. Restore a consistent backup to roll back.
ALTER TABLE users RENAME TO users_before_foundation;
CREATE TABLE users (
 id TEXT PRIMARY KEY NOT NULL,
 google_subject TEXT UNIQUE,
 email TEXT NOT NULL UNIQUE,
 real_name TEXT NOT NULL,
 department TEXT NOT NULL,
 job_level TEXT NOT NULL,
 role TEXT NOT NULL DEFAULT 'MEMBER' CHECK(role IN ('MEMBER','HR_ADMIN')),
 status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK(status IN ('ACTIVE','SUSPENDED')),
 created_at TEXT NOT NULL DEFAULT(strftime('%Y-%m-%dT%H:%M:%fZ','now')),
 updated_at TEXT NOT NULL DEFAULT(strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
INSERT INTO users SELECT * FROM users_before_foundation;
DROP TABLE users_before_foundation;
CREATE TABLE sessions (
 id TEXT PRIMARY KEY NOT NULL,
 user_id TEXT NOT NULL REFERENCES users(id),
 token_hash TEXT NOT NULL UNIQUE CHECK(length(token_hash)=64),
 csrf_token TEXT NOT NULL,
 created_at TEXT NOT NULL,
 expires_at TEXT NOT NULL,
 revoked_at TEXT
);
CREATE INDEX sessions_user ON sessions(user_id);
CREATE TABLE products (
 id TEXT PRIMARY KEY NOT NULL,
 seller_id TEXT NOT NULL REFERENCES users(id),
 title TEXT NOT NULL CHECK(length(trim(title))>0),
 description TEXT NOT NULL CHECK(length(trim(description))>0),
 unit_price INTEGER NOT NULL CHECK(typeof(unit_price)='integer' AND unit_price>=0),
 remaining_quantity INTEGER NOT NULL CHECK(typeof(remaining_quantity)='integer' AND remaining_quantity>=0),
 approval_status TEXT NOT NULL DEFAULT 'PENDING' CHECK(approval_status IN ('PENDING','APPROVED','REJECTED')),
 recruitment_closed INTEGER NOT NULL DEFAULT 0 CHECK(recruitment_closed IN (0,1)),
 next_applicant_number INTEGER NOT NULL DEFAULT 1 CHECK(next_applicant_number>0),
 next_inquirer_number INTEGER NOT NULL DEFAULT 1 CHECK(next_inquirer_number>0),
 deleted_at TEXT,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX products_public ON products(approval_status,deleted_at,created_at,id);
CREATE INDEX products_seller ON products(seller_id,created_at);
CREATE TABLE participants (
 id TEXT PRIMARY KEY NOT NULL,
 product_id TEXT NOT NULL REFERENCES products(id),
 buyer_id TEXT NOT NULL REFERENCES users(id),
 inquirer_number INTEGER CHECK(inquirer_number>0),
 applicant_number INTEGER CHECK(applicant_number>0),
 identity_revealed_at TEXT,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 UNIQUE(product_id,buyer_id),
 UNIQUE(product_id,inquirer_number),
 UNIQUE(product_id,applicant_number),
 CHECK(inquirer_number IS NOT NULL OR applicant_number IS NOT NULL)
);
CREATE TABLE deals (
 id TEXT PRIMARY KEY NOT NULL,
 participant_id TEXT NOT NULL REFERENCES participants(id),
 status TEXT NOT NULL CHECK(status IN ('REQUESTED','WITHDRAWN','REJECTED','ACCEPTED','COMPLETED')),
 requested_quantity INTEGER NOT NULL CHECK(typeof(requested_quantity)='integer' AND requested_quantity>0),
 agreed_unit_price INTEGER CHECK(agreed_unit_price IS NULL OR (typeof(agreed_unit_price)='integer' AND agreed_unit_price>=0)),
 agreed_quantity INTEGER CHECK(agreed_quantity IS NULL OR (typeof(agreed_quantity)='integer' AND agreed_quantity>0)),
 requested_at TEXT NOT NULL,
 quantity_updated_at TEXT,
 accepted_at TEXT,
 completed_at TEXT,
 rejected_at TEXT,
 withdrawn_at TEXT,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 CHECK((status IN ('ACCEPTED','COMPLETED') AND agreed_unit_price IS NOT NULL AND agreed_quantity IS NOT NULL AND accepted_at IS NOT NULL) OR (status NOT IN ('ACCEPTED','COMPLETED') AND agreed_unit_price IS NULL AND agreed_quantity IS NULL AND accepted_at IS NULL)),
 CHECK((status='COMPLETED' AND completed_at IS NOT NULL) OR (status<>'COMPLETED' AND completed_at IS NULL)),
 CHECK((status='REJECTED' AND rejected_at IS NOT NULL) OR (status<>'REJECTED' AND rejected_at IS NULL)),
 CHECK((status='WITHDRAWN' AND withdrawn_at IS NOT NULL) OR (status<>'WITHDRAWN' AND withdrawn_at IS NULL))
);
CREATE INDEX deals_participant ON deals(participant_id,status);
CREATE INDEX deals_completed ON deals(status,completed_at);
CREATE TABLE messages (
 id TEXT PRIMARY KEY NOT NULL,
 participant_id TEXT NOT NULL REFERENCES participants(id),
 sender_id TEXT NOT NULL REFERENCES users(id),
 recipient_id TEXT NOT NULL REFERENCES users(id),
 body TEXT NOT NULL CHECK(length(trim(body))>0),
 read_at TEXT,
 created_at TEXT NOT NULL,
 CHECK(sender_id<>recipient_id)
);
CREATE INDEX messages_conversation ON messages(participant_id,created_at,id);
CREATE INDEX messages_unread ON messages(recipient_id,read_at);
CREATE TABLE notifications (
 id TEXT PRIMARY KEY NOT NULL,
 recipient_id TEXT NOT NULL REFERENCES users(id),
 deal_id TEXT NOT NULL REFERENCES deals(id),
 type TEXT NOT NULL CHECK(type='APPLICATION_REJECTED'),
 read_at TEXT,
 created_at TEXT NOT NULL,
 UNIQUE(deal_id,type)
);
CREATE INDEX notifications_unread ON notifications(recipient_id,read_at,created_at);
CREATE TABLE images (
 id TEXT PRIMARY KEY NOT NULL,
 uploader_id TEXT NOT NULL REFERENCES users(id),
 product_id TEXT REFERENCES products(id),
 storage_key TEXT NOT NULL UNIQUE,
 sort_order INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL
);
CREATE TABLE reviews (
 id TEXT PRIMARY KEY NOT NULL,
 product_id TEXT NOT NULL REFERENCES products(id),
 reviewer_id TEXT NOT NULL REFERENCES users(id),
 decision TEXT NOT NULL CHECK(decision IN ('APPROVED','REJECTED')),
 reason TEXT,
 created_at TEXT NOT NULL,
 CHECK(decision<>'REJECTED' OR (reason IS NOT NULL AND length(trim(reason))>0))
);
CREATE INDEX reviews_product ON reviews(product_id,created_at);

-- +goose StatementBegin
CREATE TRIGGER deals_agreement_immutable BEFORE UPDATE OF agreed_unit_price,agreed_quantity ON deals
WHEN OLD.status IN ('ACCEPTED','COMPLETED') AND (NEW.agreed_unit_price IS NOT OLD.agreed_unit_price OR NEW.agreed_quantity IS NOT OLD.agreed_quantity)
BEGIN
 SELECT RAISE(ABORT,'agreed values are immutable');
END;
-- +goose StatementEnd
