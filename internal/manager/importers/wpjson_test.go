package importers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewWPJSONImporter(t *testing.T) {
	importer := NewWPJSONImporter()

	if importer == nil {
		t.Fatal("Expected non-nil importer")
	}

	if importer.GetSourceType() != "wp-json" {
		t.Errorf("Expected source type 'wp-json', got %s", importer.GetSourceType())
	}
}

func TestWPJSONImporter_ValidateSource(t *testing.T) {
	importer := NewWPJSONImporter()

	tests := []struct {
		name        string
		sourceURL   string
		expectError bool
		expectedErr error
		description string
	}{
		{
			name:        "valid wp-json endpoint",
			sourceURL:   "https://example.com/wp-json/wp/v2/posts",
			expectError: false,
			expectedErr: nil,
			description: "should accept valid WordPress JSON API endpoint",
		},
		{
			name:        "valid wp-json with custom post type",
			sourceURL:   "https://blog.example.com/wp-json/wp/v2/custom_posts",
			expectError: false,
			expectedErr: nil,
			description: "should accept valid WordPress JSON API with custom post type",
		},
		{
			name:        "valid wp-json with subdirectory",
			sourceURL:   "https://example.com/blog/wp-json/wp/v2/posts",
			expectError: false,
			expectedErr: nil,
			description: "should accept WordPress JSON API in subdirectory",
		},
		{
			name:        "invalid URL - not wp-json",
			sourceURL:   "https://example.com/api/posts",
			expectError: true,
			expectedErr: ErrNotWordPressAPI,
			description: "should reject non-WordPress API endpoints",
		},
		{
			name:        "invalid URL - malformed",
			sourceURL:   "://invalid-url",
			expectError: true,
			expectedErr: nil, // URL parse error
			description: "should reject malformed URLs",
		},
		{
			name:        "invalid URL - missing wp-json",
			sourceURL:   "https://example.com/posts",
			expectError: true,
			expectedErr: ErrNotWordPressAPI,
			description: "should reject URLs without wp-json path",
		},
		{
			name:        "valid wp-json with query params",
			sourceURL:   "https://example.com/wp-json/wp/v2/posts?per_page=10",
			expectError: false,
			expectedErr: nil,
			description: "should accept WordPress JSON API with query parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := importer.ValidateSource(tt.sourceURL)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if tt.expectedErr != nil && err != tt.expectedErr {
				t.Errorf("Expected error %v, got %v for test: %s", tt.expectedErr, err, tt.description)
			}
		})
	}
}

func TestWPJSONImporter_GetPostIDs(t *testing.T) {
	// Create a test server that simulates WordPress JSON API
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		perPage := r.URL.Query().Get("per_page")

		// Simulate pagination
		switch page {
		case "1", "":
			// First page with posts
			posts := []map[string]interface{}{
				{"id": float64(1), "title": map[string]interface{}{"rendered": "Post 1"}},
				{"id": float64(2), "title": map[string]interface{}{"rendered": "Post 2"}},
				{"id": float64(3), "title": map[string]interface{}{"rendered": "Post 3"}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(posts)
		case "2":
			// Second page with fewer posts
			posts := []map[string]interface{}{
				{"id": float64(4), "title": map[string]interface{}{"rendered": "Post 4"}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(posts)
		case "3":
			// Third page - no more posts (WordPress returns 400)
			w.WriteHeader(http.StatusBadRequest)
			w.Write(
				[]byte(
					`{"code":"rest_post_invalid_page_number","message":"The page number requested is larger than the number of pages available."}`,
				),
			)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}

		// Check perPage parameter is passed correctly
		if perPage == "" {
			t.Errorf("Expected per_page parameter to be set")
		}
	}))
	defer testServer.Close()

	importer := NewWPJSONImporter()
	importer.SetPerPage(10) // Set smaller page size for testing

	tests := []struct {
		name        string
		baseURL     string
		expectError bool
		expectedIDs []int
		description string
	}{
		{
			name:        "successful pagination",
			baseURL:     testServer.URL + "/wp-json/wp/v2/posts",
			expectError: false,
			expectedIDs: []int{1, 2, 3, 4},
			description: "should fetch all post IDs across multiple pages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			postIDs, err := importer.getPostIDs(ctx, tt.baseURL)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if !tt.expectError {
				if len(postIDs) != len(tt.expectedIDs) {
					t.Errorf(
						"Expected %d post IDs, got %d for test: %s",
						len(tt.expectedIDs),
						len(postIDs),
						tt.description,
					)
					return
				}

				for i, expectedID := range tt.expectedIDs {
					if postIDs[i] != expectedID {
						t.Errorf(
							"Expected post ID %d at index %d, got %d for test: %s",
							expectedID,
							i,
							postIDs[i],
							tt.description,
						)
					}
				}
			}
		})
	}
}

func TestWPJSONImporter_GetPostIDs_ErrorHandling(t *testing.T) {
	importer := NewWPJSONImporter()

	t.Run("server error", func(t *testing.T) {
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer errorServer.Close()
		
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		baseURL := errorServer.URL + "/wp-json/wp/v2/posts"
		_, err := importer.getPostIDs(ctx, baseURL)
		
		// The implementation logs error but doesn't return it
		if err != nil {
			t.Errorf("Expected no error for server error (should handle gracefully), got: %v", err)
		}
	})
	
	t.Run("invalid JSON response", func(t *testing.T) {
		invalidJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer invalidJSONServer.Close()
		
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		baseURL := invalidJSONServer.URL + "/wp-json/wp/v2/posts"
		_, err := importer.getPostIDs(ctx, baseURL)
		
		if err == nil {
			t.Error("Expected error for invalid JSON response")
		}
	})
	
	t.Run("invalid URL", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		_, err := importer.getPostIDs(ctx, "://invalid-url")
		
		if err == nil {
			t.Error("Expected error for invalid URL")
		}
	})
}


func TestWPJSONImporter_ContextCancellation(t *testing.T) {
	// Test server that delays response
	delayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than our context timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer delayServer.Close()

	importer := NewWPJSONImporter()
	importer.SetTimeout(5 * time.Second) // Set longer timeout on client

	t.Run("context cancellation during getPostIDs", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		baseURL := delayServer.URL + "/wp-json/wp/v2/posts"
		_, err := importer.getPostIDs(ctx, baseURL)

		if err == nil {
			t.Error("Expected context cancellation error")
		}
	})
}

func TestWPJSONImporter_Configuration(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		importer := NewWPJSONImporter()
		
		// Test that importer starts with sensible defaults
		if importer.GetSourceType() != "wp-json" {
			t.Errorf("Expected source type 'wp-json', got %s", importer.GetSourceType())
		}
		
		// Test default values
		if importer.perPage != defaultPerPage {
			t.Errorf("Expected perPage %d, got %d", defaultPerPage, importer.perPage)
		}
		
		if importer.maxPages != maxPages {
			t.Errorf("Expected maxPages %d, got %d", maxPages, importer.maxPages)
		}
		
		if importer.concurrency != defaultConcurrency {
			t.Errorf("Expected concurrency %d, got %d", defaultConcurrency, importer.concurrency)
		}
		
		// Test HTTP client configuration
		if importer.client == nil {
			t.Error("Expected non-nil HTTP client")
		}
		
		if importer.client.Timeout <= 0 {
			t.Error("Expected positive timeout on HTTP client")
		}
	})
	
	t.Run("setter methods", func(t *testing.T) {
		importer := NewWPJSONImporter()
		
		// Test SetConcurrency
		importer.SetConcurrency(10)
		if importer.concurrency != 10 {
			t.Errorf("Expected concurrency 10, got %d", importer.concurrency)
		}
		
		// Test SetPerPage
		importer.SetPerPage(50)
		if importer.perPage != 50 {
			t.Errorf("Expected perPage 50, got %d", importer.perPage)
		}
		
		// Test SetMaxPages
		importer.SetMaxPages(500)
		if importer.maxPages != 500 {
			t.Errorf("Expected maxPages 500, got %d", importer.maxPages)
		}
		
		// Test SetTimeout
		originalTimeout := importer.client.Timeout
		newTimeout := 60 * time.Second
		importer.SetTimeout(newTimeout)
		
		if importer.client.Timeout != newTimeout {
			t.Errorf("Expected timeout %v, got %v", newTimeout, importer.client.Timeout)
		}
		
		if importer.client.Timeout == originalTimeout {
			t.Error("Timeout should have changed from original value")
		}
	})
}

// Benchmark tests
func BenchmarkWPJSONImporter_ValidateSource(b *testing.B) {
	importer := NewWPJSONImporter()
	sourceURL := "https://example.com/wp-json/wp/v2/posts"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		importer.ValidateSource(sourceURL)
	}
}

func BenchmarkNewWPJSONImporter(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewWPJSONImporter()
	}
}
