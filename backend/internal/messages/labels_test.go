package messages

import (
	"database/sql"
	"testing"

	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
)

func TestInquirerLetters(t *testing.T) {
	for n, want := range map[int64]string{1: "A", 2: "B", 26: "Z", 27: "AA", 52: "AZ", 53: "BA", 702: "ZZ", 703: "AAA"} {
		if got := inquirerLetters(n); got != want {
			t.Errorf("%d: %q want %q", n, got, want)
		}
	}
}

func TestCounterpartyLabel(t *testing.T) {
	n := func(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }
	none := sql.NullInt64{}
	cases := []struct {
		seller   bool
		inq, app sql.NullInt64
		want     string
	}{
		{false, n(1), none, "판매자"},
		{true, n(1), none, "문의자 A"},
		{true, n(2), n(3), "익명 3"},
		{true, none, n(1), "익명 1"},
	}
	for _, c := range cases {
		if got := counterpartyLabel(c.seller, c.inq, c.app); got != c.want {
			t.Errorf("%+v: %q want %q", c, got, c.want)
		}
	}
}

func TestMigrationIdempotencyIndex(t *testing.T) {
	f := testkit.New(t)
	product := f.Product(seller, "APPROVED", 1000, 5)
	p := f.Participant(product, buyer, "inquiry")
	insert := func(id string, key any) error {
		_, err := f.DB.Exec(`INSERT INTO messages(id,participant_id,sender_id,recipient_id,body,created_at,idempotency_key) VALUES(?,?,?,?,'본문','2026-11-05T03:00:00.000Z',?)`, id, p.ID, buyer, seller, key)
		return err
	}
	if err := insert("m1", "k1"); err != nil {
		t.Fatal(err)
	}
	if err := insert("m2", "k1"); err == nil {
		t.Fatal("same participant, sender and key must be rejected")
	}
	if err := insert("m3", nil); err != nil {
		t.Fatal(err)
	}
	if err := insert("m4", nil); err != nil {
		t.Fatal("rows without a key must not collide:", err)
	}
}
