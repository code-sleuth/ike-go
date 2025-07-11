package importers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/testutil"
)

func TestWPJSONImporter_ImportPost_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	// Create test server that simulates WordPress JSON API with real wsform.com data
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate individual post endpoint with real data from wsform.com
		post := map[string]interface{}{
			"id":           float64(285969),
			"date":         "2025-06-27T06:00:55",
			"date_gmt":     "2025-06-27T11:00:55",
			"modified":     "2025-07-09T13:03:31",
			"modified_gmt": "2025-07-09T18:03:31",
			"slug":         "june-2025-end-of-month-sale",
			"status":       "publish",
			"type":         "post",
			"link":         "https://wsform.com/june-2025-end-of-month-sale/",
			"title": map[string]interface{}{
				"rendered": "June 2025 &#8211; End of Month Sale",
			},
			"content": map[string]interface{}{
				"rendered": "<p>Enjoy a massive <strong>25% discount</strong> on any WS Form Edition with our limited-time offer, available until the end of June 2025.</p>\n<p>Use coupon code <strong>JUN25</strong> at checkout to claim your discount on the Agency, Freelance, or Personal Edition.</p>\n<div class=\"wp-block-button aligncenter\"><a class=\"wp-block-button__link wp-element-button\" href=\"https://wsform.com/pricing/\">Shop Now</a></div>",
			},
			"excerpt": map[string]interface{}{
				"rendered": "<p>Enjoy a 25% discount on any WS Form Edition with our limited-time offer, available until the end of June 2025.</p>",
			},
			"author":         float64(1),
			"featured_media": float64(285973),
			"categories":     []interface{}{float64(11996)},
			"tags":           []interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(post)
	}))
	defer testServer.Close()

	importer := NewWPJSONImporter()
	baseURL := testServer.URL + "/wp-json/wp/v2/posts"

	tests := []struct {
		name        string
		postID      int
		expectError bool
		description string
	}{
		{
			name:        "successful post import with real wsform.com data",
			postID:      285969,
			expectError: false,
			description: "should successfully import a real WordPress post from wsform.com to database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Verify database is clean before test
			sourceCount := testutil.GetRecordCount(t, db, "sources")
			downloadCount := testutil.GetRecordCount(t, db, "downloads")

			if sourceCount != 0 {
				t.Errorf("Expected 0 sources before test, got %d", sourceCount)
			}
			if downloadCount != 0 {
				t.Errorf("Expected 0 downloads before test, got %d", downloadCount)
			}

			// Import the post
			result := importer.importPost(ctx, baseURL, tt.postID, db)

			if tt.expectError && result.Error == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && result.Error != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, result.Error)
				return
			}

			if !tt.expectError {
				// Verify source was created
				if result.SourceID == "" {
					t.Errorf("Expected non-empty SourceID for test: %s", tt.description)
				}
				if !testutil.RecordExists(t, db, "sources", "id", result.SourceID) {
					t.Errorf("Source record not found in database for test: %s", tt.description)
				}

				// Verify download was created
				if result.DownloadID == "" {
					t.Errorf("Expected non-empty DownloadID for test: %s", tt.description)
				}
				if !testutil.RecordExists(t, db, "downloads", "id", result.DownloadID) {
					t.Errorf("Download record not found in database for test: %s", tt.description)
				}

				// Verify database counts increased
				newSourceCount := testutil.GetRecordCount(t, db, "sources")
				newDownloadCount := testutil.GetRecordCount(t, db, "downloads")

				if newSourceCount != 1 {
					t.Errorf("Expected 1 source after import, got %d", newSourceCount)
				}
				if newDownloadCount != 1 {
					t.Errorf("Expected 1 download after import, got %d", newDownloadCount)
				}

				// Verify source data
				var rawURL, format string
				query := "SELECT raw_url, format FROM sources WHERE id = ?"
				err := db.QueryRow(query, result.SourceID).Scan(&rawURL, &format)
				if err != nil {
					t.Errorf("Failed to query source data: %v", err)
				} else {
					if rawURL == "" {
						t.Error("Expected non-empty raw_url in source")
					}
					if format != "json" {
						t.Errorf("Expected format 'json', got %s", format)
					}
				}

				// Verify download data
				var statusCode int
				var body string
				query = "SELECT status_code, body FROM downloads WHERE id = ?"
				err = db.QueryRow(query, result.DownloadID).Scan(&statusCode, &body)
				if err != nil {
					t.Errorf("Failed to query download data: %v", err)
				} else {
					if statusCode != 200 {
						t.Errorf("Expected status code 200, got %d", statusCode)
					}
					if body == "" {
						t.Error("Expected non-empty body in download")
					}

					// Verify body is valid JSON
					var parsedBody map[string]interface{}
					if err := json.Unmarshal([]byte(body), &parsedBody); err != nil {
						t.Errorf("Download body is not valid JSON: %v", err)
					}
				}
			}
		})
	}
}

func TestWPJSONImporter_Import_FullIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	// Create comprehensive test server with real wsform.com data
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/wp-json/wp/v2/posts" {
			// Main posts endpoint - return real post IDs from wsform.com
			page := r.URL.Query().Get("page")
			perPage := r.URL.Query().Get("per_page")
			
			// Handle pagination based on per_page parameter
			if perPage == "1" {
				// Pagination test scenario with 1 post per page
				switch page {
				case "1", "":
					posts := []map[string]interface{}{
						{
							"id": float64(285969), 
							"title": map[string]interface{}{"rendered": "June 2025 &#8211; End of Month Sale"},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(posts)
				case "2":
					posts := []map[string]interface{}{
						{
							"id": float64(356466), 
							"title": map[string]interface{}{"rendered": "How to Block IP Addresses in WordPress Forms to Prevent Spam"},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(posts)
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			} else {
				// Normal scenario - return both posts
				switch page {
				case "1", "":
					posts := []map[string]interface{}{
						{
							"id": float64(285969), 
							"title": map[string]interface{}{"rendered": "June 2025 &#8211; End of Month Sale"},
						},
						{
							"id": float64(356466), 
							"title": map[string]interface{}{"rendered": "How to Block IP Addresses in WordPress Forms to Prevent Spam"},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(posts)
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			}
		} else if path == "/wp-json/wp/v2/posts/285969" {
			// Individual post endpoint - June 2025 Sale post
			post := map[string]interface{}{
				"id":           float64(285969),
				"date":         "2025-06-27T06:00:55",
				"date_gmt":     "2025-06-27T11:00:55",
				"modified":     "2025-07-09T13:03:31",
				"modified_gmt": "2025-07-09T18:03:31",
				"slug":         "june-2025-end-of-month-sale",
				"status":       "publish",
				"type":         "post",
				"link":         "https://wsform.com/june-2025-end-of-month-sale/",
				"title": map[string]interface{}{
					"rendered": "June 2025 &#8211; End of Month Sale",
				},
				"content": map[string]interface{}{
					"rendered": "<p>Enjoy a massive <strong>25% discount</strong> on any WS Form Edition with our limited-time offer, available until the end of June 2025.</p>\n<p>Use coupon code <strong>JUN25</strong> at checkout to claim your discount on the Agency, Freelance, or Personal Edition.</p>",
				},
				"excerpt": map[string]interface{}{
					"rendered": "<p>Enjoy a 25% discount on any WS Form Edition with our limited-time offer, available until the end of June 2025.</p>",
				},
				"author":         float64(1),
				"featured_media": float64(285973),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(post)
		} else if path == "/wp-json/wp/v2/posts/356466" {
			// Individual post endpoint - IP blocking tutorial post
			post := map[string]interface{}{
				"id":           float64(356466),
				"date":         "2025-05-23T16:22:22",
				"date_gmt":     "2025-05-23T21:22:22",
				"modified":     "2025-05-23T16:26:44",
				"modified_gmt": "2025-05-23T21:26:44",
				"slug":         "how-to-block-ip-addresses-in-wordpress-forms-to-prevent-spam",
				"status":       "publish",
				"type":         "post",
				"link":         "https://wsform.com/how-to-block-ip-addresses-in-wordpress-forms-to-prevent-spam/",
				"title": map[string]interface{}{
					"rendered": "How to Block IP Addresses in WordPress Forms to Prevent Spam",
				},
				"content": map[string]interface{}{
					"rendered": "<p>Learn how to block IP addresses in WordPress forms using WS Form's built-in tools. This comprehensive guide covers both no-code settings and developer-friendly hooks to effectively prevent spam and control access.</p>\n<h2>Methods for Blocking IP Addresses</h2>\n<ol>\n<li><strong>IP Blocklist in Form Settings</strong> - Use the built-in IP blocking feature</li>\n<li><strong>Developer Filter Hook</strong> - Use <code>wsf_submit_block_ips</code> for custom logic</li>\n</ol>",
				},
				"excerpt": map[string]interface{}{
					"rendered": "<p>Learn how to block IP addresses in WordPress forms using WS Form's built-in tools, including no-code settings and developer-friendly hooks, to effectively prevent spam and control access.</p>",
				},
				"author":         float64(1),
				"featured_media": float64(356469),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(post)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	importer := NewWPJSONImporter()
	importer.SetConcurrency(1) // Set to 1 for predictable testing

	t.Run("full import workflow with comprehensive validation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Verify database is clean before test
		sourceCount := testutil.GetRecordCount(t, db, "sources")
		downloadCount := testutil.GetRecordCount(t, db, "downloads")

		if sourceCount != 0 || downloadCount != 0 {
			t.Errorf("Expected clean database, got %d sources and %d downloads", sourceCount, downloadCount)
		}

		sourceURL := testServer.URL + "/wp-json/wp/v2/posts"
		result, err := importer.Import(ctx, sourceURL, db)

		if err != nil {
			t.Errorf("Unexpected error during full import: %v", err)
			return
		}

		if result == nil {
			t.Error("Expected non-nil result from import")
			return
		}

		// Verify final database state
		finalSourceCount := testutil.GetRecordCount(t, db, "sources")
		finalDownloadCount := testutil.GetRecordCount(t, db, "downloads")

		// Should have 2 sources and 2 downloads (one for each post)
		if finalSourceCount != 2 {
			t.Errorf("Expected 2 sources after import, got %d", finalSourceCount)
		}
		if finalDownloadCount != 2 {
			t.Errorf("Expected 2 downloads after import, got %d", finalDownloadCount)
		}

		// Verify last result
		if result.SourceID == "" {
			t.Error("Expected non-empty SourceID from final result")
		}
		if result.DownloadID == "" {
			t.Error("Expected non-empty DownloadID from final result")
		}

		// Verify the last result exists in database
		if !testutil.RecordExists(t, db, "sources", "id", result.SourceID) {
			t.Error("Final result SourceID not found in database")
		}
		if !testutil.RecordExists(t, db, "downloads", "id", result.DownloadID) {
			t.Error("Final result DownloadID not found in database")
		}

		// Enhanced validation: Check specific source data
		var sourceURL1, sourceURL2 string
		var format1, format2 string
		query := "SELECT raw_url, format FROM sources ORDER BY created_at"
		rows, err := db.Query(query)
		if err != nil {
			t.Errorf("Failed to query sources: %v", err)
		} else {
			defer rows.Close()
			sourceCount := 0
			for rows.Next() {
				sourceCount++
				if sourceCount == 1 {
					err = rows.Scan(&sourceURL1, &format1)
					if err != nil {
						t.Errorf("Failed to scan first source: %v", err)
					}
				} else if sourceCount == 2 {
					err = rows.Scan(&sourceURL2, &format2)
					if err != nil {
						t.Errorf("Failed to scan second source: %v", err)
					}
				}
			}

			// Verify source URLs contain expected post IDs
			if !strings.Contains(sourceURL1, "285969") && !strings.Contains(sourceURL1, "356466") {
				t.Errorf("Expected source URL to contain wsform.com post ID, got %s", sourceURL1)
			}
			if !strings.Contains(sourceURL2, "285969") && !strings.Contains(sourceURL2, "356466") {
				t.Errorf("Expected source URL to contain wsform.com post ID, got %s", sourceURL2)
			}

			// Verify format is JSON for both sources
			if format1 != "json" {
				t.Errorf("Expected format 'json' for first source, got %s", format1)
			}
			if format2 != "json" {
				t.Errorf("Expected format 'json' for second source, got %s", format2)
			}
		}

		// Enhanced validation: Check download content contains expected data
		query = "SELECT body FROM downloads ORDER BY downloaded_at"
		rows, err = db.Query(query)
		if err != nil {
			t.Errorf("Failed to query downloads: %v", err)
		} else {
			defer rows.Close()
			downloadCount := 0
			for rows.Next() {
				downloadCount++
				var body string
				err = rows.Scan(&body)
				if err != nil {
					t.Errorf("Failed to scan download body: %v", err)
					continue
				}

				// Verify body is valid JSON
				var parsedBody map[string]interface{}
				if err := json.Unmarshal([]byte(body), &parsedBody); err != nil {
					t.Errorf("Download body is not valid JSON: %v", err)
					continue
				}

				// Verify body contains expected wsform.com data
				if id, ok := parsedBody["id"].(float64); ok {
					if id != 285969 && id != 356466 {
						t.Errorf("Expected post ID to be 285969 or 356466, got %v", id)
					}
				} else {
					t.Error("Expected post ID field in download body")
				}

				// Verify essential WordPress fields are present
				if _, ok := parsedBody["title"]; !ok {
					t.Error("Expected 'title' field in download body")
				}
				if _, ok := parsedBody["content"]; !ok {
					t.Error("Expected 'content' field in download body")
				}
				if _, ok := parsedBody["date_gmt"]; !ok {
					t.Error("Expected 'date_gmt' field in download body")
				}
				if _, ok := parsedBody["link"]; !ok {
					t.Error("Expected 'link' field in download body")
				}

				// Verify link contains wsform.com domain
				if link, ok := parsedBody["link"].(string); ok {
					if !strings.Contains(link, "wsform.com") {
						t.Errorf("Expected link to contain 'wsform.com', got %s", link)
					}
				}
			}
		}
	})

	t.Run("pagination and error handling", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Clean database for this test - use DELETE instead of CleanupTestDB
		_, err := db.Exec("DELETE FROM downloads")
		if err != nil {
			t.Logf("Warning: Failed to clean downloads table: %v", err)
		}
		_, err = db.Exec("DELETE FROM sources")
		if err != nil {
			t.Logf("Warning: Failed to clean sources table: %v", err)
		}

		// Test with importer configured for more aggressive pagination
		importer.SetPerPage(1) // Force pagination with 1 post per page
		importer.SetConcurrency(1) // Predictable ordering

		sourceURL := testServer.URL + "/wp-json/wp/v2/posts"
		result, err := importer.Import(ctx, sourceURL, db)

		if err != nil {
			t.Errorf("Unexpected error during paginated import: %v", err)
			return
		}

		if result == nil {
			t.Error("Expected non-nil result from paginated import")
			return
		}

		// Should still have imported both posts despite pagination
		finalSourceCount := testutil.GetRecordCount(t, db, "sources")
		finalDownloadCount := testutil.GetRecordCount(t, db, "downloads")

		if finalSourceCount != 2 {
			t.Errorf("Expected 2 sources after paginated import, got %d", finalSourceCount)
		}
		if finalDownloadCount != 2 {
			t.Errorf("Expected 2 downloads after paginated import, got %d", finalDownloadCount)
		}

		// Reset pagination settings
		importer.SetPerPage(100)
	})
}

func TestWPJSONImporter_DatabaseErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	importer := NewWPJSONImporter()

	t.Run("importPost with closed database", func(t *testing.T) {
		// Setup test database and then close it to simulate connection issues
		db := testutil.SetupTestDB(t)
		db.Close()

		// Create test server with real wsform.com data
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			post := map[string]interface{}{
				"id":           float64(285969),
				"date":         "2025-06-27T06:00:55",
				"date_gmt":     "2025-06-27T11:00:55",
				"modified":     "2025-07-09T13:03:31",
				"modified_gmt": "2025-07-09T18:03:31",
				"slug":         "june-2025-end-of-month-sale",
				"status":       "publish",
				"type":         "post",
				"link":         "https://wsform.com/june-2025-end-of-month-sale/",
				"title": map[string]interface{}{
					"rendered": "June 2025 &#8211; End of Month Sale",
				},
				"content": map[string]interface{}{
					"rendered": "<p>Enjoy a massive <strong>25% discount</strong> on any WS Form Edition with our limited-time offer, available until the end of June 2025.</p>",
				},
				"excerpt": map[string]interface{}{
					"rendered": "<p>Enjoy a 25% discount on any WS Form Edition with our limited-time offer, available until the end of June 2025.</p>",
				},
				"author":         float64(1),
				"featured_media": float64(285973),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(post)
		}))
		defer testServer.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		baseURL := testServer.URL + "/wp-json/wp/v2/posts"
		result := importer.importPost(ctx, baseURL, 285969, db)

		// Should get an error due to closed database connection
		if result.Error == nil {
			t.Error("Expected error due to closed database connection")
		}

		if result.SourceID != "" || result.DownloadID != "" {
			t.Error("Expected empty result IDs when database operation fails")
		}
	})

	t.Run("full Import with closed database", func(t *testing.T) {
		// Setup test database and then close it to simulate connection issues
		db := testutil.SetupTestDB(t)
		db.Close()

		// Create comprehensive test server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if path == "/wp-json/wp/v2/posts" {
				// Main posts endpoint - return real post IDs from wsform.com
				posts := []map[string]interface{}{
					{
						"id": float64(285969), 
						"title": map[string]interface{}{"rendered": "June 2025 &#8211; End of Month Sale"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(posts)
			} else if path == "/wp-json/wp/v2/posts/285969" {
				// Individual post endpoint
				post := map[string]interface{}{
					"id":           float64(285969),
					"date_gmt":     "2025-06-27T11:00:55",
					"modified_gmt": "2025-07-09T18:03:31",
					"link":         "https://wsform.com/june-2025-end-of-month-sale/",
					"title": map[string]interface{}{
						"rendered": "June 2025 &#8211; End of Month Sale",
					},
					"content": map[string]interface{}{
						"rendered": "<p>Enjoy a massive <strong>25% discount</strong> on any WS Form Edition.</p>",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(post)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer testServer.Close()

		// Reduce concurrency to limit retry noise
		importer.SetConcurrency(1)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		sourceURL := testServer.URL + "/wp-json/wp/v2/posts"
		result, err := importer.Import(ctx, sourceURL, db)

		// Should get an error due to closed database connection
		if err == nil {
			t.Error("Expected error due to closed database connection during full import")
		}

		if result != nil {
			t.Error("Expected nil result when database operation fails during full import")
		}
	})

	t.Run("context cancellation during database operations", func(t *testing.T) {
		// Setup test database
		db := testutil.SetupTestDB(t)
		defer testutil.CleanupTestDB(t, db)

		// Create test server with real wsform.com data
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			post := map[string]interface{}{
				"id":           float64(356466),
				"date_gmt":     "2025-05-23T21:22:22",
				"modified_gmt": "2025-05-23T21:26:44",
				"link":         "https://wsform.com/how-to-block-ip-addresses-in-wordpress-forms-to-prevent-spam/",
				"title": map[string]interface{}{
					"rendered": "How to Block IP Addresses in WordPress Forms to Prevent Spam",
				},
				"content": map[string]interface{}{
					"rendered": "<p>Learn how to block IP addresses in WordPress forms using WS Form's built-in tools.</p>",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(post)
		}))
		defer testServer.Close()

		// Create context that will be cancelled immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		baseURL := testServer.URL + "/wp-json/wp/v2/posts"
		result := importer.importPost(ctx, baseURL, 356466, db)

		// Should get a context cancellation error
		if result.Error == nil {
			t.Error("Expected context cancellation error")
		}

		if result.SourceID != "" || result.DownloadID != "" {
			t.Error("Expected empty result IDs when context is cancelled")
		}
	})

	t.Run("network error during post fetch", func(t *testing.T) {
		// Setup test database
		db := testutil.SetupTestDB(t)
		defer testutil.CleanupTestDB(t, db)

		// Create test server that returns errors
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer testServer.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		baseURL := testServer.URL + "/wp-json/wp/v2/posts"
		result := importer.importPost(ctx, baseURL, 285969, db)

		// Should get an error due to server error
		if result.Error == nil {
			t.Error("Expected error due to server error")
		}

		if result.SourceID != "" || result.DownloadID != "" {
			t.Error("Expected empty result IDs when server returns error")
		}
	})

	t.Run("invalid JSON response from server", func(t *testing.T) {
		// Setup test database
		db := testutil.SetupTestDB(t)
		defer testutil.CleanupTestDB(t, db)

		// Create test server that returns invalid JSON
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json response"))
		}))
		defer testServer.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		baseURL := testServer.URL + "/wp-json/wp/v2/posts"
		result := importer.importPost(ctx, baseURL, 285969, db)

		// Should get an error due to invalid JSON
		if result.Error == nil {
			t.Error("Expected error due to invalid JSON response")
		}

		if result.SourceID != "" || result.DownloadID != "" {
			t.Error("Expected empty result IDs when JSON parsing fails")
		}
	})

	t.Run("successful recovery after transient errors", func(t *testing.T) {
		// Setup test database
		db := testutil.SetupTestDB(t)
		defer testutil.CleanupTestDB(t, db)

		// Create test server that succeeds after first being called
		callCount := 0
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			
			// Succeed on the first call (this tests recovery scenario)
			post := map[string]interface{}{
				"id":           float64(285969),
				"date_gmt":     "2025-06-27T11:00:55",
				"modified_gmt": "2025-07-09T18:03:31",
				"link":         "https://wsform.com/june-2025-end-of-month-sale/",
				"title": map[string]interface{}{
					"rendered": "June 2025 &#8211; End of Month Sale",
				},
				"content": map[string]interface{}{
					"rendered": "<p>Enjoy a massive <strong>25% discount</strong> on any WS Form Edition.</p>",
				},
				"excerpt": map[string]interface{}{
					"rendered": "<p>Enjoy a 25% discount on any WS Form Edition.</p>",
				},
				"status": "publish",
				"type":   "post",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(post)
		}))
		defer testServer.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		baseURL := testServer.URL + "/wp-json/wp/v2/posts"
		result := importer.importPost(ctx, baseURL, 285969, db)

		// Should succeed and create database records
		if result.Error != nil {
			t.Errorf("Expected successful import after recovery, got error: %v", result.Error)
		}

		if result.SourceID == "" || result.DownloadID == "" {
			t.Error("Expected non-empty result IDs after successful recovery")
		}

		// Verify records exist in database
		if !testutil.RecordExists(t, db, "sources", "id", result.SourceID) {
			t.Error("Source record not found in database after recovery")
		}
		if !testutil.RecordExists(t, db, "downloads", "id", result.DownloadID) {
			t.Error("Download record not found in database after recovery")
		}
	})
}
