package auth

import (
	"context"
	"github.com/Mrbaeksang/kch-relay/backend/internal/testkit"
	"testing"
)

func TestBootstrapOnlyExactPendingHRWithoutExistingAdmin(t *testing.T) {
	f := testkit.New(t)
	ctx := context.Background()
	if err := BootstrapAdmin(ctx, f.DB, "missing@example.org"); err == nil {
		t.Fatal("bootstrap with existing admin accepted")
	}
	f.DB.Exec(`UPDATE users SET role='MEMBER'`)
	f.DB.Exec(`INSERT INTO users(id,google_subject,email,real_name,department,department_id,job_level,membership_status) VALUES('first','google-first','first@example.org','가상 관리자','인사팀','hr','사원','PROFILE_REQUIRED')`)
	if err := BootstrapAdmin(ctx, f.DB, "first@example.org"); err == nil {
		t.Fatal("unsubmitted application accepted")
	}
	f.DB.Exec(`UPDATE users SET membership_status='PENDING' WHERE id='first'`)
	if err := BootstrapAdmin(ctx, f.DB, "other@example.org"); err == nil {
		t.Fatal("wrong email accepted")
	}
	if err := BootstrapAdmin(ctx, f.DB, "first@example.org"); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapAdmin(ctx, f.DB, "first@example.org"); err == nil {
		t.Fatal("repeat bootstrap accepted")
	}
}
