package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sachncs/promptsheon/internal/api/handlers"
)

func TestJSON_SetsContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := handlers.JSON(rec, http.StatusOK, map[string]string{"k": "v"}); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestJSON_WritesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := handlers.JSON(rec, http.StatusTeapot, nil); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("Code = %d, want 418", rec.Code)
	}
}

func TestJSON_EncodesValue(t *testing.T) {
	rec := httptest.NewRecorder()
	in := struct {
		Name string `json:"name"`
	}{Name: "x"}
	if err := handlers.JSON(rec, http.StatusOK, in); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var out struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Name != "x" {
		t.Errorf("Name = %q, want x", out.Name)
	}
}

func TestFunc_Type(t *testing.T) {
	// Func is an alias the rest of the package uses; assert it
	// can be assigned and called without panicking.
	var f handlers.Func = func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	rec := httptest.NewRecorder()
	if err := f(rec, httptest.NewRequest(http.MethodGet, "/", nil)); err != nil {
		t.Fatalf("Func: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("Code = %d, want 204", rec.Code)
	}
}
