package transformers

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/internal/manager/testutil"
)

func TestWPJSONTransformer_Transform_DatabaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	transformer := NewWPJSONTransformer()

	tests := []struct {
		name        string
		download    *models.Download
		expectError bool
		description string
	}{
		{
			name: "transform valid WordPress post",
			download: &models.Download{
				ID:       "download-123",
				SourceID: "source-123",
				Headers:  `{"Content-Type": ["application/json"]}`,
				Body: stringPtrWPJSON(`{
					"id": 42,
					"title": {
						"rendered": "Integration Test Post"
					},
					"content": {
						"rendered": "<h1>Welcome</h1><p>This is a test post with <strong>formatting</strong> and <a href='https://example.com'>links</a>.</p><ul><li>Item 1</li><li>Item 2</li></ul>"
					},
					"excerpt": {
						"rendered": "<p>This is a test excerpt with <em>emphasis</em>.</p>"
					},
					"date_gmt": "2023-01-01T10:30:00",
					"modified_gmt": "2023-01-02T15:45:00",
					"link": "https://example.com/integration-test-post",
					"author": 5,
					"status": "publish",
					"type": "post",
					"slug": "integration-test-post",
					"featured_media": 123,
					"categories": [1, 2, 3],
					"tags": [4, 5]
				}`),
			},
			expectError: false,
			description: "should transform WordPress post to document and create all records",
		},
		{
			name: "transform minimal WordPress post",
			download: &models.Download{
				ID:       "download-456",
				SourceID: "source-456",
				Headers:  `{"Content-Type": ["application/json"]}`,
				Body: stringPtrWPJSON(`{
					"id": 43,
					"title": {
						"rendered": "Minimal Test Post"
					},
					"content": {
						"rendered": "<p>Simple content.</p>"
					},
					"date_gmt": "2023-01-01T10:30:00",
					"modified_gmt": "2023-01-01T10:30:00"
				}`),
			},
			expectError: false,
			description: "should transform minimal WordPress post",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Setup required parent records for foreign key constraints
			setupTestSource(t, db, tt.download.SourceID)
			setupTestDownload(t, db, tt.download)

			// Verify database is clean before test
			documentCount := testutil.GetRecordCount(t, db, "documents")
			metaCount := testutil.GetRecordCount(t, db, "document_meta")

			if documentCount != 0 {
				t.Errorf("Expected 0 documents before test, got %d", documentCount)
			}
			if metaCount != 0 {
				t.Errorf("Expected 0 document_meta before test, got %d", metaCount)
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
				var sourceID, downloadID, format string
				var minChunkSize, maxChunkSize int
				query := "SELECT source_id, download_id, format, min_chunk_size, max_chunk_size FROM documents WHERE id = ?"
				err := db.QueryRow(query, documentID).
					Scan(&sourceID, &downloadID, &format, &minChunkSize, &maxChunkSize)
				if err != nil {
					t.Errorf("Failed to query document data: %v", err)
				} else {
					if sourceID != tt.download.SourceID {
						t.Errorf("Expected source_id %s, got %s", tt.download.SourceID, sourceID)
					}
					if downloadID != tt.download.ID {
						t.Errorf("Expected download_id %s, got %s", tt.download.ID, downloadID)
					}
					if format != "json" {
						t.Errorf("Expected format 'json', got %s", format)
					}
					if minChunkSize != 212 {
						t.Errorf("Expected min_chunk_size 212, got %d", minChunkSize)
					}
					if maxChunkSize != 8191 {
						t.Errorf("Expected max_chunk_size 8191, got %d", maxChunkSize)
					}
				}

				// Verify metadata was created
				metaCountAfter := testutil.GetRecordCount(t, db, "document_meta")
				if metaCountAfter == 0 {
					t.Errorf("Expected metadata records to be created for test: %s", tt.description)
				}

				// Verify specific metadata exists
				metadataKeys := []string{"document_title", "links_count"}
				for _, key := range metadataKeys {
					var count int
					query := "SELECT COUNT(*) FROM document_meta WHERE document_id = ? AND key = ?"
					err := db.QueryRow(query, documentID, key).Scan(&count)
					if err != nil {
						t.Errorf("Failed to check metadata key %s: %v", key, err)
					} else if count == 0 {
						t.Errorf("Expected metadata key %s not found for test: %s", key, tt.description)
					}
				}

				// For the full test, verify content conversion
				if tt.name == "transform valid WordPress post" {
					// Verify that content was converted from HTML to markdown
					var titleMeta string
					query = "SELECT meta FROM document_meta WHERE document_id = ? AND key = 'document_title'"
					err = db.QueryRow(query, documentID).Scan(&titleMeta)
					if err != nil {
						t.Errorf("Failed to get document title: %v", err)
					} else if titleMeta != "Integration Test Post" {
						t.Errorf("Expected title 'Integration Test Post', got %s", titleMeta)
					}

					// Verify links count
					var linksCount string
					query = "SELECT meta FROM document_meta WHERE document_id = ? AND key = 'links_count'"
					err = db.QueryRow(query, documentID).Scan(&linksCount)
					if err != nil {
						t.Errorf("Failed to get links count: %v", err)
					} else if linksCount != "1" {
						t.Errorf("Expected links_count '1', got %s", linksCount)
					}
				}

				// Verify database counts increased
				newDocumentCount := testutil.GetRecordCount(t, db, "documents")
				if newDocumentCount != documentCount+1 {
					t.Errorf("Expected document count to increase by 1, got %d -> %d", documentCount, newDocumentCount)
				}

				// Clean up for next test
				_, err = db.Exec("DELETE FROM document_meta WHERE document_id = ?", documentID)
				if err != nil {
					t.Errorf("Failed to clean up document_meta: %v", err)
				}
				_, err = db.Exec("DELETE FROM documents WHERE id = ?", documentID)
				if err != nil {
					t.Errorf("Failed to clean up document: %v", err)
				}
			}
		})
	}
}

func TestWPJSONTransformer_CanTransform_DatabaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	transformer := NewWPJSONTransformer()

	tests := []struct {
		name        string
		download    *models.Download
		expected    bool
		description string
	}{
		{
			name: "valid WordPress JSON",
			download: &models.Download{
				Body: stringPtrWPJSON(`{
					"id": 1,
					"title": {"rendered": "Test"},
					"content": {"rendered": "<p>Content</p>"},
					"date_gmt": "2023-01-01T10:30:00",
					"modified_gmt": "2023-01-01T10:30:00"
				}`),
			},
			expected:    true,
			description: "should detect valid WordPress JSON",
		},
		{
			name: "invalid JSON",
			download: &models.Download{
				Body: stringPtr(`invalid json content`),
			},
			expected:    false,
			description: "should reject invalid JSON",
		},
		{
			name: "missing required fields",
			download: &models.Download{
				Body: stringPtrWPJSON(`{
					"id": 1,
					"title": {"rendered": "Test"}
				}`),
			},
			expected:    false,
			description: "should reject JSON missing required fields",
		},
		{
			name: "nil body",
			download: &models.Download{
				Body: nil,
			},
			expected:    false,
			description: "should reject nil body",
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

func TestWPJSONTransformer_ExtractContent_DatabaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	transformer := NewWPJSONTransformer()

	tests := []struct {
		name        string
		wpData      map[string]interface{}
		expectError bool
		expectedMD  string
		description string
	}{
		{
			name: "complex HTML conversion",
			wpData: map[string]interface{}{
				"content": map[string]interface{}{
					"rendered": `<h1>Main Title</h1>
					<h2>Subtitle</h2>
					<p>This is a paragraph with <strong>bold</strong> and <em>italic</em> text.</p>
					<ul>
					<li>List item 1</li>
					<li>List item 2 with <a href="https://example.com">a link</a></li>
					</ul>
					<blockquote>This is a quote</blockquote>
					<pre><code>console.log("code block");</code></pre>`,
				},
			},
			expectError: false,
			expectedMD:  "# Main Title",
			description: "should convert complex HTML to markdown",
		},
		{
			name: "WordPress shortcodes and embeds",
			wpData: map[string]interface{}{
				"content": map[string]interface{}{
					"rendered": `<p>Check out this video:</p>
					<figure class="wp-block-embed">
					<div class="wp-block-embed__wrapper">
					https://www.youtube.com/watch?v=example
					</div>
					</figure>
					<p>And here's a gallery:</p>
					[gallery ids="1,2,3"]`,
				},
			},
			expectError: false,
			expectedMD:  "Check out this video:",
			description: "should handle WordPress-specific content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := transformer.extractContent(tt.wpData)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if !tt.expectError {
				if content == "" {
					t.Errorf("Expected non-empty content for test: %s", tt.description)
				}
				if tt.expectedMD != "" && !strings.Contains(content, tt.expectedMD) {
					t.Errorf("Expected content to contain '%s', got: %s", tt.expectedMD, content)
				}
			}
		})
	}
}

func TestWPJSONTransformer_DatabaseErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database and then close it
	db := testutil.SetupTestDB(t)
	db.Close()

	transformer := NewWPJSONTransformer()

	download := &models.Download{
		ID:       "download-error",
		SourceID: "source-error",
		Headers:  `{"Content-Type": ["application/json"]}`,
		Body: stringPtrWPJSON(`{
			"id": 1,
			"title": {"rendered": "Test"},
			"content": {"rendered": "<p>Content</p>"},
			"date_gmt": "2023-01-01T10:30:00",
			"modified_gmt": "2023-01-01T10:30:00"
		}`),
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

// setupTestSource creates a test source record
func setupTestSource(t *testing.T, db *sql.DB, sourceID string) {
	t.Helper()
	
	_, err := db.Exec(`
		INSERT OR IGNORE INTO sources (id, active_domain, created_at, updated_at) 
		VALUES (?, 1, datetime('now'), datetime('now'))
	`, sourceID)
	if err != nil {
		t.Fatalf("Failed to create test source: %v", err)
	}
}

// setupTestDownload creates a test download record
func setupTestDownload(t *testing.T, db *sql.DB, download *models.Download) {
	t.Helper()
	
	_, err := db.Exec(`
		INSERT OR IGNORE INTO downloads (id, source_id, headers, body) 
		VALUES (?, ?, ?, ?)
	`, download.ID, download.SourceID, download.Headers, *download.Body)
	if err != nil {
		t.Fatalf("Failed to create test download: %v", err)
	}
}

// Helper function to create string pointer
func stringPtrWPJSON(s string) *string {
	return &s
}
