package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

type Actor struct {
	ID         string `json:"id"`
	RealName   string `json:"realName"`
	Department string `json:"department"`
	JobLevel   string `json:"jobLevel"`
	Role       string `json:"role"`
	Status     string `json:"status"`
}
type actorKey struct{}
type authErrorKey struct{}

func WithAuthError(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, authErrorKey{}, err)
}

type requestIDKey struct{}

func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}
func CurrentUser(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	return a, ok
}
func RequestID(ctx context.Context) string { id, _ := ctx.Value(requestIDKey{}).(string); return id }
func NewID() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("secure randomness unavailable")
	}
	return hex.EncodeToString(b[:])
}

type Meta struct {
	HasUnreadMessages      bool `json:"hasUnreadMessages"`
	HasUnreadNotifications bool `json:"hasUnreadNotifications"`
}
type MetaProvider func(context.Context, string) (Meta, error)
type Result struct {
	Status  int
	Data    any
	Cookies []*http.Cookie
}
type Handler func(*http.Request) (Result, error)
type AppError struct {
	Status        int
	Code, Message string
	Details       any
	Cause         error
}

func (e *AppError) Error() string { return e.Code }
func (e *AppError) Unwrap() error { return e.Cause }
func Error(status int, code, message string) error {
	return &AppError{Status: status, Code: code, Message: message}
}
func Invalid(message string) error { return Error(400, "VALIDATION_ERROR", message) }

func Adapt(next Handler, meta MetaProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := next(r)
		var flags *Meta
		if a, ok := CurrentUser(r.Context()); ok && err == nil {
			m := Meta{}
			if meta != nil {
				m, err = meta(r.Context(), a.ID)
			}
			if err == nil {
				flags = &m
			}
		}
		if err == nil && r.Context().Err() != nil {
			err = r.Context().Err()
		}
		if err != nil {
			WriteError(w, r, err)
			return
		}
		status := result.Status
		if status == 0 {
			status = 200
		}
		body, err := json.Marshal(struct {
			Data any   `json:"data"`
			Meta *Meta `json:"meta,omitempty"`
		}{result.Data, flags})
		if err != nil {
			WriteError(w, r, err)
			return
		}
		headers(w)
		for _, cookie := range result.Cookies {
			http.SetCookie(w, cookie)
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}
}
func headers(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
}
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	e := &AppError{Status: 500, Code: "INTERNAL_ERROR", Message: "처리 중 오류가 발생했습니다. 다시 시도해 주세요."}
	var known *AppError
	if errors.As(err, &known) {
		e = known
	}
	if errors.Is(err, context.DeadlineExceeded) {
		e = &AppError{Status: 504, Code: "REQUEST_TIMEOUT", Message: "처리 시간이 초과되었습니다."}
	}
	id := RequestID(r.Context())
	if id == "" {
		id = NewID()
		w.Header().Set("X-Request-ID", id)
	}
	body, _ := json.Marshal(map[string]any{"error": struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Details   any    `json:"details,omitempty"`
		RequestID string `json:"requestId"`
	}{e.Code, e.Message, e.Details, id}})
	headers(w)
	w.WriteHeader(e.Status)
	_, _ = w.Write(body)
}
func RequireUser(next Handler) Handler {
	return func(r *http.Request) (Result, error) {
		a, ok := CurrentUser(r.Context())
		if !ok {
			if failure, exists := r.Context().Value(authErrorKey{}).(error); exists {
				return Result{}, failure
			}
			return Result{}, Error(401, "UNAUTHENTICATED", "로그인이 필요합니다.")
		}
		if a.Status != "ACTIVE" {
			return Result{}, Error(403, "ACCOUNT_SUSPENDED", "이용이 제한된 계정입니다.")
		}
		return next(r)
	}
}
func RequireAdmin(next Handler) Handler {
	return RequireUser(func(r *http.Request) (Result, error) {
		a, _ := CurrentUser(r.Context())
		if a.Role != "HR_ADMIN" {
			return Result{}, Error(403, "FORBIDDEN", "관리자만 이용할 수 있습니다.")
		}
		return next(r)
	})
}

const MaxJSONBytes int64 = 1 << 20

func DecodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return Invalid("Content-Type은 application/json이어야 합니다.")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, MaxJSONBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return Invalid("요청 내용을 확인해 주세요.")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Invalid("하나의 JSON 요청만 허용합니다.")
	}
	return nil
}
