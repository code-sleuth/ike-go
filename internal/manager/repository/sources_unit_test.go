package repository

import (
	"testing"

	"github.com/code-sleuth/ike-go/pkg/db"
)

// Test NewSourceRepository constructor
func TestNewSourceRepository_Unit(t *testing.T) {
	dbWrapper := &db.DB{}
	repo := NewSourceRepository(dbWrapper)

	if repo == nil {
		t.Fatal("Expected non-nil repository")
	}
	if repo.db != dbWrapper {
		t.Error("Expected database to be set correctly")
	}
}

// Test error constants
func TestSourceRepository_ErrorConstants(t *testing.T) {
	if errSourceNotFound == nil {
		t.Error("Expected errSourceNotFound to be defined")
	}
	if errSourceNotFound.Error() != "source not found" {
		t.Errorf("Expected 'source not found', got '%s'", errSourceNotFound.Error())
	}
}


// Helper function for tests
func stringPtr(s string) *string {
	return &s
}
