package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/code-sleuth/ike-go/pkg/db"
)

// SetupTestDB creates a test database connection and runs migrations.
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// Load environment variables from .env file
	err := LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Fatalf("Failed to load .env file: %v", err)
	}

	// Check if database environment variables are set
	dbURL := os.Getenv("TURSO_DATABASE_URL")
	authToken := os.Getenv("TURSO_AUTH_TOKEN")

	if dbURL == "" || authToken == "" {
		t.Skip("Database environment variables not set - skipping integration test")
	}

	// Create database connection
	database, err := db.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Ensure database is clean for testing
	cleanupTestData(t, database)

	return database
}

// CleanupTestDB performs cleanup after tests.
func CleanupTestDB(t *testing.T, database *sql.DB) {
	t.Helper()
	if database == nil {
		return
	}

	cleanupTestData(t, database)
	database.Close()
}

// cleanupTestData removes all test data from database tables.
func cleanupTestData(t *testing.T, database *sql.DB) {
	t.Helper()
	// Clean up in reverse order of dependencies
	tables := []string{
		"embeddings",
		"document_meta",
		"document_tags",
		"tags",
		"chunks",
		"documents",
		"downloads",
		"sources",
		"requests",
	}

	for _, table := range tables {
		// Use fmt.Sprintf to avoid string concatenation security warning
		query := fmt.Sprintf("DELETE FROM %s", table) // #nosec G201 -- table names are hardcoded, not user input
		_, err := database.Exec(query)
		if err != nil {
			t.Logf("Warning: Failed to clean table %s: %v", table, err)
		}
	}
}

// LoadEnvFromFile loads environment variables from a file.
func LoadEnvFromFile(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read the file content and parse environment variables
	const maxFileSize = 1024
	content := make([]byte, maxFileSize)
	n, err := file.Read(content)
	if err != nil && n == 0 {
		return err
	}

	// Simple parsing of export statements
	lines := strings.Split(string(content[:n]), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "export ") {
			// Remove "export " prefix
			line = strings.TrimPrefix(line, "export ")

			// Split on first "=" to get key and value
			const expectedParts = 2
			parts := strings.SplitN(line, "=", expectedParts)
			if len(parts) == expectedParts {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				// Remove quotes if present
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					value = value[1 : len(value)-1]
				}

				os.Setenv(key, value)
			}
		}
	}

	return nil
}

// Helper function to check if a record exists in a table.
func RecordExists(t *testing.T, db *sql.DB, table, idColumn, id string) bool {
	t.Helper()
	// #nosec G201 -- table and column names are hardcoded, not user input
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, idColumn)
	var count int
	err := db.QueryRow(query, id).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check if record exists: %v", err)
	}
	return count > 0
}

// Helper function to get record count from a table.
func GetRecordCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table) // #nosec G201 -- table name is hardcoded, not user input
	var count int
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to get record count: %v", err)
	}
	return count
}
