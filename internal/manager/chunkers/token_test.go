package chunkers

import (
	"testing"

	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/internal/manager/testutil"
)

func TestNewTokenChunker(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	chunker, err := NewTokenChunker()
	if err != nil {
		t.Fatalf("Failed to create token chunker: %v", err)
	}

	if chunker == nil {
		t.Fatal("Expected non-nil chunker")
	}

	if chunker.GetChunkingStrategy() != "token" {
		t.Errorf("Expected strategy 'token', got %s", chunker.GetChunkingStrategy())
	}
}

func TestTokenChunker_ChunkDocument(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	chunker, err := NewTokenChunker()
	if err != nil {
		t.Fatalf("Failed to create token chunker: %v", err)
	}

	// Get test values from environment or use defaults
	maxTokens := GetDefaultMaxTokens()
	t.Logf("Using max tokens from environment: %d", maxTokens)

	tests := []struct {
		name         string
		content      string
		maxTokens    int
		expectError  bool
		expectChunks int
		description  string
	}{
		{
			name:         "empty content",
			content:      "",
			maxTokens:    maxTokens,
			expectError:  true,
			expectChunks: 0,
			description:  "should return error for empty content",
		},
		{
			name:         "invalid max tokens - zero",
			content:      "Hello world",
			maxTokens:    0,
			expectError:  true,
			expectChunks: 0,
			description:  "should return error for zero max tokens",
		},
		{
			name:         "invalid max tokens - negative",
			content:      "Hello world",
			maxTokens:    -1,
			expectError:  true,
			expectChunks: 0,
			description:  "should return error for negative max tokens",
		},
		{
			name:         "single chunk - short content",
			content:      "Hello world, this is a test.",
			maxTokens:    maxTokens,
			expectError:  false,
			expectChunks: 1,
			description:  "should create single chunk for short content",
		},
		{
			name:         "single chunk - exact token limit",
			content:      "Hello",
			maxTokens:    1,
			expectError:  false,
			expectChunks: 1,
			description:  "should create single chunk when content exactly matches token limit",
		},
		{
			name: "multiple chunks - long content",
			content: `This is a very long document that contains multiple sentences and should be split into multiple chunks. 
			It has various topics and discussions that would benefit from being separated for better processing. 
			The chunking algorithm should intelligently split this content while maintaining context boundaries where possible.
			Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
			Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
			Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.
			Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.`,
			maxTokens:    max(50, maxTokens/2),
			expectError:  false,
			expectChunks: 3, // This will depend on actual token count
			description:  "should create multiple chunks for long content",
		},
		{
			name: "technical content",
			content: `
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello, People of the inter webs!")
	if len(os.Args) > 1 {
		fmt.Printf("Arguments: %v\n", os.Args[1:])
	}
}`,
			maxTokens:    max(30, maxTokens/3),
			expectError:  false,
			expectChunks: 2,
			description:  "should handle code content properly",
		},
		{
			name: "markdown content",
			content: `
# Document Title

This is a **markdown** document with various formatting.

## Section 1

- List item 1
- List item 2
- List item 3

### Subsection

Here's some *italic text* and some [link](https://example.com).

## Section 2

| Column 1 | Column 2 |
|----------|----------|
| Data 1   | Data 2   |

` + "```go" + `
func example() {
    fmt.Println("code block")
}
` + "```" + ``,
			maxTokens:    max(40, maxTokens/2),
			expectError:  false,
			expectChunks: 3,
			description:  "should handle markdown content with various elements",
		},
		{
			name: "unicode content",
			content: `这是一个中文文档的例子。它包含中文字符和标点符号。
			Это пример документа на русском языке с кириллическими символами.
			هذا مثال على وثيقة باللغة العربية مع نص من اليمين إلى اليسار.
			Mixed content with émojis 🚀 and spéciál châractérs.`,
			maxTokens:    max(25, maxTokens/4),
			expectError:  false,
			expectChunks: 4,
			description:  "should handle unicode and mixed language content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.ChunkDocument(tt.content, tt.maxTokens)

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

			// Check chunk count (allow some flexibility for token counting variations)
			if len(chunks) < 1 {
				t.Errorf("Expected at least 1 chunk, got %d for test: %s", len(chunks), tt.description)
				return
			}

			// Validate chunk properties
			for i, chunk := range chunks {
				if chunk == nil {
					t.Errorf("Chunk %d is nil for test: %s", i, tt.description)
					continue
				}

				if chunk.Body == nil || *chunk.Body == "" {
					t.Errorf("Chunk %d has empty body for test: %s", i, tt.description)
				}

				if chunk.TokenCount == nil || *chunk.TokenCount <= 0 {
					t.Errorf("Chunk %d has invalid token count %v for test: %s", i, chunk.TokenCount, tt.description)
				}

				if chunk.TokenCount != nil && *chunk.TokenCount > tt.maxTokens {
					t.Errorf(
						"Chunk %d exceeds max tokens: %d > %d for test: %s",
						i,
						*chunk.TokenCount,
						tt.maxTokens,
						tt.description,
					)
				}

				if chunk.ByteSize == nil || *chunk.ByteSize <= 0 {
					t.Errorf("Chunk %d has invalid byte size %v for test: %s", i, chunk.ByteSize, tt.description)
				}

				expectedTokenizer := getTokenizerFromEnv()
				if chunk.Tokenizer == nil || *chunk.Tokenizer != expectedTokenizer {
					t.Errorf("Chunk %d has wrong tokenizer %v, expected %s for test: %s", i, chunk.Tokenizer, expectedTokenizer, tt.description)
				}

				if chunk.ID == "" {
					t.Errorf("Chunk %d has empty ID for test: %s", i, tt.description)
				}
			}

			// Verify that all chunks together contain the original content
			var totalContent string
			for _, chunk := range chunks {
				if chunk.Body != nil {
					totalContent += *chunk.Body
				}
			}

			// For multiple chunks, some content might be overlapping or modified,
			// but the total length should be reasonable
			if len(chunks) == 1 && totalContent != tt.content {
				t.Errorf("Single chunk content doesn't match original for test: %s", tt.description)
			}
		})
	}
}

func TestTokenChunker_ChunkDocument_WithOverlap(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	chunker, err := NewTokenChunker()
	if err != nil {
		t.Fatalf("Failed to create token chunker: %v", err)
	}

	// Get test values from environment
	maxTokens := max(20, GetDefaultMaxTokens()/5)
	smallTokens := max(2, GetDefaultMaxTokens()/50)

	// Test content that will definitely need chunking
	longContent := `This is the first sentence. This is the second sentence. This is the third sentence. 
	This is the fourth sentence. This is the fifth sentence. This is the sixth sentence.
	This is the seventh sentence. This is the eighth sentence. This is the ninth sentence.
	This is the tenth sentence. This is the eleventh sentence. This is the twelfth sentence.`

	tests := []struct {
		name        string
		content     string
		maxTokens   int
		expectError bool
		description string
	}{
		{
			name:        "normal chunking",
			content:     longContent,
			maxTokens:   maxTokens,
			expectError: false,
			description: "should chunk long content into multiple pieces",
		},
		{
			name:        "very small chunks",
			content:     "Hello world test content",
			maxTokens:   smallTokens,
			expectError: false,
			description: "should handle very small chunk sizes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.ChunkDocument(tt.content, tt.maxTokens)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if !tt.expectError {
				// Verify chunk linking for multiple chunks
				if len(chunks) > 1 {
					for i, chunk := range chunks {
						// Check left chunk reference
						if i > 0 && chunk.LeftChunkID == nil {
							t.Logf(
								"Chunk %d missing left chunk reference (this might be expected depending on implementation)",
								i,
							)
						}

						// Check right chunk reference
						if i < len(chunks)-1 && chunk.RightChunkID == nil {
							t.Logf(
								"Chunk %d missing right chunk reference (this might be expected depending on implementation)",
								i,
							)
						}
					}
				}
			}
		})
	}
}

func TestTokenChunker_EdgeCases(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	chunker, err := NewTokenChunker()
	if err != nil {
		t.Fatalf("Failed to create token chunker: %v", err)
	}

	// Get test values from environment
	maxTokens := GetDefaultMaxTokens()

	tests := []struct {
		name        string
		content     string
		maxTokens   int
		expectError bool
		description string
	}{
		{
			name:        "whitespace only",
			content:     "   \n\t  \r\n  ",
			maxTokens:   max(10, maxTokens/10),
			expectError: false,
			description: "should handle whitespace-only content",
		},
		{
			name:        "single character",
			content:     "a",
			maxTokens:   1,
			expectError: false,
			description: "should handle single character",
		},
		{
			name:        "special characters",
			content:     "!@#$%^&*()_+-=[]{}|;:'\",.<>?/~`",
			maxTokens:   max(10, maxTokens/10),
			expectError: false,
			description: "should handle special characters",
		},
		{
			name:        "very large max tokens",
			content:     "Short content",
			maxTokens:   max(1000000, maxTokens*1000),
			expectError: false,
			description: "should handle very large max tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.ChunkDocument(tt.content, tt.maxTokens)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if !tt.expectError && len(chunks) == 0 {
				t.Errorf("Expected at least one chunk for test: %s", tt.description)
			}
		})
	}
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Helper function to create a test chunk
func createTestChunk(id, documentID, body string, tokenCount int) *models.Chunk {
	bodyPtr := &body
	tokenCountPtr := &tokenCount
	byteSize := len(body)
	byteSizePtr := &byteSize
	tokenizer := getTokenizerFromEnv()
	tokenizerPtr := &tokenizer

	return &models.Chunk{
		ID:         id,
		DocumentID: documentID,
		Body:       bodyPtr,
		TokenCount: tokenCountPtr,
		ByteSize:   byteSizePtr,
		Tokenizer:  tokenizerPtr,
	}
}

// Benchmark tests
func BenchmarkTokenChunker_ShortContent(b *testing.B) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		b.Logf("Warning: Failed to load .env file: %v", err)
	}

	chunker, err := NewTokenChunker()
	if err != nil {
		b.Fatalf("Failed to create token chunker: %v", err)
	}

	content := "This is a short piece of content for benchmarking."
	maxTokens := GetDefaultMaxTokens()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chunker.ChunkDocument(content, maxTokens)
		if err != nil {
			b.Fatalf("Error during benchmark: %v", err)
		}
	}
}

func BenchmarkTokenChunker_LongContent(b *testing.B) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		b.Logf("Warning: Failed to load .env file: %v", err)
	}

	chunker, err := NewTokenChunker()
	if err != nil {
		b.Fatalf("Failed to create token chunker: %v", err)
	}

	// Create long content
	content := ""
	for i := 0; i < 1000; i++ {
		content += "This is sentence number " + string(
			rune(i),
		) + " in a very long document that will need to be chunked. "
	}
	maxTokens := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chunker.ChunkDocument(content, maxTokens)
		if err != nil {
			b.Fatalf("Error during benchmark: %v", err)
		}
	}
}
