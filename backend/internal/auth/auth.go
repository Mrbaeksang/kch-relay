package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/config"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

const CookieName = "relay_session"

type sessionKey struct{}
type session struct{ ID, CSRF string }
type Service struct {
	db     *sql.DB
	config config.Config
	now    func() time.Time
	meta   httpx.MetaProvider
}

func New(db *sql.DB, c config.Config, now func() time.Time) (*Service, error) {
	if c.AuthMode == "development" && c.AppEnv != "development" {
		return nil, errors.New("development authentication is forbidden outside development")
	}
	if c.AuthMode != "" && c.AuthMode != "disabled" && c.AuthMode != "development" {
		return nil, errors.New("authentication mode is not implemented")
	}
	if c.AuthMode == "" {
		c.AuthMode = "disabled"
	}
	if c.SessionTTL <= 0 {
		c.SessionTTL = 8 * time.Hour
	}
	if now == nil {
		now = time.Now
	}
	s := &Service{db: db, config: c, now: now}
	s.meta = func(ctx context.Context, user string) (httpx.Meta, error) { return database.UnreadMeta(ctx, db, user) }
	return s, nil
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.AuthMode != "development" {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(CookieName)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		var a httpx.Actor
		var current session
		var expires string
		var revoked sql.NullString
		if len(cookie.Value) != 64 {
			err = sql.ErrNoRows
		} else {
			err = s.db.QueryRowContext(r.Context(), `SELECT u.id,u.real_name,u.department,u.job_level,u.role,u.status,s.id,s.csrf_token,s.expires_at,s.revoked_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=?`, tokenHash(cookie.Value)).Scan(&a.ID, &a.RealName, &a.Department, &a.JobLevel, &a.Role, &a.Status, &current.ID, &current.CSRF, &expires, &revoked)
		}
		var failure error
		switch {
		case errors.Is(err, sql.ErrNoRows):
			failure = httpx.Error(401, "UNAUTHENTICATED", "로그인이 필요합니다.")
		case err != nil:
			httpx.WriteError(w, r, err)
			return
		default:
			deadline, parseErr := time.Parse(time.RFC3339Nano, expires)
			if parseErr != nil {
				httpx.WriteError(w, r, parseErr)
				return
			}
			if revoked.Valid || !s.now().Before(deadline) {
				failure = httpx.Error(401, "SESSION_EXPIRED", "로그인이 만료되었습니다.")
			} else if a.Status != "ACTIVE" {
				failure = httpx.Error(403, "ACCOUNT_SUSPENDED", "이용이 제한된 계정입니다.")
			}
		}
		if failure != nil {
			next.ServeHTTP(w, r.WithContext(httpx.WithAuthError(r.Context(), failure)))
			return
		}
		ctx := context.WithValue(httpx.WithActor(r.Context(), a), sessionKey{}, current)
		r = r.WithContext(ctx)
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && r.URL.Path != "/api/v1/auth/development-login" {
			if err := s.checkOrigin(r); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			if subtle.ConstantTimeCompare([]byte(current.CSRF), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
				httpx.WriteError(w, r, httpx.Error(403, "FORBIDDEN", "요청을 확인할 수 없습니다. 화면을 새로고침해 주세요."))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Service) checkOrigin(r *http.Request) error {
	if s.config.AppOrigin == "" || r.Header.Get("Origin") != s.config.AppOrigin {
		return httpx.Error(403, "FORBIDDEN", "허용되지 않은 요청 출처입니다.")
	}
	return nil
}
func (s *Service) Register(r chi.Router) {
	r.Get("/auth/config", httpx.Adapt(func(r *http.Request) (httpx.Result, error) {
		accounts := []database.DevelopmentAccount{}
		if s.config.AuthMode == "development" {
			accounts = database.DevelopmentAccounts()
		}
		return httpx.Result{Data: map[string]any{"authMode": s.config.AuthMode, "developmentLogin": s.config.AuthMode == "development", "accounts": accounts}}, nil
	}, s.meta))
	if s.config.AuthMode == "development" {
		r.Post("/auth/development-login", httpx.Adapt(s.login, s.meta))
	}
	r.Get("/auth/me", httpx.Adapt(httpx.RequireUser(s.me), s.meta))
	r.Post("/auth/logout", httpx.Adapt(httpx.RequireUser(s.logout), s.meta))
	r.Get("/access", httpx.Adapt(httpx.RequireUser(s.access), s.meta))
	r.Get("/admin/access", httpx.Adapt(httpx.RequireAdmin(s.access), s.meta))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, httpx.Error(404, "NOT_FOUND", "아직 제공하지 않는 API입니다."))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, httpx.Error(405, "METHOD_NOT_ALLOWED", "허용하지 않는 요청 방식입니다."))
	})
}
func (s *Service) access(r *http.Request) (httpx.Result, error) {
	a, _ := httpx.CurrentUser(r.Context())
	return httpx.Result{Data: map[string]any{"allowed": true, "role": a.Role}}, nil
}
func (s *Service) me(r *http.Request) (httpx.Result, error) {
	a, _ := httpx.CurrentUser(r.Context())
	current, _ := r.Context().Value(sessionKey{}).(session)
	return httpx.Result{Data: map[string]any{"user": a, "csrfToken": current.CSRF}}, nil
}
func (s *Service) login(r *http.Request) (httpx.Result, error) {
	if err := s.checkOrigin(r); err != nil {
		return httpx.Result{}, err
	}
	var input struct {
		AccountKey string `json:"accountKey"`
	}
	if err := httpx.DecodeJSON(r, &input); err != nil {
		return httpx.Result{}, err
	}
	allowed := false
	for _, a := range database.DevelopmentAccounts() {
		if a.Key == input.AccountKey {
			allowed = true
		}
	}
	if !allowed {
		return httpx.Result{}, httpx.Invalid("테스트 계정을 선택해 주세요.")
	}
	var a httpx.Actor
	var raw, csrf string
	now := s.now()
	err := database.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(r.Context(), `SELECT id,real_name,department,job_level,role,status FROM users WHERE id=?`, input.AccountKey).Scan(&a.ID, &a.RealName, &a.Department, &a.JobLevel, &a.Role, &a.Status); err != nil {
			return err
		}
		if a.Status != "ACTIVE" {
			return httpx.Error(403, "ACCOUNT_SUSPENDED", "이용이 제한된 계정입니다.")
		}
		if old, err := r.Cookie(CookieName); err == nil {
			if _, err := tx.ExecContext(r.Context(), "UPDATE sessions SET revoked_at=? WHERE token_hash=? AND revoked_at IS NULL", database.Timestamp(now), tokenHash(old.Value)); err != nil {
				return err
			}
		}
		raw = httpx.NewID()
		csrf = httpx.NewID()
		_, err := tx.ExecContext(r.Context(), `INSERT INTO sessions(id,user_id,token_hash,csrf_token,created_at,expires_at) VALUES(?,?,?,?,?,?)`, httpx.NewID(), a.ID, tokenHash(raw), csrf, database.Timestamp(now), database.Timestamp(now.Add(s.config.SessionTTL)))
		return err
	})
	if err != nil {
		return httpx.Result{}, err
	}
	*r = *r.WithContext(httpx.WithActor(r.Context(), a))
	return httpx.Result{Data: map[string]any{"user": a, "csrfToken": csrf}, Cookies: []*http.Cookie{s.cookie(raw, int(s.config.SessionTTL.Seconds()), now.Add(s.config.SessionTTL))}}, nil
}
func (s *Service) cookie(value string, maxAge int, expires time.Time) *http.Cookie {
	return &http.Cookie{Name: CookieName, Value: value, Path: "/api", HttpOnly: true, Secure: s.config.AppEnv == "production", SameSite: http.SameSiteLaxMode, MaxAge: maxAge, Expires: expires}
}
func (s *Service) logout(r *http.Request) (httpx.Result, error) {
	current, _ := r.Context().Value(sessionKey{}).(session)
	_, err := s.db.ExecContext(r.Context(), "UPDATE sessions SET revoked_at=? WHERE id=? AND revoked_at IS NULL", database.Timestamp(s.now()), current.ID)
	if err != nil {
		return httpx.Result{}, err
	}
	return httpx.Result{Data: map[string]bool{"loggedOut": true}, Cookies: []*http.Cookie{s.cookie("", -1, time.Unix(1, 0))}}, nil
}
