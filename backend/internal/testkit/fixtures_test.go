package testkit

import (
	"context"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"testing"
)

func TestSharedFixturesAndUnreadIsolation(t *testing.T) {
	f := New(t)
	product := f.Product("dev-admin", "APPROVED", 10000, 5)
	inquiry := f.Participant(product, "dev-employee-a", "inquiry")
	applicant := f.Participant(product, "dev-employee-a", "application")
	if inquiry.ID != applicant.ID || applicant.ApplicantNumber.Int64 != 1 {
		t.Fatal("inquiry and application lost their shared identity")
	}
	if again := f.Participant(product, "dev-employee-a", "application"); again.ID != applicant.ID {
		t.Fatal("duplicate participant")
	}
	deal := f.Deal(applicant.ID, "REJECTED", 2, 0)
	id := f.Notification(deal, "dev-employee-a")
	if id != f.Notification(deal, "dev-employee-a") {
		t.Fatal("duplicate rejection notification")
	}
	f.Message(applicant.ID, "dev-admin", "dev-employee-a", false)
	flags, err := database.UnreadMeta(context.Background(), f.DB, "dev-employee-a")
	if err != nil || !flags.HasUnreadMessages || !flags.HasUnreadNotifications {
		t.Fatalf("flags=%+v err=%v", flags, err)
	}
	flags, err = database.UnreadMeta(context.Background(), f.DB, "dev-employee-b")
	if err != nil || flags.HasUnreadMessages || flags.HasUnreadNotifications {
		t.Fatal("unread state leaked to another user")
	}
	accepted := f.Deal(applicant.ID, "ACCEPTED", 1, 10000)
	if _, err := f.DB.Exec("UPDATE deals SET agreed_unit_price=20000 WHERE id=?", accepted); err == nil {
		t.Fatal("agreed price changed after acceptance")
	}
	if _, err := f.DB.Exec("UPDATE products SET remaining_quantity=-1 WHERE id=?", product); err == nil {
		t.Fatal("negative stock accepted")
	}
}
