package storage

import (
	"path/filepath"
	"testing"
)

func TestWriteAndReadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := map[string]int{"value": 42}
	if err := WriteJSON(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	var got map[string]int
	if err := ReadJSON(path, &got); err != nil {
		t.Fatal(err)
	}
	if got["value"] != 42 {
		t.Fatalf("value = %d, want 42", got["value"])
	}
}
