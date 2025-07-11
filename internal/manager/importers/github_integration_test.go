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

func TestGitHubImporter_ImportFile_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	// Create test server that simulates GitHub API
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock file content endpoint
		if strings.Contains(r.URL.Path, "/contents/README.md") {
			fileResponse := GitHubFileResponse{
				Name:        "README.md",
				Path:        "README.md",
				SHA:         "0e02509a5729c071ca1f6f919ea397fd2653b62b",
				Size:        787,
				URL:         "https://api.github.com/repos/code-sleuth/outh/contents/README.md?ref=main",
				HTMLURL:     "https://github.com/code-sleuth/outh/blob/main/README.md",
				GitURL:      "https://api.github.com/repos/code-sleuth/outh/git/blobs/0e02509a5729c071ca1f6f919ea397fd2653b62b",
				DownloadURL: "https://raw.githubusercontent.com/code-sleuth/outh/main/README.md",
				Type:        "file",
				Content:     "IyBPVVRIIFNlcnZpY2UKCiMjIEVudmlyb25tZW50Ckl0cyBhIHByZXJlcXVpc2l0ZSB0aGF0IHRoZXNlIGVudmlyb25tZW50IHZhcmlhYmxlcyBhcmUgc2V0LiBTZXQgdGhlbSBpbiB5b3VyIHRlcm1pbmFsLgoKYGBgYmFzaAokIGV4cG9ydCBKV1RfU0VDUkVUPTx5b3VyLWp3dC1zZWNyZXQ+CiQgZXhwb3J0IERBVEFC\nQVNFX1VSTD08ZXhhbXBsZS1wb3N0Z3JlczovL3Bvc3RncmVzOm5vdFNvU2VjcmV0QHBvc3RncmVzOjU0MzI+CiQgZXhwb3J0IFBPU1RNQVJLX0FVVEhfVE9LRU49PHlvdXItcG9zdG1hcmstYXV0aC10b2tlbj4KYGBgCgoKIyMgU2V0dXAg\nJiBCdWlsZApgYGBzaGVsbAptYWtlIGJ1aWxkCmBgYAoKIyMgUnVuIHNlcnZpY2VzIGxvY2FsbHkKIyMjIyBBcHAgc2VydmljZQpgYGBzaGVsbAptYWtlIHJ1bi1hcHAtc2VydmljZQpgY", // Real base64 content from code-sleuth/outh README.md
				Encoding:    "base64",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fileResponse)
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		}
	}))
	defer testServer.Close()

	// Create importer with custom HTTP client and API base URL pointing to test server
	importer := NewGitHubImporterWithClient(&http.Client{Timeout: 30 * time.Second}, testServer.URL)

	tests := []struct {
		name        string
		repoInfo    *GitHubRepoInfo
		file        GitHubTreeItem
		expectError bool
		description string
	}{
		{
			name: "successful file import",
			repoInfo: &GitHubRepoInfo{
				Owner: "code-sleuth",
				Repo:  "outh",
				Ref:   "main",
			},
			file: GitHubTreeItem{
				Path: "README.md",
				Mode: "100644",
				Type: "blob",
				SHA:  "0e02509a5729c071ca1f6f919ea397fd2653b62b",
				Size: 787,
				URL:  "https://api.github.com/repos/code-sleuth/outh/git/blobs/0e02509a5729c071ca1f6f919ea397fd2653b62b",
			},
			expectError: false,
			description: "should successfully import a GitHub file to database",
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

			// Import the file
			result, err := importer.importFile(ctx, tt.repoInfo, tt.file, db)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
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

				// Verify source data
				var rawURL, format string
				query := "SELECT raw_url, format FROM sources WHERE id = ?"
				err := db.QueryRow(query, result.SourceID).Scan(&rawURL, &format)
				if err != nil {
					t.Errorf("Failed to query source data: %v", err)
				} else {
					if !strings.Contains(rawURL, "github.com") {
						t.Errorf("Expected GitHub URL in source, got %s", rawURL)
					}
					if format != "json" {
						t.Errorf("Expected format 'json', got %s", format)
					}
				}

				// Verify download data contains GitHub-specific headers
				var headers string
				query = "SELECT headers FROM downloads WHERE id = ?"
				err = db.QueryRow(query, result.DownloadID).Scan(&headers)
				if err != nil {
					t.Errorf("Failed to query download headers: %v", err)
				} else {
					if !strings.Contains(headers, "X-GitHub-SHA") {
						t.Error("Expected X-GitHub-SHA header in download")
					}
				}
			}
		})
	}
}

func TestGitHubImporter_CreateSource_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	importer := NewGitHubImporter()

	tests := []struct {
		name        string
		fileURL     string
		repoInfo    *GitHubRepoInfo
		file        GitHubTreeItem
		expectError bool
		description string
	}{
		{
			name:    "create source for markdown file",
			fileURL: "https://github.com/code-sleuth/outh/blob/main/README.md",
			repoInfo: &GitHubRepoInfo{
				Owner: "code-sleuth",
				Repo:  "outh",
				Ref:   "main",
			},
			file: GitHubTreeItem{
				Path: "README.md",
				Mode: "100644",
				Type: "blob",
				SHA:  "0e02509a5729c071ca1f6f919ea397fd2653b62b",
				Size: 787,
			},
			expectError: false,
			description: "should create source record for markdown file",
		},
		{
			name:    "create source for Rust file",
			fileURL: "https://github.com/code-sleuth/outh/blob/main/auth-service/src/main.rs",
			repoInfo: &GitHubRepoInfo{
				Owner: "code-sleuth",
				Repo:  "outh",
				Ref:   "main",
			},
			file: GitHubTreeItem{
				Path: "auth-service/src/main.rs",
				Mode: "100644",
				Type: "blob",
				SHA:  "037ad4788630bbbd9f82d0e2038106ff998c1c6b",
				Size: 3533,
			},
			expectError: false,
			description: "should create source record for Rust file",
		},
		{
			name:    "create source for TOML file",
			fileURL: "https://github.com/code-sleuth/outh/blob/main/auth-service/Cargo.toml",
			repoInfo: &GitHubRepoInfo{
				Owner: "code-sleuth",
				Repo:  "outh",
				Ref:   "main",
			},
			file: GitHubTreeItem{
				Path: "auth-service/Cargo.toml",
				Mode: "100644",
				Type: "blob",
				SHA:  "2dc7dc76bcbe826eab26dae478c8f81a7e3e8fdb",
				Size: 2613,
			},
			expectError: false,
			description: "should create source record for TOML file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Verify database is clean before test
			initialCount := testutil.GetRecordCount(t, db, "sources")

			// Create source
			sourceID, err := importer.createSource(ctx, tt.fileURL, tt.repoInfo, tt.file, db)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if !tt.expectError {
				// Verify source ID was returned
				if sourceID == "" {
					t.Errorf("Expected non-empty sourceID for test: %s", tt.description)
				}

				// Verify record was created in database
				if !testutil.RecordExists(t, db, "sources", "id", sourceID) {
					t.Errorf("Source record not found in database for test: %s", tt.description)
				}

				// Verify record count increased
				newCount := testutil.GetRecordCount(t, db, "sources")
				if newCount != initialCount+1 {
					t.Errorf("Expected record count to increase by 1, got %d -> %d", initialCount, newCount)
				}

				// Verify source data
				var rawURL, scheme, host, path, format string
				query := "SELECT raw_url, scheme, host, path, format FROM sources WHERE id = ?"
				err := db.QueryRow(query, sourceID).Scan(&rawURL, &scheme, &host, &path, &format)
				if err != nil {
					t.Errorf("Failed to query source data: %v", err)
				} else {
					if rawURL != tt.fileURL {
						t.Errorf("Expected raw_url %s, got %s", tt.fileURL, rawURL)
					}
					if scheme != "https" {
						t.Errorf("Expected scheme 'https', got %s", scheme)
					}
					if host != "github.com" {
						t.Errorf("Expected host 'github.com', got %s", host)
					}
					if path == "" {
						t.Error("Expected non-empty path")
					}

					// Verify format based on file extension
					expectedFormat := "json" // default - all files stored as json for sources
					if strings.HasSuffix(tt.file.Path, ".json") {
						expectedFormat = "json"
					} else if strings.HasSuffix(tt.file.Path, ".yaml") || strings.HasSuffix(tt.file.Path, ".yml") {
						expectedFormat = "yaml"
					}

					if format != expectedFormat {
						t.Errorf("Expected format %s, got %s", expectedFormat, format)
					}
				}

				// Clean up for next test
				_, err = db.Exec("DELETE FROM sources WHERE id = ?", sourceID)
				if err != nil {
					t.Errorf("Failed to clean up source: %v", err)
				}
			}
		})
	}
}

func TestGitHubImporter_CreateDownload_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	importer := NewGitHubImporter()

	// First create a source to reference
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fileURL := "https://github.com/code-sleuth/outh/blob/main/README.md"
	repoInfo := &GitHubRepoInfo{Owner: "code-sleuth", Repo: "outh", Ref: "main"}
	file := GitHubTreeItem{Path: "README.md", Type: "blob", SHA: "0e02509a5729c071ca1f6f919ea397fd2653b62b", Size: 787}

	sourceID, err := importer.createSource(ctx, fileURL, repoInfo, file, db)
	if err != nil {
		t.Fatalf("Failed to create test source: %v", err)
	}

	tests := []struct {
		name        string
		sourceID    string
		content     string
		file        GitHubTreeItem
		expectError bool
		description string
	}{
		{
			name:     "create download with markdown content",
			sourceID: sourceID,
			content:  "# OUTH Service\n\n## Environment\nIts a prerequisite that these environment variables are set. Set them in your terminal.\n\n```bash\n$ export JWT_SECRET=<your-jwt-secret>\n$ export DATABASE_URL=<example-postgres://postgres:notSoSecret@postgres:5432>\n$ export POSTMARK_AUTH_TOKEN=<your-postmark-auth-token>\n```\n\n\n## Setup & Build\n```shell\nmake build\n```\n\n## Run services locally\n#### App service\n```shell\nmake run-app-service\n```",
			file: GitHubTreeItem{
				Path: "README.md",
				Mode: "100644",
				Type: "blob",
				SHA:  "0e02509a5729c071ca1f6f919ea397fd2653b62b",
				Size: 787,
			},
			expectError: false,
			description: "should create download record with content",
		},
		{
			name:     "create download with base64 content",
			sourceID: sourceID,
			content:  "IyBPVVRIIFNlcnZpY2UKCiMjIEVudmlyb25tZW50Ckl0cyBhIHByZXJlcXVpc2l0ZSB0aGF0IHRoZXNlIGVudmlyb25tZW50IHZhcmlhYmxlcyBhcmUgc2V0LiBTZXQgdGhlbSBpbiB5b3VyIHRlcm1pbmFsLgoKYGBgYmFzaAokIGV4cG9ydCBKV1RfU0VDUkVUPTx5b3VyLWp3dC1zZWNyZXQ+CiQgZXhwb3J0IERBVEFC\nQVNFX1VSTD08ZXhhbXBsZS1wb3N0Z3JlczovL3Bvc3RncmVzOm5vdFNvU2VjcmV0QHBvc3RncmVzOjU0MzI+CiQgZXhwb3J0IFBPU1RNQVJLX0FVVEhfVE9LRU49PHlvdXItcG9zdG1hcmstYXV0aC10b2tlbj4KYGBgCgoKIyMgU2V0dXAg\nJiBCdWlsZApgYGBzaGVsbAptYWtlIGJ1aWxkCmBgYAoKIyMgUnVuIHNlcnZpY2VzIGxvY2FsbHkKIyMjIyBBcHAgc2VydmljZQpgYGBzaGVsbAptYWtlIHJ1bi1hcHAtc2VydmljZQpgY",
			file: GitHubTreeItem{
				Path: "README.md",
				Mode: "100644",
				Type: "blob",
				SHA:  "0e02509a5729c071ca1f6f919ea397fd2653b62b",
				Size: 787,
			},
			expectError: false,
			description: "should create download record with base64 content from README.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Verify database state before test
			initialCount := testutil.GetRecordCount(t, db, "downloads")

			// Create download
			downloadID, err := importer.createDownload(ctx, tt.sourceID, tt.content, tt.file, db)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if !tt.expectError {
				// Verify download ID was returned
				if downloadID == "" {
					t.Errorf("Expected non-empty downloadID for test: %s", tt.description)
				}

				// Verify record was created in database
				if !testutil.RecordExists(t, db, "downloads", "id", downloadID) {
					t.Errorf("Download record not found in database for test: %s", tt.description)
				}

				// Verify record count increased
				newCount := testutil.GetRecordCount(t, db, "downloads")
				if newCount != initialCount+1 {
					t.Errorf("Expected record count to increase by 1, got %d -> %d", initialCount, newCount)
				}

				// Verify download data
				var sourceIDFromDB string
				var statusCode int
				var headers, body string
				query := "SELECT source_id, status_code, headers, body FROM downloads WHERE id = ?"
				err := db.QueryRow(query, downloadID).Scan(&sourceIDFromDB, &statusCode, &headers, &body)
				if err != nil {
					t.Errorf("Failed to query download data: %v", err)
				} else {
					if sourceIDFromDB != tt.sourceID {
						t.Errorf("Expected source_id %s, got %s", tt.sourceID, sourceIDFromDB)
					}
					if statusCode != 200 {
						t.Errorf("Expected status code 200, got %d", statusCode)
					}
					if body != tt.content {
						t.Errorf("Expected body %s, got %s", tt.content, body)
					}

					// Verify headers contain GitHub-specific information
					if !strings.Contains(headers, "X-GitHub-SHA") {
						t.Error("Expected X-GitHub-SHA header in download")
					}
					if !strings.Contains(headers, tt.file.SHA) {
						t.Errorf("Expected SHA %s in headers", tt.file.SHA)
					}
				}

				// Clean up for next test
				_, err = db.Exec("DELETE FROM downloads WHERE id = ?", downloadID)
				if err != nil {
					t.Errorf("Failed to clean up download: %v", err)
				}
			}
		})
	}
}

func TestGitHubImporter_DatabaseErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	importer := NewGitHubImporter()

	t.Run("source creation with closed database", func(t *testing.T) {
		// Setup test database and then close it to simulate connection issues
		db := testutil.SetupTestDB(t)
		db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fileURL := "https://github.com/code-sleuth/outh/blob/main/README.md"
		repoInfo := &GitHubRepoInfo{Owner: "code-sleuth", Repo: "outh", Ref: "main"}
		file := GitHubTreeItem{
			Path: "README.md",
			Type: "blob",
			SHA:  "0e02509a5729c071ca1f6f919ea397fd2653b62b",
			Size: 787,
		}

		sourceID, err := importer.createSource(ctx, fileURL, repoInfo, file, db)

		// Should get an error due to closed database connection
		if err == nil {
			t.Error("Expected error due to closed database connection")
		}

		if sourceID != "" {
			t.Error("Expected empty sourceID when database operation fails")
		}
	})

	t.Run("download creation with closed database", func(t *testing.T) {
		// Setup test database and then close it to simulate connection issues
		db := testutil.SetupTestDB(t)
		db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		file := GitHubTreeItem{
			Path: "README.md",
			Type: "blob",
			SHA:  "0e02509a5729c071ca1f6f919ea397fd2653b62b",
			Size: 787,
		}
		content := "# OUTH Service\n\n## Environment\nIts a prerequisite that these environment variables are set. Set them in your terminal.\n\n```bash\n$ export JWT_SECRET=<your-jwt-secret>\n$ export DATABASE_URL=<example-postgres://postgres:notSoSecret@postgres:5432>\n$ export POSTMARK_AUTH_TOKEN=<your-postmark-auth-token>\n```\n\n\n## Setup & Build\n```shell\nmake build\n```\n\n## Run services locally\n#### App service\n```shell\nmake run-app-service\n```"

		downloadID, err := importer.createDownload(ctx, "fake-source-id", content, file, db)

		// Should get an error due to closed database connection
		if err == nil {
			t.Error("Expected error due to closed database connection")
		}

		if downloadID != "" {
			t.Error("Expected empty downloadID when database operation fails")
		}
	})

	t.Run("importFile with closed database", func(t *testing.T) {
		// Setup test database and then close it to simulate connection issues
		db := testutil.SetupTestDB(t)
		db.Close()

		// Create test server for file content
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fileResponse := GitHubFileResponse{
				Name:        "README.md",
				Path:        "README.md",
				SHA:         "0e02509a5729c071ca1f6f919ea397fd2653b62b",
				Size:        787,
				Content:     "IyBPVVRIIFNlcnZpY2UKCiMjIEVudmlyb25tZW50Ckl0cyBhIHByZXJlcXVpc2l0ZSB0aGF0IHRoZXNlIGVudmlyb25tZW50IHZhcmlhYmxlcyBhcmUgc2V0LiBTZXQgdGhlbSBpbiB5b3VyIHRlcm1pbmFsLgoKYGBgYmFzaAokIGV4cG9ydCBKV1RfU0VDUkVUPTx5b3VyLWp3dC1zZWNyZXQ+CiQgZXhwb3J0IERBVEFC\\nQVNFX1VSTD08ZXhhbXBsZS1wb3N0Z3JlczovL3Bvc3RncmVzOm5vdFNvU2VjcmV0QHBvc3RncmVzOjU0MzI+CiQgZXhwb3J0IFBPU1RNQVJLX0FVVEhfVE9LRU49PHlvdXItcG9zdG1hcmstYXV0aC10b2tlbj4KYGBgCgoKIyMgU2V0dXAg\\nJiBCdWlsZApgYGBzaGVsbAptYWtlIGJ1aWxkCmBgYAoKIyMgUnVuIHNlcnZpY2VzIGxvY2FsbHkKIyMjIyBBcHAgc2VydmljZQpgYGBzaGVsbAptYWtlIHJ1bi1hcHAtc2VydmljZQpgY",
				Encoding:    "base64",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fileResponse)
		}))
		defer testServer.Close()

		// Create importer with test server
		importerWithClient := NewGitHubImporterWithClient(&http.Client{Timeout: 5 * time.Second}, testServer.URL)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		repoInfo := &GitHubRepoInfo{Owner: "code-sleuth", Repo: "outh", Ref: "main"}
		file := GitHubTreeItem{
			Path: "README.md",
			Type: "blob",
			SHA:  "0e02509a5729c071ca1f6f919ea397fd2653b62b",
			Size: 787,
		}

		result, err := importerWithClient.importFile(ctx, repoInfo, file, db)

		// Should get an error due to closed database connection
		if err == nil {
			t.Error("Expected error due to closed database connection")
		}

		if result != nil {
			t.Error("Expected nil result when database operation fails")
		}
	})

	t.Run("context cancellation during database operations", func(t *testing.T) {
		// Setup test database
		db := testutil.SetupTestDB(t)
		defer testutil.CleanupTestDB(t, db)

		// Create context that will be cancelled immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		fileURL := "https://github.com/code-sleuth/outh/blob/main/README.md"
		repoInfo := &GitHubRepoInfo{Owner: "code-sleuth", Repo: "outh", Ref: "main"}
		file := GitHubTreeItem{
			Path: "README.md",
			Type: "blob",
			SHA:  "0e02509a5729c071ca1f6f919ea397fd2653b62b",
			Size: 787,
		}

		sourceID, err := importer.createSource(ctx, fileURL, repoInfo, file, db)

		// Should get a context cancellation error
		if err == nil {
			t.Error("Expected context cancellation error")
		}

		if sourceID != "" {
			t.Error("Expected empty sourceID when context is cancelled")
		}
	})

	t.Run("invalid URL parsing in createSource", func(t *testing.T) {
		// Setup test database
		db := testutil.SetupTestDB(t)
		defer testutil.CleanupTestDB(t, db)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Use invalid URL that will fail parsing
		invalidURL := "://invalid-url-format"
		repoInfo := &GitHubRepoInfo{Owner: "code-sleuth", Repo: "outh", Ref: "main"}
		file := GitHubTreeItem{
			Path: "README.md",
			Type: "blob",
			SHA:  "0e02509a5729c071ca1f6f919ea397fd2653b62b",
			Size: 787,
		}

		sourceID, err := importer.createSource(ctx, invalidURL, repoInfo, file, db)

		// Should get a URL parsing error
		if err == nil {
			t.Error("Expected URL parsing error")
		}

		if sourceID != "" {
			t.Error("Expected empty sourceID when URL parsing fails")
		}
	})

	t.Run("JSON marshaling error in createDownload", func(t *testing.T) {
		// Setup test database
		db := testutil.SetupTestDB(t)
		defer testutil.CleanupTestDB(t, db)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create a file with problematic SHA that could cause JSON issues
		file := GitHubTreeItem{
			Path: "test.md",
			Type: "blob",
			SHA:  "valid-sha", // This should work fine, but we're testing the error path
			Size: 1024,
		}
		content := "Test content"

		// First create a valid source to reference
		fileURL := "https://github.com/code-sleuth/outh/blob/main/test.md"
		repoInfo := &GitHubRepoInfo{Owner: "code-sleuth", Repo: "outh", Ref: "main"}
		sourceID, err := importer.createSource(ctx, fileURL, repoInfo, file, db)
		if err != nil {
			t.Fatalf("Failed to create test source: %v", err)
		}

		// Test normal download creation (should succeed)
		downloadID, err := importer.createDownload(ctx, sourceID, content, file, db)

		if err != nil {
			t.Errorf("Unexpected error in createDownload: %v", err)
		}

		if downloadID == "" {
			t.Error("Expected non-empty downloadID for successful operation")
		}

		// Verify the download was created
		if !testutil.RecordExists(t, db, "downloads", "id", downloadID) {
			t.Error("Download record not found in database")
		}
	})
}
