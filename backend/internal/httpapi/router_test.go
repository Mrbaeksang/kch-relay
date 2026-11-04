package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
)

func TestServiceReadinessAndWebRouting(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	web := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>KCH Relay</title>")}, "assets/app.js": {Data: []byte("console.log('ready')")}}
	server := httptest.NewServer(NewRouter(db, web))
	defer server.Close()
	for _, tc := range []struct {
		path    string
		status  int
		content string
	}{
		{"/healthz", 200, "ok"}, {"/api/readyz", 200, "connected"}, {"/", 200, "KCH Relay"},
		{"/products/123", 200, "KCH Relay"}, {"/assets/app.js", 200, "ready"},
		{"/assets/missing.js", 404, "404"}, {"/api/v1/products", 404, "NOT_FOUND"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			response, err := http.Get(server.URL + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)
			if response.StatusCode != tc.status || !strings.Contains(string(body), tc.content) {
				t.Fatalf("status=%d body=%s", response.StatusCode, body)
			}
		})
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(server.URL + "/api/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result map[string]string
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 503 || result["status"] != "unavailable" {
		t.Fatalf("status=%d body=%v", response.StatusCode, result)
	}
}
