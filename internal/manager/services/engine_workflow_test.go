package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/internal/manager/testutil"
)

// Test ProcessSource workflow
func TestProcessingEngine_ProcessSource(t *testing.T) {
	tests := []struct {
		name        string
		sourceURL   string
		setup       func(engine *ProcessingEngine)
		options     *interfaces.ProcessingOptions
		expectError bool
		expectedErr error
		description string
	}{
		{
			name:      "successful import step",
			sourceURL: "https://github.com/owner/repo",
			setup: func(engine *ProcessingEngine) {
				// Register successful importer
				importer := &mockImporter{
					sourceType: "github",
					importResult: &interfaces.ImportResult{
						SourceID:   "source-123",
						DownloadID: "download-123",
					},
					validateError: nil,
				}
				engine.RegisterImporter(importer)
			},
			options: &interfaces.ProcessingOptions{
				MaxTokens:      1000,
				ChunkStrategy:  "token",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    2,
				Timeout:        30 * time.Second,
			},
			expectError: true, // Will fail at ProcessDocument due to missing download record
			description: "should complete import step but fail at document processing",
		},
		{
			name:      "no importer for source type",
			sourceURL: "https://unsupported.com/content",
			setup: func(engine *ProcessingEngine) {
				// Register importer that doesn't support this URL
				importer := &mockImporter{
					sourceType:    "github",
					validateError: errors.New("unsupported URL"),
				}
				engine.RegisterImporter(importer)
			},
			options: &interfaces.ProcessingOptions{
				MaxTokens:      1000,
				ChunkStrategy:  "token",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    2,
			},
			expectError: true,
			expectedErr: ErrNoImporterCanHandle,
			description: "should fail when no importer can handle URL",
		},
		{
			name:      "import failure",
			sourceURL: "https://github.com/owner/repo",
			setup: func(engine *ProcessingEngine) {
				// Register importer that fails during import
				importer := &mockImporter{
					sourceType:    "github",
					importError:   errors.New("import failed"),
					validateError: nil,
				}
				engine.RegisterImporter(importer)
			},
			options: &interfaces.ProcessingOptions{
				MaxTokens:      1000,
				ChunkStrategy:  "token",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    2,
			},
			expectError: true,
			description: "should fail when import fails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test database
			testDB := testutil.SetupTestDB(t)
			defer testutil.CleanupTestDB(t, testDB)

			engine := NewProcessingEngine()
			tt.setup(engine)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := engine.ProcessSource(ctx, tt.sourceURL, tt.options, testDB)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if tt.expectedErr != nil && err != tt.expectedErr {
				t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

// Test ProcessDocument workflow with mocked database
func TestProcessingEngine_ProcessDocument_Logic(t *testing.T) {
	tests := []struct {
		name        string
		downloadID  string
		setup       func(engine *ProcessingEngine)
		options     *interfaces.ProcessingOptions
		expectError bool
		expectedErr error
		description string
	}{
		{
			name:       "no transformer registered",
			downloadID: "download-123",
			setup: func(engine *ProcessingEngine) {
				// Don't register any transformers
			},
			options: &interfaces.ProcessingOptions{
				ChunkStrategy:  "token",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    2,
			},
			expectError: true,
			expectedErr: ErrNoTransformerRegistered,
			description: "should fail when no transformer is registered",
		},
		{
			name:       "no chunker registered",
			downloadID: "download-123",
			setup: func(engine *ProcessingEngine) {
				// Register transformer but no chunker
				transformer := &mockTransformer{
					sourceType: "github",
					transformResult: &interfaces.TransformResult{
						Document: &models.Document{ID: "doc-123"},
						Content:  "test content",
					},
				}
				engine.RegisterTransformer(transformer)
			},
			options: &interfaces.ProcessingOptions{
				ChunkStrategy:  "token",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    2,
			},
			expectError: true,
			expectedErr: ErrNoChunkerRegistered,
			description: "should fail when no chunker is registered",
		},
		{
			name:       "no embedder registered",
			downloadID: "download-123",
			setup: func(engine *ProcessingEngine) {
				// Register transformer and chunker but no embedder
				transformer := &mockTransformer{
					sourceType: "github",
					transformResult: &interfaces.TransformResult{
						Document: &models.Document{ID: "doc-123"},
						Content:  "test content",
					},
				}
				engine.RegisterTransformer(transformer)

				chunker := &mockChunker{
					strategy: "token",
					chunks: []*models.Chunk{
						{Body: stringPtr("chunk 1")},
					},
				}
				engine.RegisterChunker(chunker)
			},
			options: &interfaces.ProcessingOptions{
				ChunkStrategy:  "token",
				EmbeddingModel: "text-embedding-ada-002",
				Concurrency:    2,
			},
			expectError: true,
			expectedErr: ErrNoEmbedderRegistered,
			description: "should fail when no embedder is registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test database
			testDB := testutil.SetupTestDB(t)
			defer testutil.CleanupTestDB(t, testDB)

			// Create test data for the specific download ID
			setupTestDownload(t, testDB, tt.downloadID)

			engine := NewProcessingEngine()
			tt.setup(engine)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := engine.ProcessDocument(ctx, tt.downloadID, tt.options, testDB)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if tt.expectedErr != nil && err != tt.expectedErr {
				t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

// Test chunk processing worker logic
func TestProcessingEngine_chunkWorker(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() (*mockEmbedder, []*models.Chunk)
		expectError bool
		description string
	}{
		{
			name: "successful chunk processing",
			setup: func() (*mockEmbedder, []*models.Chunk) {
				embedder := &mockEmbedder{
					modelName: "text-embedding-ada-002",
					dimension: 1536,
					embedding: make([]float32, 1536),
				}
				chunks := []*models.Chunk{
					{Body: stringPtr("test content")},
				}
				return embedder, chunks
			},
			expectError: false,
			description: "should process chunks successfully",
		},
		{
			name: "embedding generation failure",
			setup: func() (*mockEmbedder, []*models.Chunk) {
				embedder := &mockEmbedder{
					modelName:  "text-embedding-ada-002",
					dimension:  1536,
					embedError: errors.New("embedding failed"),
				}
				chunks := []*models.Chunk{
					{Body: stringPtr("test content")},
				}
				return embedder, chunks
			},
			expectError: true,
			description: "should handle embedding generation failure",
		},
		{
			name: "unsupported embedding dimension",
			setup: func() (*mockEmbedder, []*models.Chunk) {
				embedder := &mockEmbedder{
					modelName: "unsupported-model",
					dimension: 999, // Unsupported dimension
					embedding: make([]float32, 999),
				}
				chunks := []*models.Chunk{
					{Body: stringPtr("test content")},
				}
				return embedder, chunks
			},
			expectError: true,
			description: "should handle unsupported embedding dimensions",
		},
		{
			name: "nil chunk body",
			setup: func() (*mockEmbedder, []*models.Chunk) {
				embedder := &mockEmbedder{
					modelName: "text-embedding-ada-002",
					dimension: 1536,
					embedding: make([]float32, 1536),
				}
				chunks := []*models.Chunk{
					{Body: nil}, // Nil body
				}
				return embedder, chunks
			},
			expectError: false,
			description: "should handle nil chunk body gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test database
			testDB := testutil.SetupTestDB(t)
			defer testutil.CleanupTestDB(t, testDB)

			// Create required parent records for foreign key constraints
			setupTestDocument(t, testDB, "doc-123")

			engine := NewProcessingEngine()
			embedder, chunks := tt.setup()

			// Create channels for worker communication
			chunkChan := make(chan *models.Chunk, len(chunks))
			resultChan := make(chan *interfaces.ChunkResult, len(chunks))

			// Send chunks to worker
			for _, chunk := range chunks {
				chunkChan <- chunk
			}
			close(chunkChan)

			// Run worker in goroutine
			go engine.chunkWorker(context.Background(), chunkChan, resultChan, "doc-123", embedder, testDB)

			// Collect results
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
		})
	}
}

// Test processChunks orchestration
func TestProcessingEngine_processChunks(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() ([]*models.Chunk, *mockEmbedder)
		concurrency int
		expectError bool
		description string
	}{
		{
			name: "successful processing with single worker",
			setup: func() ([]*models.Chunk, *mockEmbedder) {
				chunks := []*models.Chunk{
					{Body: stringPtr("chunk 1")},
					{Body: stringPtr("chunk 2")},
				}
				embedder := &mockEmbedder{
					modelName: "text-embedding-ada-002",
					dimension: 1536,
					embedding: make([]float32, 1536),
				}
				return chunks, embedder
			},
			concurrency: 1,
			expectError: false,
			description: "should process chunks with single worker",
		},
		{
			name: "successful processing with multiple workers",
			setup: func() ([]*models.Chunk, *mockEmbedder) {
				chunks := []*models.Chunk{
					{Body: stringPtr("chunk 1")},
					{Body: stringPtr("chunk 2")},
					{Body: stringPtr("chunk 3")},
					{Body: stringPtr("chunk 4")},
				}
				embedder := &mockEmbedder{
					modelName: "text-embedding-ada-002",
					dimension: 1536,
					embedding: make([]float32, 1536),
				}
				return chunks, embedder
			},
			concurrency: 3,
			expectError: false,
			description: "should process chunks with multiple workers",
		},
		{
			name: "processing with errors",
			setup: func() ([]*models.Chunk, *mockEmbedder) {
				chunks := []*models.Chunk{
					{Body: stringPtr("chunk 1")},
					{Body: stringPtr("chunk 2")},
				}
				embedder := &mockEmbedder{
					modelName:  "text-embedding-ada-002",
					dimension:  1536,
					embedError: errors.New("embedding failed"),
				}
				return chunks, embedder
			},
			concurrency: 2,
			expectError: true,
			description: "should handle processing errors",
		},
		{
			name: "empty chunks list",
			setup: func() ([]*models.Chunk, *mockEmbedder) {
				chunks := []*models.Chunk{}
				embedder := &mockEmbedder{
					modelName: "text-embedding-ada-002",
					dimension: 1536,
					embedding: make([]float32, 1536),
				}
				return chunks, embedder
			},
			concurrency: 2,
			expectError: false,
			description: "should handle empty chunks list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test database
			testDB := testutil.SetupTestDB(t)
			defer testutil.CleanupTestDB(t, testDB)

			// Create required parent records for foreign key constraints
			setupTestDocument(t, testDB, "doc-123")

			engine := NewProcessingEngine()
			chunks, embedder := tt.setup()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := engine.processChunks(ctx, chunks, "doc-123", embedder, testDB, tt.concurrency)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
		})
	}
}

// Test error types and constants
func TestProcessingEngine_ErrorTypes(t *testing.T) {
	errorTests := []struct {
		name  string
		error error
	}{
		{"ErrImporterAlreadyRegistered", ErrImporterAlreadyRegistered},
		{"ErrTransformerAlreadyRegistered", ErrTransformerAlreadyRegistered},
		{"ErrChunkerAlreadyRegistered", ErrChunkerAlreadyRegistered},
		{"ErrEmbedderAlreadyRegistered", ErrEmbedderAlreadyRegistered},
		{"ErrUpdaterAlreadyRegistered", ErrUpdaterAlreadyRegistered},
		{"ErrNoImporterRegistered", ErrNoImporterRegistered},
		{"ErrNoTransformerRegistered", ErrNoTransformerRegistered},
		{"ErrNoChunkerRegistered", ErrNoChunkerRegistered},
		{"ErrNoEmbedderRegistered", ErrNoEmbedderRegistered},
		{"ErrNoImporterCanHandle", ErrNoImporterCanHandle},
		{"ErrCannotDetermineSourceType", ErrCannotDetermineSourceType},
		{"ErrChunkProcessingFailed", ErrChunkProcessingFailed},
		{"ErrUnsupportedEmbeddingDim", ErrUnsupportedEmbeddingDim},
		{"ErrNoEmbeddingVector", ErrNoEmbeddingVector},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.error == nil {
				t.Errorf("Expected non-nil error for %s", tt.name)
			}
			if tt.error.Error() == "" {
				t.Errorf("Expected non-empty error message for %s", tt.name)
			}
		})
	}
}

// Test embedding dimension constants
func TestProcessingEngine_EmbeddingDimensions(t *testing.T) {
	dimensionTests := []struct {
		name      string
		dimension int
		expected  int
	}{
		{"embeddingDim768", embeddingDim768, 768},
		{"embeddingDim1536", embeddingDim1536, 1536},
		{"embeddingDim3072", embeddingDim3072, 3072},
	}

	for _, tt := range dimensionTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dimension != tt.expected {
				t.Errorf("Expected dimension %d, got %d", tt.expected, tt.dimension)
			}
		})
	}
}

// setupTestDocument creates the required parent records for foreign key constraints
func setupTestDocument(t *testing.T, db *sql.DB, documentID string) {
	t.Helper()
	
	// Clean up any existing data first
	cleanupTestData(t, db)
	
	// Fix embeddings table schema if needed
	_, err := db.Exec(`
		DROP TABLE IF EXISTS embeddings;
		CREATE TABLE embeddings (
			id TEXT NOT NULL PRIMARY KEY,
			embedding_1536 TEXT,
			embedding_3072 TEXT,
			embedding_768 TEXT,
			model TEXT,
			embedded_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
			object_id TEXT NOT NULL,
			object_type TEXT NOT NULL DEFAULT 'chunk',
			FOREIGN KEY (object_id) REFERENCES chunks(id)
		);
	`)
	if err != nil {
		t.Fatalf("Failed to fix embeddings table schema: %v", err)
	}
	
	// Create source record
	sourceID := "test-source-123"
	_, err = db.Exec(`
		INSERT INTO sources (id, active_domain, created_at, updated_at) 
		VALUES (?, 1, datetime('now'), datetime('now'))
	`, sourceID)
	if err != nil {
		t.Fatalf("Failed to create test source: %v", err)
	}
	
	// Create download record
	downloadID := "test-download-123"
	_, err = db.Exec(`
		INSERT INTO downloads (id, source_id, headers) 
		VALUES (?, ?, '{}')
	`, downloadID, sourceID)
	if err != nil {
		t.Fatalf("Failed to create test download: %v", err)
	}
	
	// Create document record
	_, err = db.Exec(`
		INSERT INTO documents (id, source_id, download_id, min_chunk_size, max_chunk_size) 
		VALUES (?, ?, ?, 100, 1000)
	`, documentID, sourceID, downloadID)
	if err != nil {
		t.Fatalf("Failed to create test document: %v", err)
	}
}

// setupTestDownload creates test data for a specific download ID
func setupTestDownload(t *testing.T, db *sql.DB, downloadID string) {
	t.Helper()
	
	// Clean up any existing data first
	cleanupTestData(t, db)
	
	// Fix embeddings table schema if needed
	_, err := db.Exec(`
		DROP TABLE IF EXISTS embeddings;
		CREATE TABLE embeddings (
			id TEXT NOT NULL PRIMARY KEY,
			embedding_1536 TEXT,
			embedding_3072 TEXT,
			embedding_768 TEXT,
			model TEXT,
			embedded_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
			object_id TEXT NOT NULL,
			object_type TEXT NOT NULL DEFAULT 'chunk',
			FOREIGN KEY (object_id) REFERENCES chunks(id)
		);
	`)
	if err != nil {
		t.Fatalf("Failed to fix embeddings table schema: %v", err)
	}
	
	// Create source record
	sourceID := "test-source-456"
	_, err = db.Exec(`
		INSERT INTO sources (id, active_domain, host, created_at, updated_at) 
		VALUES (?, 1, 'github.com', datetime('now'), datetime('now'))
	`, sourceID)
	if err != nil {
		t.Fatalf("Failed to create test source: %v", err)
	}
	
	// Create the specific download record the test expects
	_, err = db.Exec(`
		INSERT INTO downloads (id, source_id, headers, body) 
		VALUES (?, ?, '{}', '{"test": "data"}')
	`, downloadID, sourceID)
	if err != nil {
		t.Fatalf("Failed to create test download: %v", err)
	}
}

// cleanupTestData removes test data to prevent foreign key conflicts
func cleanupTestData(t *testing.T, db *sql.DB) {
	t.Helper()
	
	// Clean up in reverse order of dependencies
	tables := []string{
		"embeddings",
		"chunks", 
		"documents",
		"downloads",
		"sources",
	}
	
	for _, table := range tables {
		_, err := db.Exec("DELETE FROM " + table + " WHERE id LIKE 'test-%' OR id LIKE '%123%'")
		if err != nil {
			t.Logf("Warning: Failed to clean table %s: %v", table, err)
		}
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
