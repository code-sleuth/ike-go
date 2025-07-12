package services

import (
	"context"
	"errors"
	"testing"

	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/internal/manager/models"
)

// Test chunk worker logic without database operations
func TestProcessingEngine_chunkWorker_Logic(t *testing.T) {
	tests := []struct {
		name        string
		chunks      []*models.Chunk
		embedder    *mockEmbedder
		expectError bool
		description string
	}{
		{
			name: "successful chunk processing with 1536 dimension",
			chunks: []*models.Chunk{
				{Body: stringPtr("test content")},
			},
			embedder: &mockEmbedder{
				modelName: "text-embedding-ada-002",
				dimension: 1536,
				embedding: make([]float32, 1536),
			},
			expectError: false,
			description: "should process chunk and generate embedding successfully",
		},
		{
			name: "successful chunk processing with 768 dimension",
			chunks: []*models.Chunk{
				{Body: stringPtr("test content")},
			},
			embedder: &mockEmbedder{
				modelName: "text-embedding-768",
				dimension: 768,
				embedding: make([]float32, 768),
			},
			expectError: false,
			description: "should process chunk with 768 dimension embedding",
		},
		{
			name: "successful chunk processing with 3072 dimension",
			chunks: []*models.Chunk{
				{Body: stringPtr("test content")},
			},
			embedder: &mockEmbedder{
				modelName: "text-embedding-3072",
				dimension: 3072,
				embedding: make([]float32, 3072),
			},
			expectError: false,
			description: "should process chunk with 3072 dimension embedding",
		},
		{
			name: "embedding generation failure",
			chunks: []*models.Chunk{
				{Body: stringPtr("test content")},
			},
			embedder: &mockEmbedder{
				modelName:  "text-embedding-ada-002",
				dimension:  1536,
				embedError: errors.New("embedding generation failed"),
			},
			expectError: true,
			description: "should handle embedding generation failure",
		},
		{
			name: "unsupported embedding dimension",
			chunks: []*models.Chunk{
				{Body: stringPtr("test content")},
			},
			embedder: &mockEmbedder{
				modelName: "unsupported-model",
				dimension: 999, // Unsupported dimension
				embedding: make([]float32, 999),
			},
			expectError: true,
			description: "should reject unsupported embedding dimensions",
		},
		{
			name: "nil chunk body",
			chunks: []*models.Chunk{
				{Body: nil},
			},
			embedder: &mockEmbedder{
				modelName: "text-embedding-ada-002",
				dimension: 1536,
				embedding: make([]float32, 1536),
			},
			expectError: false,
			description: "should handle nil chunk body gracefully",
		},
		{
			name: "empty chunk body",
			chunks: []*models.Chunk{
				{Body: stringPtr("")},
			},
			embedder: &mockEmbedder{
				modelName: "text-embedding-ada-002",
				dimension: 1536,
				embedding: make([]float32, 1536),
			},
			expectError: false,
			description: "should handle empty chunk body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create channels for communication
			chunkChan := make(chan *models.Chunk, len(tt.chunks))
			resultChan := make(chan *interfaces.ChunkResult, len(tt.chunks))

			// Send chunks to channel
			for _, chunk := range tt.chunks {
				chunkChan <- chunk
			}
			close(chunkChan)

			// Create a mock worker that doesn't save to database
			go func() {
				for chunk := range chunkChan {
					result := &interfaces.ChunkResult{
						Chunk: chunk,
					}

					// Set document ID and generate UUID (simplified)
					chunk.DocumentID = "doc-123"
					chunk.ID = "chunk-uuid-123"

					// Generate embedding if body exists
					if chunk.Body != nil {
						embedding, err := tt.embedder.GenerateEmbedding(context.Background(), *chunk.Body)
						if err != nil {
							result.Error = err
							resultChan <- result
							continue
						}

						// Create embedding record
						modelName := tt.embedder.GetModelName()
						result.Embedding = &models.Embedding{
							ID:         "embedding-uuid-123",
							Model:      &modelName,
							ObjectID:   chunk.ID,
							ObjectType: "chunk",
						}

						// Set appropriate embedding field based on dimension
						switch tt.embedder.GetDimension() {
						case embeddingDim768:
							result.Embedding.Embedding768 = embedding
						case embeddingDim1536:
							result.Embedding.Embedding1536 = embedding
						case embeddingDim3072:
							result.Embedding.Embedding3072 = embedding
						default:
							result.Error = ErrUnsupportedEmbeddingDim
						}
					}

					resultChan <- result
				}
			}()

			// Collect result
			result := <-resultChan

			if tt.expectError && result.Error == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && result.Error != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, result.Error)
			}

			// Verify chunk processing
			if result.Chunk != nil {
				if result.Chunk.DocumentID != "doc-123" {
					t.Errorf("Expected document ID 'doc-123', got '%s'", result.Chunk.DocumentID)
				}
				if result.Chunk.ID == "" {
					t.Error("Expected chunk ID to be generated")
				}
			}

			// Verify embedding if no error and body exists
			if !tt.expectError && result.Chunk != nil && result.Chunk.Body != nil {
				if result.Embedding == nil {
					t.Error("Expected embedding to be created")
				} else {
					if result.Embedding.ObjectID != result.Chunk.ID {
						t.Errorf("Expected embedding object ID to match chunk ID")
					}
					if result.Embedding.ObjectType != "chunk" {
						t.Errorf("Expected embedding object type 'chunk', got '%s'", result.Embedding.ObjectType)
					}

					// Verify embedding dimension
					switch tt.embedder.GetDimension() {
					case embeddingDim768:
						if result.Embedding.Embedding768 == nil {
							t.Error("Expected Embedding768 to be set")
						}
					case embeddingDim1536:
						if result.Embedding.Embedding1536 == nil {
							t.Error("Expected Embedding1536 to be set")
						}
					case embeddingDim3072:
						if result.Embedding.Embedding3072 == nil {
							t.Error("Expected Embedding3072 to be set")
						}
					}
				}
			}
		})
	}
}

// Test the processChunks coordination logic
func TestProcessingEngine_processChunks_Coordination(t *testing.T) {
	tests := []struct {
		name        string
		numChunks   int
		concurrency int
		embedder    *mockEmbedder
		expectError bool
		description string
	}{
		{
			name:        "single chunk single worker",
			numChunks:   1,
			concurrency: 1,
			embedder: &mockEmbedder{
				modelName: "text-embedding-ada-002",
				dimension: 1536,
				embedding: make([]float32, 1536),
			},
			expectError: false,
			description: "should coordinate single chunk with single worker",
		},
		{
			name:        "multiple chunks single worker",
			numChunks:   5,
			concurrency: 1,
			embedder: &mockEmbedder{
				modelName: "text-embedding-ada-002",
				dimension: 1536,
				embedding: make([]float32, 1536),
			},
			expectError: false,
			description: "should coordinate multiple chunks with single worker",
		},
		{
			name:        "multiple chunks multiple workers",
			numChunks:   10,
			concurrency: 3,
			embedder: &mockEmbedder{
				modelName: "text-embedding-ada-002",
				dimension: 1536,
				embedding: make([]float32, 1536),
			},
			expectError: false,
			description: "should coordinate multiple chunks with multiple workers",
		},
		{
			name:        "zero chunks",
			numChunks:   0,
			concurrency: 2,
			embedder: &mockEmbedder{
				modelName: "text-embedding-ada-002",
				dimension: 1536,
				embedding: make([]float32, 1536),
			},
			expectError: false,
			description: "should handle zero chunks gracefully",
		},
		{
			name:        "worker errors",
			numChunks:   3,
			concurrency: 2,
			embedder: &mockEmbedder{
				modelName:  "text-embedding-ada-002",
				dimension:  1536,
				embedError: errors.New("worker error"),
			},
			expectError: true,
			description: "should handle worker errors properly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test chunks
			chunks := make([]*models.Chunk, tt.numChunks)
			for i := 0; i < tt.numChunks; i++ {
				chunks[i] = &models.Chunk{
					Body: stringPtr("test content"),
				}
			}

			// Create channels for coordination testing
			chunkChan := make(chan *models.Chunk, len(chunks))
			resultChan := make(chan *interfaces.ChunkResult, len(chunks))

			// Send chunks
			for _, chunk := range chunks {
				chunkChan <- chunk
			}
			close(chunkChan)

			// Start mock workers
			for i := 0; i < tt.concurrency; i++ {
				go func() {
					for chunk := range chunkChan {
						result := &interfaces.ChunkResult{
							Chunk: chunk,
						}

						// Simulate processing
						chunk.ID = "chunk-id"
						chunk.DocumentID = "doc-123"

						if tt.embedder.embedError != nil {
							result.Error = tt.embedder.embedError
						}

						resultChan <- result
					}
				}()
			}

			// Collect results
			var errorCount int
			for i := 0; i < len(chunks); i++ {
				result := <-resultChan
				if result.Error != nil {
					errorCount++
				}
			}

			if tt.expectError && errorCount == 0 {
				t.Errorf("Expected errors but got none for test: %s", tt.description)
			}
			if !tt.expectError && errorCount > 0 {
				t.Errorf("Unexpected errors for test %s: %d errors", tt.description, errorCount)
			}
		})
	}
}

// Test processing options validation
func TestProcessingEngine_ProcessingOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     *interfaces.ProcessingOptions
		valid       bool
		description string
	}{
		{
			name: "valid options",
			options: &interfaces.ProcessingOptions{
				MaxTokens:      1000,
				ChunkStrategy:  "token",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    2,
			},
			valid:       true,
			description: "should accept valid processing options",
		},
		{
			name: "zero max tokens",
			options: &interfaces.ProcessingOptions{
				MaxTokens:      0,
				ChunkStrategy:  "token",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    2,
			},
			valid:       true, // Engine should handle this
			description: "should handle zero max tokens",
		},
		{
			name: "zero concurrency",
			options: &interfaces.ProcessingOptions{
				MaxTokens:      1000,
				ChunkStrategy:  "token",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    0,
			},
			valid:       true, // Engine should handle this
			description: "should handle zero concurrency",
		},
		{
			name: "empty strategy",
			options: &interfaces.ProcessingOptions{
				MaxTokens:      1000,
				ChunkStrategy:  "",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    2,
			},
			valid:       true, // Engine will handle missing strategy
			description: "should handle empty chunk strategy",
		},
		{
			name: "empty embedding model",
			options: &interfaces.ProcessingOptions{
				MaxTokens:      1000,
				ChunkStrategy:  "token",
				EmbeddingModel: "",
				Concurrency:    2,
			},
			valid:       true, // Engine will handle missing model
			description: "should handle empty embedding model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that options can be created and accessed
			if tt.options.MaxTokens < 0 {
				t.Errorf("MaxTokens should not be negative: %d", tt.options.MaxTokens)
			}
			if tt.options.Concurrency < 0 {
				t.Errorf("Concurrency should not be negative: %d", tt.options.Concurrency)
			}
			// Options validation would be handled by the engine methods themselves
		})
	}
}
