package embedders

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/testutil"
)

func TestNewTogetherAIEmbedder(t *testing.T) {
	// Save original env var
	originalAPIKey := os.Getenv("TOGETHER_API_KEY")
	defer os.Setenv("TOGETHER_API_KEY", originalAPIKey)

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
			name:        "valid m2-bert-80M-8k-retrieval",
			model:       "togethercomputer/m2-bert-80M-8k-retrieval",
			apiKey:      "test-api-key",
			expectError: false,
			expectedDim: 768,
			expectedMax: 8192,
			description: "should create embedder for m2-bert-80M-8k-retrieval",
		},
		{
			name:        "valid m2-bert-80M-32k-retrieval",
			model:       "togethercomputer/m2-bert-80M-32k-retrieval",
			apiKey:      "test-api-key",
			expectError: false,
			expectedDim: 768,
			expectedMax: 32768,
			description: "should create embedder for m2-bert-80M-32k-retrieval",
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
			model:       "togethercomputer/m2-bert-80M-8k-retrieval",
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
		{
			name:        "non-togethercomputer model",
			model:       "some-other/model",
			apiKey:      "test-api-key",
			expectError: true,
			expectedDim: 0,
			expectedMax: 0,
			description: "should return error for non-togethercomputer model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			os.Setenv("TOGETHER_API_KEY", tt.apiKey)

			embedder, err := NewTogetherAIEmbedder(tt.model)

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

func TestTogetherAIEmbedder_GenerateEmbedding(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		t.Skip("TOGETHER_API_KEY not set, skipping real API tests")
	}

	embedder, err := NewTogetherAIEmbedder("togethercomputer/m2-bert-80M-8k-retrieval")
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
			name:        "scientific content",
			content:     "The quadratic formula is x = (-b ± √(b²-4ac)) / 2a where a ≠ 0.",
			expectError: false,
			description: "should handle scientific content with special symbols",
		},
		{
			name:        "code content",
			content:     "def fibonacci(n):\n    if n <= 1:\n        return n\n    return fibonacci(n-1) + fibonacci(n-2)",
			expectError: false,
			description: "should handle code content",
		},
		{
			name:        "multilingual content",
			content:     "Hello, world! Bonjour le monde! Hola mundo! 你好世界! こんにちは世界!",
			expectError: false,
			description: "should handle multilingual content",
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

func TestTogetherAIEmbedder_GenerateEmbedding_ErrorHandling(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		t.Skip("TOGETHER_API_KEY not set, skipping real API tests")
	}

	// Test error handling with invalid API key
	t.Run("invalid API key", func(t *testing.T) {
		// Temporarily set invalid API key
		originalAPIKey := os.Getenv("TOGETHER_API_KEY")
		os.Setenv("TOGETHER_API_KEY", "invalid-key-12345")
		defer os.Setenv("TOGETHER_API_KEY", originalAPIKey)

		embedder, err := NewTogetherAIEmbedder("togethercomputer/m2-bert-80M-8k-retrieval")
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
		embedder, err := NewTogetherAIEmbedder("togethercomputer/m2-bert-80M-8k-retrieval")
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

func TestTogetherAIEmbedder_ModelCompatibility(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		t.Skip("TOGETHER_API_KEY not set, skipping real API tests")
	}

	tests := []struct {
		name              string
		model             string
		expectedDimension int
		expectedMaxTokens int
		expectError       bool
		description       string
	}{
		{
			name:              "8k model",
			model:             "togethercomputer/m2-bert-80M-8k-retrieval",
			expectedDimension: 768,
			expectedMaxTokens: 8192,
			expectError:       false,
			description:       "should work with 8k context model",
		},
		{
			name:              "32k model",
			model:             "togethercomputer/m2-bert-80M-32k-retrieval",
			expectedDimension: 768,
			expectedMaxTokens: 32768,
			expectError:       false,
			description:       "should work with 32k context model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embedder, err := NewTogetherAIEmbedder(tt.model)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if tt.expectError {
				return
			}

			if embedder.GetDimension() != tt.expectedDimension {
				t.Errorf("Expected dimension %d, got %d for test: %s",
					tt.expectedDimension, embedder.GetDimension(), tt.description)
			}

			if embedder.GetMaxTokens() != tt.expectedMaxTokens {
				t.Errorf("Expected max tokens %d, got %d for test: %s",
					tt.expectedMaxTokens, embedder.GetMaxTokens(), tt.description)
			}

			if embedder.GetModelName() != tt.model {
				t.Errorf("Expected model name %s, got %s for test: %s",
					tt.model, embedder.GetModelName(), tt.description)
			}
		})
	}
}

func TestTogetherAIEmbedder_ContentCleaning(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		t.Skip("TOGETHER_API_KEY not set, skipping real API tests")
	}

	embedder, err := NewTogetherAIEmbedder("togethercomputer/m2-bert-80M-8k-retrieval")
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
			name:        "content with newlines",
			content:     "Line 1\nLine 2\nLine 3",
			expectError: false,
			description: "should handle content with newlines",
		},
		{
			name:        "content with multiple spaces",
			content:     "Word1    Word2     Word3",
			expectError: false,
			description: "should handle content with multiple spaces",
		},
		{
			name:        "content with tabs",
			content:     "Word1\tWord2\tWord3",
			expectError: false,
			description: "should handle content with tabs",
		},
		{
			name:        "content with mixed whitespace",
			content:     "  \n\t  Word1  \n  Word2  \t\n  ",
			expectError: false,
			description: "should handle content with mixed whitespace",
		},
		{
			name:        "content with carriage returns",
			content:     "Line1\r\nLine2\r\nLine3",
			expectError: false,
			description: "should handle content with carriage returns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Use real API to test content cleaning

			_, err := embedder.GenerateEmbedding(ctx, tt.content)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
		})
	}
}

// Test helper functions
func TestTogetherAIEmbedder_ModelProperties(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		t.Skip("TOGETHER_API_KEY not set, skipping real API tests")
	}

	models := map[string]struct {
		dimension int
		maxTokens int
	}{
		"togethercomputer/m2-bert-80M-8k-retrieval":  {768, 8192},
		"togethercomputer/m2-bert-80M-32k-retrieval": {768, 32768},
	}

	for model, expected := range models {
		t.Run(model, func(t *testing.T) {
			embedder, err := NewTogetherAIEmbedder(model)
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
func BenchmarkNewTogetherAIEmbedder(b *testing.B) {
	// Check if we have a real API key, skip if not
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		b.Skip("TOGETHER_API_KEY not set, skipping real API tests")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewTogetherAIEmbedder("togethercomputer/m2-bert-80M-8k-retrieval")
		if err != nil {
			b.Fatalf("Error creating embedder: %v", err)
		}
	}
}
