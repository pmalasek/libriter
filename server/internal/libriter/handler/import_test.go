package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"mediater/internal/libriter/importer"
	"mediater/internal/libriter/scanner"
)

func TestImportCreateTooManySessions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := t.TempDir()
	scn := scanner.New(filepath.Join(base, "books"), "", nil)
	svc, err := importer.New(ctx, filepath.Join(base, "import"), 1<<20, 1, scn, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := NewImport(svc, nil)

	create := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.Create(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/import", nil))
		return rec
	}

	if rec := create(); rec.Code != http.StatusCreated {
		t.Fatalf("první import: status %d, %s", rec.Code, rec.Body)
	}
	rec := create()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("druhý import: status %d, chci 429", rec.Code)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "import.too_many_sessions" {
		t.Errorf("kód %q, chci import.too_many_sessions", body.Code)
	}
}
