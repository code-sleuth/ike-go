package transformers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/internal/manager/testutil"
)

func TestGitHubTransformer_Transform_DatabaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		download    *models.Download
		expectError bool
		description string
	}{
		{
			name: "transform GitHub markdown file",
			download: &models.Download{
				ID:       "download-md-123",
				SourceID: "source-md-123",
				Headers:  `{"Content-Type": ["text/plain"], "X-GitHub-SHA": ["abc123def456"]}`,
				Body: stringPtrGH(`# GitHub Integration Test

This is a **markdown** file from GitHub with various elements:

- List item 1
- List item 2 with [a link](https://example.com)

## Code Section

Here's some Go code:

` + "```go" + `
func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

> This is a blockquote

The end.`),
			},
			expectError: false,
			description: "should transform GitHub markdown file and create all records",
		},
		{
			name: "transform GitHub Go source file",
			download: &models.Download{
				ID:       "download-go-456",
				SourceID: "source-go-456",
				Headers:  `{"Content-Type": ["text/plain"], "X-GitHub-SHA": ["def456ghi789"]}`,
				Body: stringPtrGH(`package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello, World!")
	if len(os.Args) > 1 {
		fmt.Printf("Arguments: %v\n", os.Args[1:])
	}
}`),
			},
			expectError: false,
			description: "should transform GitHub Go source file",
		},
		{
			name: "transform GitHub JSON file",
			download: &models.Download{
				ID:       "download-json-789",
				SourceID: "source-json-789",
				Headers:  `{"Content-Type": ["application/json"], "X-GitHub-SHA": ["ghi789jkl012"]}`,
				Body: stringPtrGH(`{
  "name": "test-project",
  "version": "1.0.0",
  "description": "A test project for integration testing",
  "main": "index.js",
  "scripts": {
    "test": "jest",
    "start": "node index.js"
  },
  "dependencies": {
    "express": "^4.18.0"
  }
}`),
			},
			expectError: false,
			description: "should transform GitHub JSON file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Create a mock source first for the transformer to query
			sourceID := tt.download.SourceID

			// Determine the file path based on the test case
			var filePath string
			switch tt.name {
			case "transform GitHub markdown file":
				filePath = "README.md"
			case "transform GitHub Go source file":
				filePath = "main.go"
			case "transform GitHub JSON file":
				filePath = "package.json"
			}

			// Create the source URL
			sourceURL := "https://github.com/code-sleuth/outh/blob/main/" + filePath

			// Insert mock source record
			sourceQuery := `INSERT INTO sources (id, raw_url, scheme, host, path, active_domain, format, created_at, updated_at)
						   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
			now := time.Now().Format(time.RFC3339)
			_, err := db.Exec(sourceQuery, sourceID, sourceURL, "https", "github.com",
				"/code-sleuth/outh/blob/main/"+filePath, 1, "json", now, now)
			if err != nil {
				t.Fatalf("Failed to create mock source: %v", err)
			}

			// Insert mock download record
			downloadQuery := `INSERT INTO downloads (id, source_id, status_code, headers, body, downloaded_at)
							  VALUES (?, ?, ?, ?, ?, ?)`
			_, err = db.Exec(downloadQuery, tt.download.ID, sourceID, 200, tt.download.Headers,
				*tt.download.Body, now)
			if err != nil {
				t.Fatalf("Failed to create mock download: %v", err)
			}

			// Verify database is clean before test
			documentCount := testutil.GetRecordCount(t, db, "documents")

			if documentCount != 0 {
				t.Errorf("Expected 0 documents before test, got %d", documentCount)
			}

			// Transform the download
			result, err := transformer.Transform(ctx, tt.download, db)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if !tt.expectError {
				// Verify result
				if result == nil {
					t.Errorf("Expected non-nil result for test: %s", tt.description)
					return
				}
				if result.Document == nil {
					t.Errorf("Expected non-nil document in result for test: %s", tt.description)
					return
				}

				// Verify document was created in database
				documentID := result.Document.ID
				if !testutil.RecordExists(t, db, "documents", "id", documentID) {
					t.Errorf("Document record not found in database for test: %s", tt.description)
				}

				// Verify document data
				var sourceIDFromDB, downloadID, format string
				var minChunkSize, maxChunkSize int
				query := "SELECT source_id, download_id, format, min_chunk_size, max_chunk_size FROM documents WHERE id = ?"
				err := db.QueryRow(query, documentID).
					Scan(&sourceIDFromDB, &downloadID, &format, &minChunkSize, &maxChunkSize)
				if err != nil {
					t.Errorf("Failed to query document data: %v", err)
				} else {
					if sourceIDFromDB != tt.download.SourceID {
						t.Errorf("Expected source_id %s, got %s", tt.download.SourceID, sourceIDFromDB)
					}
					if downloadID != tt.download.ID {
						t.Errorf("Expected download_id %s, got %s", tt.download.ID, downloadID)
					}

					// Verify format based on file type
					expectedFormat := "json"
					switch tt.name {
					case "transform GitHub markdown file":
						expectedFormat = "json"
					case "transform GitHub JSON file":
						expectedFormat = "json"
					}

					if format != expectedFormat {
						t.Errorf("Expected format %s, got %s", expectedFormat, format)
					}
					if minChunkSize != 212 {
						t.Errorf("Expected min_chunk_size 212, got %d", minChunkSize)
					}
					if maxChunkSize != 8191 {
						t.Errorf("Expected max_chunk_size 8191, got %d", maxChunkSize)
					}
				}

				// Note: This transformer only creates documents and metadata, not chunks
				// Chunks would be created by a separate chunking service

				// Verify no chunks exist yet (since chunking is a separate step)
				chunkCount := testutil.GetRecordCount(t, db, "chunks")
				if chunkCount != 0 {
					t.Errorf("Expected 0 chunks (chunking is separate step), got %d", chunkCount)
				}

				// Verify database counts increased
				newDocumentCount := testutil.GetRecordCount(t, db, "documents")
				if newDocumentCount != documentCount+1 {
					t.Errorf("Expected document count to increase by 1, got %d -> %d", documentCount, newDocumentCount)
				}

				// Clean up for next test (order matters due to foreign keys)
				_, err = db.Exec("DELETE FROM document_meta WHERE document_id = ?", documentID)
				if err != nil {
					t.Errorf("Failed to clean up document_meta: %v", err)
				}
				_, err = db.Exec("DELETE FROM documents WHERE id = ?", documentID)
				if err != nil {
					t.Errorf("Failed to clean up document: %v", err)
				}
			}

			// Clean up download and source (order matters due to foreign keys)
			_, err = db.Exec("DELETE FROM downloads WHERE source_id = ?", sourceID)
			if err != nil {
				t.Errorf("Failed to clean up downloads: %v", err)
			}
			_, err = db.Exec("DELETE FROM sources WHERE id = ?", sourceID)
			if err != nil {
				t.Errorf("Failed to clean up source: %v", err)
			}
		})
	}
}

func TestGitHubTransformer_CanTransform_DatabaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		download    *models.Download
		expected    bool
		description string
	}{
		{
			name: "valid GitHub download with SHA header",
			download: &models.Download{
				Headers: `{"X-GitHub-SHA": ["abc123def456"], "Content-Type": ["text/plain"]}`,
				Body:    stringPtrGH("file content"),
			},
			expected:    true,
			description: "should detect valid GitHub download",
		},
		{
			name: "download without GitHub headers",
			download: &models.Download{
				Headers: `{"Content-Type": ["text/plain"]}`,
				Body:    stringPtrGH("file content"),
			},
			expected:    false,
			description: "should reject non-GitHub downloads",
		},
		{
			name: "nil body",
			download: &models.Download{
				Headers: `{"X-GitHub-SHA": ["abc123def456"]}`,
				Body:    nil,
			},
			expected:    false,
			description: "should reject nil body",
		},
		{
			name: "invalid headers JSON",
			download: &models.Download{
				Headers: `invalid json`,
				Body:    stringPtrGH("file content"),
			},
			expected:    false,
			description: "should reject invalid headers JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.CanTransform(tt.download)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for test: %s", tt.expected, result, tt.description)
			}
		})
	}
}

func TestGitHubTransformer_ProcessContent_DatabaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	transformer := NewGitHubTransformer()

	tests := []struct {
		name            string
		body            string
		filePath        string
		expectSubstring string
		description     string
	}{
		{
			name: "process complex Go file",
			body: `package main

import (
	"fmt"
	"net/http"
	"encoding/json"
)

type User struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func main() {
	http.HandleFunc("/users", handleUsers)
	http.ListenAndServe(":8080", nil)
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	users := []User{
		{ID: 1, Name: "John"},
		{ID: 2, Name: "Jane"},
	}
	json.NewEncoder(w).Encode(users)
}`,
			filePath:        "server.go",
			expectSubstring: "```go\n",
			description:     "should wrap complex Go code in code blocks",
		},
		{
			name: "process HTML file conversion",
			body: `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
</head>
<body>
	<h1>Welcome</h1>
	<p>This is a <strong>test</strong> page with <a href="https://example.com">links</a>.</p>
	<ul>
		<li>Item 1</li>
		<li>Item 2</li>
	</ul>
</body>
</html>`,
			filePath:        "index.html",
			expectSubstring: "# Welcome",
			description:     "should convert HTML to markdown",
		},
		{
			name: "process complex markdown file",
			body: `# Project Documentation

## Overview

This project demonstrates **advanced features** including:

1. Code syntax highlighting
2. [External links](https://example.com)
3. Embedded images ![logo](logo.png)

### Code Examples

Here's some JavaScript:

` + "```javascript" + `
function greet(name) {
    console.log(` + "`Hello, ${name}!`" + `);
}
` + "```" + `

> **Note**: This is important information.

## Installation

Run the following commands:

` + "```bash" + `
npm install
npm start
` + "```" + ``,
			filePath:        "README.md",
			expectSubstring: "# Project Documentation",
			description:     "should preserve complex markdown as-is",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.processContent(tt.body, tt.filePath)

			if result == "" {
				t.Errorf("Expected non-empty result for test: %s", tt.description)
			}

			if !strings.Contains(result, tt.expectSubstring) {
				t.Errorf("Expected result to contain '%s' for test: %s\nGot: %s",
					tt.expectSubstring, tt.description, result)
			}
		})
	}
}

func TestGitHubTransformer_DatabaseErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database and then close it
	db := testutil.SetupTestDB(t)
	db.Close()

	transformer := NewGitHubTransformer()

	download := &models.Download{
		ID:       "download-error",
		SourceID: "source-error",
		Headers:  `{"X-GitHub-SHA": ["abc123"], "Content-Type": ["text/plain"]}`,
		Body:     stringPtrGH("# Test file content"),
	}

	t.Run("database connection error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := transformer.Transform(ctx, download, db)

		// Should get an error due to closed database connection
		if err == nil {
			t.Error("Expected error due to closed database connection")
		}

		if result != nil {
			t.Error("Expected nil result when database operation fails")
		}
	})
}

// Helper function to create string pointer
func stringPtrGH(s string) *string {
	return &s
}
