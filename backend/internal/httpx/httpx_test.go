package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGlobalErrorsAndAccessControl(t *testing.T) {
	handler := Stack(nil, 0)(Adapt(RequireAdmin(func(r *http.Request) (Result, error) { return Result{Data: "allowed"}, nil }), nil))
	for _, tc := range []struct {
		role   string
		status int
	}{{"", 401}, {"MEMBER", 403}, {"HR_ADMIN", 200}} {
		r := httptest.NewRequest("GET", "/private", nil)
		if tc.role != "" {
			r = r.WithContext(WithActor(r.Context(), Actor{ID: "test", Role: tc.role, Status: "ACTIVE"}))
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("role=%s status=%d body=%s", tc.role, w.Code, w.Body.String())
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Fatal("missing request ID")
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
	}
	bad := Stack(nil, 0)(Adapt(func(r *http.Request) (Result, error) {
		var in struct {
			Name string `json:"name"`
		}
		return Result{}, DecodeJSON(r, &in)
	}, nil))
	w := httptest.NewRecorder()
	bad.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"unexpected":true}`)))
	if w.Code != 400 {
		t.Fatalf("unknown input accepted: %d", w.Code)
	}
}

func TestJSONRequiresJSONContentType(t *testing.T) {
	h := Adapt(func(r *http.Request) (Result, error) {
		var in struct {
			Name string `json:"name"`
		}
		return Result{Data: in}, DecodeJSON(r, &in)
	}, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"value"}`))
	r.Header.Set("Content-Type", "text/plain")
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("non-JSON content type accepted: %d", w.Code)
	}
}

func TestFailuresDoNotLeakDetailsOrDoubleWrite(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	for _, mode := range []string{"error", "panic", "timeout", "meta"} {
		t.Run(mode, func(t *testing.T) {
			committed := false
			h := Handler(func(r *http.Request) (Result, error) {
				switch mode {
				case "error":
					return Result{}, errors.New("SECRET SQL cookie=value")
				case "panic":
					panic("SECRET private data")
				case "timeout":
					<-r.Context().Done()
					return Result{}, r.Context().Err()
				default:
					committed = true
					return Result{Data: "done"}, nil
				}
			})
			meta := MetaProvider(func(context.Context, string) (Meta, error) { return Meta{}, errors.New("SECRET metadata") })
			r := httptest.NewRequest("GET", "/private?email=SECRET", nil)
			if mode == "meta" {
				r = r.WithContext(WithActor(r.Context(), Actor{ID: "person", Status: "ACTIVE"}))
			}
			w := httptest.NewRecorder()
			Stack(logger, time.Millisecond)(Adapt(h, meta)).ServeHTTP(w, r)
			if w.Code < 500 || strings.Contains(w.Body.String(), "SECRET") {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var result map[string]any
			if json.Unmarshal(w.Body.Bytes(), &result) != nil {
				t.Fatal("invalid/double error response")
			}
			if mode == "meta" && !committed {
				t.Fatal("metadata error cannot imply rollback")
			}
		})
	}
	if strings.Contains(logs.String(), "SECRET") {
		t.Fatal("private request/error details leaked into logs")
	}
}

func TestSizePaginationAndKSTRange(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"`+strings.Repeat("x", int(MaxJSONBytes))+`"}`))
	r.Header.Set("Content-Type", "application/json")
	var body struct {
		Name string `json:"name"`
	}
	if DecodeJSON(r, &body) == nil {
		t.Fatal("oversize accepted")
	}
	if _, err := ParsePage(url.Values{"limit": {"101"}}); err == nil {
		t.Fatal("limit accepted")
	}
	c := Cursor{CreatedAt: "2026-11-05T00:00:00.000Z", ID: "opaque"}
	p, err := ParsePage(url.Values{"cursor": {EncodeCursor(c)}})
	if err != nil || p.After.ID != "opaque" {
		t.Fatalf("page=%+v err=%v", p, err)
	}
	start, end, err := ParseDateRange("2026-11-05", "2026-11-05")
	if err != nil || start.Format(time.RFC3339) != "2026-11-04T15:00:00Z" || end.Sub(start) != 24*time.Hour {
		t.Fatalf("range=%s..%s err=%v", start, end, err)
	}
}
