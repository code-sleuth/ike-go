package embedders

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/testutil"
)

func TestNewOpenAIEmbedder(t *testing.T) {
	// Save original env var
	originalAPIKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalAPIKey)

	tests := []struct {
		name        string
		model       string
		apiKey      string
		expectError bool
		expectedDim int
		expectedMax int
		description string
	}{
		{
			name:        "valid text-embedding-3-small",
			model:       "text-embedding-3-small",
			apiKey:      "test-api-key",
			expectError: false,
			expectedDim: 1536,
			expectedMax: 8191,
			description: "should create embedder for text-embedding-3-small",
		},
		{
			name:        "valid text-embedding-3-large",
			model:       "text-embedding-3-large",
			apiKey:      "test-api-key",
			expectError: false,
			expectedDim: 3072,
			expectedMax: 8191,
			description: "should create embedder for text-embedding-3-large",
		},
		{
			name:        "valid text-embedding-ada-002",
			model:       "text-embedding-ada-002",
			apiKey:      "test-api-key",
			expectError: false,
			expectedDim: 1536,
			expectedMax: 8191,
			description: "should create embedder for text-embedding-ada-002",
		},
		{
			name:        "unsupported model",
			model:       "unsupported-model",
			apiKey:      "test-api-key",
			expectError: true,
			expectedDim: 0,
			expectedMax: 0,
			description: "should return error for unsupported model",
		},
		{
			name:        "missing api key",
			model:       "text-embedding-3-small",
			apiKey:      "",
			expectError: true,
			expectedDim: 0,
			expectedMax: 0,
			description: "should return error when API key is missing",
		},
		{
			name:        "empty model",
			model:       "",
			apiKey:      "test-api-key",
			expectError: true,
			expectedDim: 0,
			expectedMax: 0,
			description: "should return error for empty model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			os.Setenv("OPENAI_API_KEY", tt.apiKey)

			embedder, err := NewOpenAIEmbedder(tt.model)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			// If we expected an error, we're done
			if tt.expectError {
				return
			}

			// Validate embedder properties
			if embedder == nil {
				t.Errorf("Expected non-nil embedder for test: %s", tt.description)
				return
			}

			if embedder.GetModelName() != tt.model {
				t.Errorf("Expected model %s, got %s for test: %s", tt.model, embedder.GetModelName(), tt.description)
			}

			if embedder.GetDimension() != tt.expectedDim {
				t.Errorf(
					"Expected dimension %d, got %d for test: %s",
					tt.expectedDim,
					embedder.GetDimension(),
					tt.description,
				)
			}

			if embedder.GetMaxTokens() != tt.expectedMax {
				t.Errorf(
					"Expected max tokens %d, got %d for test: %s",
					tt.expectedMax,
					embedder.GetMaxTokens(),
					tt.description,
				)
			}
		})
	}
}

func TestOpenAIEmbedder_GenerateEmbedding(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping real API tests")
	}

	embedder, err := NewOpenAIEmbedder("text-embedding-3-small")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	tests := []struct {
		name        string
		content     string
		expectError bool
		description string
	}{
		{
			name:        "valid content",
			content:     "This is a test document for embedding generation.",
			expectError: false,
			description: "should generate embedding for valid content",
		},
		{
			name:        "empty content",
			content:     "",
			expectError: true,
			description: "should return error for empty content",
		},
		{
			name:        "whitespace only content",
			content:     "   \n\t  ",
			expectError: false,
			description: "should handle whitespace content",
		},
		{
			name:        "long content",
			content:     strings.Repeat("This is a long document. ", 100),
			expectError: false,
			description: "should handle long content",
		},
		{
			name:        "unicode content",
			content:     "这是一个测试文档 🚀 with émojis and spéciál characters.",
			expectError: false,
			description: "should handle unicode content",
		},
		{
			name:        "code content",
			content:     "func main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			expectError: false,
			description: "should handle code content",
		},
		{
			name:        "markdown content",
			content:     "# Title\n\nThis is **bold** and *italic* text with [links](https://example.com).",
			expectError: false,
			description: "should handle markdown content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// All tests use the real API now

			embedding, err := embedder.GenerateEmbedding(ctx, tt.content)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			// If we expected an error, we're done
			if tt.expectError {
				return
			}

			// Validate embedding
			if len(embedding) != embedder.GetDimension() {
				t.Errorf("Expected embedding dimension %d, got %d for test: %s",
					embedder.GetDimension(), len(embedding), tt.description)
			}

			// Check that embedding contains non-zero values
			hasNonZero := false
			for _, val := range embedding {
				if val != 0 {
					hasNonZero = true
					break
				}
			}
			if !hasNonZero {
				t.Errorf("Embedding contains all zero values for test: %s", tt.description)
			}
		})
	}
}

func TestOpenAIEmbedder_GenerateEmbedding_ErrorHandling(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping real API tests")
	}

	// Test error handling with invalid API key
	t.Run("invalid API key", func(t *testing.T) {
		// Temporarily set invalid API key
		originalAPIKey := os.Getenv("OPENAI_API_KEY")
		os.Setenv("OPENAI_API_KEY", "invalid-key-12345")
		defer os.Setenv("OPENAI_API_KEY", originalAPIKey)

		embedder, err := NewOpenAIEmbedder("text-embedding-3-small")
		if err != nil {
			t.Fatalf("Failed to create embedder: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = embedder.GenerateEmbedding(ctx, "test content")
		if err == nil {
			t.Error("Expected error with invalid API key but got none")
		}
	})

	// Test timeout handling with very short timeout
	t.Run("request timeout", func(t *testing.T) {
		embedder, err := NewOpenAIEmbedder("text-embedding-3-small")
		if err != nil {
			t.Fatalf("Failed to create embedder: %v", err)
		}

		// Use extremely short timeout to force timeout error
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		_, err = embedder.GenerateEmbedding(ctx, "test content")
		if err == nil {
			t.Error("Expected timeout error but got none")
		}
	})
}

func TestOpenAIEmbedder_ContextCancellation(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping real API tests")
	}

	embedder, err := NewOpenAIEmbedder("text-embedding-3-small")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = embedder.GenerateEmbedding(ctx, "test content")
	if err == nil {
		t.Error("Expected error due to cancelled context but got none")
	}
}

func TestOpenAIEmbedder_ConcurrentRequests(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping real API tests")
	}

	embedder, err := NewOpenAIEmbedder("text-embedding-3-small")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	// Test concurrent requests with real API (reduce number to avoid rate limits)
	const numRequests = 3
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			content := fmt.Sprintf("Test content for concurrent request %d", i)
			_, err := embedder.GenerateEmbedding(ctx, content)
			results <- err
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent request %d failed: %v", i, err)
		}
	}
}

// Test helper functions
func TestOpenAIEmbedder_ModelProperties(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping real API tests")
	}

	models := map[string]struct {
		dimension int
		maxTokens int
	}{
		"text-embedding-3-small": {1536, 8191},
		"text-embedding-3-large": {3072, 8191},
		"text-embedding-ada-002": {1536, 8191},
	}

	for model, expected := range models {
		t.Run(model, func(t *testing.T) {
			embedder, err := NewOpenAIEmbedder(model)
			if err != nil {
				t.Fatalf("Failed to create embedder for %s: %v", model, err)
			}

			if embedder.GetModelName() != model {
				t.Errorf("Expected model name %s, got %s", model, embedder.GetModelName())
			}

			if embedder.GetDimension() != expected.dimension {
				t.Errorf("Expected dimension %d for %s, got %d", expected.dimension, model, embedder.GetDimension())
			}

			if embedder.GetMaxTokens() != expected.maxTokens {
				t.Errorf("Expected max tokens %d for %s, got %d", expected.maxTokens, model, embedder.GetMaxTokens())
			}
		})
	}
}

// Benchmark tests
func BenchmarkNewOpenAIEmbedder(b *testing.B) {
	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		b.Skip("OPENAI_API_KEY not set, skipping real API tests")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewOpenAIEmbedder("text-embedding-3-small")
		if err != nil {
			b.Fatalf("Error creating embedder: %v", err)
		}
	}
}
