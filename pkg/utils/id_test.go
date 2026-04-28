package utils

import "testing"

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()
	if id1 == "" || id2 == "" || id1 == id2 {
		t.Fatalf("expected unique generated ids: %s %s", id1, id2)
	}
}
