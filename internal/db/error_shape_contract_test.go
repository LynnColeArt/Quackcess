package db

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorShapeIsStable(t *testing.T) {
	seed := errors.New("boom")
	err := normalizeError("bootstrap", seed)
	dbErr, ok := err.(*DBError)
	if !ok {
		t.Fatalf("expected *DBError, got %T", err)
	}
	if dbErr.Op != "bootstrap" {
		t.Fatalf("operation = %q, want %q", dbErr.Op, "bootstrap")
	}
	if !strings.Contains(dbErr.Error(), "db bootstrap") {
		t.Fatalf("error string = %q", dbErr.Error())
	}
	if !errors.Is(err, seed) {
		t.Fatalf("expected wrapped root error")
	}
}
