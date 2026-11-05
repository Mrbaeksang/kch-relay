// Package testkit creates isolated, migrated SQLite fixtures for integration tests.
package testkit

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
)

type Fixture struct {
	DB  *sql.DB
	Now time.Time
	t   testing.TB
}

func New(t testing.TB) *Fixture {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.EnsureDevelopmentUsers(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return &Fixture{DB: db, Now: time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC), t: t}
}
func (f *Fixture) Product(seller, status string, price, quantity int64) string {
	f.t.Helper()
	id := httpx.NewID()
	ts := database.Timestamp(f.Now)
	_, err := f.DB.Exec(`INSERT INTO products(id,seller_id,title,description,unit_price,remaining_quantity,approval_status,created_at,updated_at) VALUES(?,?,'가상 상품','테스트 설명',?,?,?,?,?)`, id, seller, price, quantity, status, ts, ts)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}
func (f *Fixture) Participant(product, buyer, kind string) database.Participant {
	f.t.Helper()
	var p database.Participant
	err := database.WithTx(context.Background(), f.DB, func(tx *sql.Tx) error {
		var err error
		p, err = database.EnsureParticipantTx(context.Background(), tx, product, buyer, kind, f.Now)
		return err
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}
func (f *Fixture) Deal(participant, status string, quantity, price int64) string {
	f.t.Helper()
	id := httpx.NewID()
	ts := database.Timestamp(f.Now)
	var agreedPrice, agreedQuantity, accepted, completed, rejected, withdrawn any
	switch status {
	case "ACCEPTED", "COMPLETED":
		agreedPrice = price
		agreedQuantity = quantity
		accepted = ts
		if status == "COMPLETED" {
			completed = ts
		}
	case "REJECTED":
		rejected = ts
	case "WITHDRAWN":
		withdrawn = ts
	}
	_, err := f.DB.Exec(`INSERT INTO deals(id,participant_id,status,requested_quantity,agreed_unit_price,agreed_quantity,requested_at,accepted_at,completed_at,rejected_at,withdrawn_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, participant, status, quantity, agreedPrice, agreedQuantity, ts, accepted, completed, rejected, withdrawn, ts, ts)
	if err != nil {
		f.t.Fatal(err)
	}
	if accepted != nil {
		if _, err := f.DB.Exec("UPDATE participants SET identity_revealed_at=? WHERE id=?", ts, participant); err != nil {
			f.t.Fatal(err)
		}
	}
	return id
}
func (f *Fixture) Message(participant, sender, recipient string, read bool) string {
	f.t.Helper()
	id := httpx.NewID()
	var readAt any
	if read {
		readAt = database.Timestamp(f.Now)
	}
	_, err := f.DB.Exec(`INSERT INTO messages(id,participant_id,sender_id,recipient_id,body,read_at,created_at) VALUES(?,?,?,?,'가상 쪽지',?,?)`, id, participant, sender, recipient, readAt, database.Timestamp(f.Now))
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}
func (f *Fixture) Notification(deal, recipient string) string {
	f.t.Helper()
	var id string
	err := database.WithTx(context.Background(), f.DB, func(tx *sql.Tx) error {
		var err error
		id, err = database.AppendRejectionNotificationTx(context.Background(), tx, deal, recipient, f.Now)
		return err
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}
