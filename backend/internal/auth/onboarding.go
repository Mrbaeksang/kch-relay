package auth

import (
	"database/sql"
	"errors"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strings"
	"unicode/utf8"
)

var jobLevels = []string{"사원", "주임", "대리", "과장", "차장", "부장", "이사", "상무", "전무", "부사장", "사장", "회장"}

type department struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IsHR bool   `json:"isHR"`
}

func (s *Service) requireSession(next httpx.Handler) httpx.Handler {
	return func(r *http.Request) (httpx.Result, error) {
		if _, ok := httpx.CurrentUser(r.Context()); !ok {
			return httpx.RequireUser(next)(r)
		}
		return next(r)
	}
}
func (s *Service) registerOnboarding(r chi.Router) {
	r.Get("/auth/departments", httpx.Adapt(s.requireSession(s.departments), s.meta))
	r.Post("/auth/register", httpx.Adapt(s.requireSession(s.registerMember), s.meta))
	r.Get("/admin/members", httpx.Adapt(httpx.RequireAdmin(s.members), s.meta))
	r.Post("/admin/members/{id}/{action}", httpx.Adapt(httpx.RequireAdmin(s.reviewMember), s.meta))
	r.Get("/admin/departments", httpx.Adapt(httpx.RequireAdmin(s.departments), s.meta))
	r.Post("/admin/departments", httpx.Adapt(httpx.RequireAdmin(s.createDepartment), s.meta))
	r.Patch("/admin/departments/{id}", httpx.Adapt(httpx.RequireAdmin(s.renameDepartment), s.meta))
	r.Delete("/admin/departments/{id}", httpx.Adapt(httpx.RequireAdmin(s.deleteDepartment), s.meta))
}
func (s *Service) departments(r *http.Request) (httpx.Result, error) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,name,is_hr FROM departments ORDER BY is_hr DESC,name`)
	if err != nil {
		return httpx.Result{}, err
	}
	defer rows.Close()
	list := []department{}
	for rows.Next() {
		var d department
		if err := rows.Scan(&d.ID, &d.Name, &d.IsHR); err != nil {
			return httpx.Result{}, err
		}
		list = append(list, d)
	}
	return httpx.Result{Data: map[string]any{"departments": list, "jobLevels": jobLevels}}, rows.Err()
}
func (s *Service) registerMember(r *http.Request) (httpx.Result, error) {
	a, _ := httpx.CurrentUser(r.Context())
	var in struct {
		RealName     string `json:"realName"`
		DepartmentID string `json:"departmentId"`
		JobLevel     string `json:"jobLevel"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return httpx.Result{}, err
	}
	in.RealName = strings.TrimSpace(in.RealName)
	validLevel := false
	for _, v := range jobLevels {
		if v == in.JobLevel {
			validLevel = true
		}
	}
	if utf8.RuneCountInString(in.RealName) < 1 || utf8.RuneCountInString(in.RealName) > 50 || !validLevel {
		return httpx.Result{}, httpx.Invalid("이름(1~50자)과 직급을 확인해 주세요.")
	}
	err := database.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var name string
		if err := tx.QueryRowContext(r.Context(), `SELECT name FROM departments WHERE id=?`, in.DepartmentID).Scan(&name); errors.Is(err, sql.ErrNoRows) {
			return httpx.Invalid("선택한 부서가 없습니다. 목록을 다시 불러와 주세요.")
		} else if err != nil {
			return err
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE users SET real_name=?,department=?,department_id=?,job_level=?,membership_status='PENDING',role='MEMBER',updated_at=? WHERE id=? AND membership_status='PROFILE_REQUIRED' AND status='ACTIVE'`, in.RealName, name, in.DepartmentID, in.JobLevel, database.Timestamp(s.now()), a.ID)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return httpx.Error(409, "MEMBERSHIP_CONFLICT", "이미 가입 신청을 제출했습니다.")
		}
		return nil
	})
	if err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Data: map[string]string{"status": "PENDING"}}, nil
}
func (s *Service) members(r *http.Request) (httpx.Result, error) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,email,real_name,department,job_level,role,CASE WHEN status='SUSPENDED' THEN status ELSE membership_status END,created_at FROM users WHERE membership_status!='PROFILE_REQUIRED' ORDER BY CASE membership_status WHEN 'PENDING' THEN 0 ELSE 1 END,created_at,id`)
	if err != nil {
		return httpx.Result{}, err
	}
	defer rows.Close()
	type member struct {
		httpx.Actor
		Email     string `json:"email"`
		CreatedAt string `json:"createdAt"`
	}
	list := []member{}
	for rows.Next() {
		var u member
		if err := rows.Scan(&u.ID, &u.Email, &u.RealName, &u.Department, &u.JobLevel, &u.Role, &u.Status, &u.CreatedAt); err != nil {
			return httpx.Result{}, err
		}
		list = append(list, u)
	}
	return httpx.Result{Data: map[string]any{"members": list}}, rows.Err()
}
func (s *Service) reviewMember(r *http.Request) (httpx.Result, error) {
	actor, _ := httpx.CurrentUser(r.Context())
	id, action := chi.URLParam(r, "id"), chi.URLParam(r, "action")
	if id == actor.ID {
		return httpx.Result{}, httpx.Error(409, "SELF_REVIEW", "본인 계정은 이 화면에서 변경할 수 없습니다.")
	}
	err := database.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var status, membership string
		var hr bool
		err := tx.QueryRowContext(r.Context(), `SELECT u.status,u.membership_status,coalesce(d.is_hr,0) FROM users u LEFT JOIN departments d ON d.id=u.department_id WHERE u.id=?`, id).Scan(&status, &membership, &hr)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.Error(404, "NOT_FOUND", "계정이 없습니다.")
		}
		if err != nil {
			return err
		}
		now := database.Timestamp(s.now())
		switch action {
		case "approve", "reject":
			if membership != "PENDING" || status != "ACTIVE" {
				return httpx.Error(409, "MEMBERSHIP_CONFLICT", "승인 대기 중인 계정만 처리할 수 있습니다.")
			}
			next, role := "REJECTED", "MEMBER"
			if action == "approve" {
				next = "ACTIVE"
				if hr {
					role = "HR_ADMIN"
				}
			}
			_, err = tx.ExecContext(r.Context(), `UPDATE users SET membership_status=?,role=?,reviewed_at=?,reviewed_by=?,updated_at=? WHERE id=?`, next, role, now, actor.ID, now, id)
		case "suspend", "restore":
			if membership != "ACTIVE" {
				return httpx.Error(409, "MEMBERSHIP_CONFLICT", "가입 승인된 계정만 정지·해제할 수 있습니다.")
			}
			next := "SUSPENDED"
			if action == "restore" {
				next = "ACTIVE"
			}
			if next == status {
				return httpx.Error(409, "MEMBERSHIP_CONFLICT", "이미 처리된 상태입니다.")
			}
			_, err = tx.ExecContext(r.Context(), `UPDATE users SET status=?,updated_at=? WHERE id=?`, next, now, id)
		default:
			return httpx.Error(404, "NOT_FOUND", "지원하지 않는 처리입니다.")
		}
		return err
	})
	return httpx.Result{Data: map[string]bool{"updated": err == nil}}, err
}
func departmentName(r *http.Request) (string, error) {
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return "", err
	}
	name := strings.TrimSpace(in.Name)
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 50 {
		return "", httpx.Invalid("부서 이름은 1~50자로 입력해 주세요.")
	}
	return name, nil
}
func departmentWriteError(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return httpx.Error(409, "DEPARTMENT_CONFLICT", "같은 이름의 부서가 있습니다.")
	}
	return err
}
func (s *Service) createDepartment(r *http.Request) (httpx.Result, error) {
	name, err := departmentName(r)
	if err != nil {
		return httpx.Result{}, err
	}
	d := department{ID: httpx.NewID(), Name: name}
	_, err = s.db.ExecContext(r.Context(), `INSERT INTO departments(id,name) VALUES(?,?)`, d.ID, name)
	return httpx.Result{Status: 201, Data: d}, departmentWriteError(err)
}
func (s *Service) renameDepartment(r *http.Request) (httpx.Result, error) {
	name, err := departmentName(r)
	if err != nil {
		return httpx.Result{}, err
	}
	id := chi.URLParam(r, "id")
	err = database.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(r.Context(), `UPDATE departments SET name=? WHERE id=?`, name, id)
		if err != nil {
			return departmentWriteError(err)
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return httpx.Error(404, "NOT_FOUND", "부서가 없습니다.")
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE users SET department=?,updated_at=? WHERE department_id=?`, name, database.Timestamp(s.now()), id)
		return err
	})
	return httpx.Result{Data: map[string]bool{"updated": err == nil}}, err
}
func (s *Service) deleteDepartment(r *http.Request) (httpx.Result, error) {
	id := chi.URLParam(r, "id")
	err := database.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var hr bool
		if err := tx.QueryRowContext(r.Context(), `SELECT is_hr FROM departments WHERE id=?`, id).Scan(&hr); errors.Is(err, sql.ErrNoRows) {
			return httpx.Error(404, "NOT_FOUND", "부서가 없습니다.")
		} else if err != nil {
			return err
		}
		var count int
		if err := tx.QueryRowContext(r.Context(), `SELECT count(*) FROM users WHERE department_id=?`, id).Scan(&count); err != nil {
			return err
		}
		if hr || count > 0 {
			return httpx.Error(409, "DEPARTMENT_IN_USE", "인사팀 또는 구성원이 선택한 부서는 삭제할 수 없습니다.")
		}
		_, err := tx.ExecContext(r.Context(), `DELETE FROM departments WHERE id=?`, id)
		return err
	})
	return httpx.Result{Data: map[string]bool{"deleted": err == nil}}, err
}
