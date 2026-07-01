package model

import (
	"testing"
)

func TestModelStructure(t *testing.T) {
	m := Model{
		ID:   "test-id",
		Name: "Test Name",
		Type: "test-type",
	}

	if m.ID != "test-id" {
		t.Errorf("expected ID test-id, got %s", m.ID)
	}
	if m.Name != "Test Name" {
		t.Errorf("expected Name Test Name, got %s", m.Name)
	}
}
