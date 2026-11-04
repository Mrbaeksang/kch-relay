package httpapi

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func NewRouter(db *sql.DB, web fs.FS) http.Handler {
	router := chi.NewRouter()
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { reply(w, http.StatusOK, `{"status":"ok"}`) })
	router.Get("/api/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if db == nil || db.PingContext(ctx) != nil {
			reply(w, http.StatusServiceUnavailable, `{"status":"unavailable"}`)
			return
		}
		reply(w, http.StatusOK, `{"status":"ok","database":"connected"}`)
	})
	unknownAPI := func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusNotFound, `{"error":{"code":"NOT_FOUND","message":"아직 제공하지 않는 API입니다."}}`)
	}
	router.HandleFunc("/api", unknownAPI)
	router.HandleFunc("/api/*", unknownAPI)
	router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(web, name); err == nil {
			http.FileServer(http.FS(web)).ServeHTTP(w, r)
			return
		}
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		index, err := fs.ReadFile(web, "index.html")
		if err != nil {
			http.Error(w, "화면 파일을 찾을 수 없습니다. 프론트를 먼저 빌드해 주세요.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
	return router
}

func reply(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
