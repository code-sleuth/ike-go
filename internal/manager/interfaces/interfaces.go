package interfaces

import (
	"context"
	"database/sql"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/models"
)

// ImportResult represents the result of an import operation.
type ImportResult struct {
	SourceID   string
	DownloadID string
	Error      error
}

// TransformResult represents the result of a transformation operation.
type TransformResult struct {
	Document *models.Document
	Content  string
	Language string
	Metadata map[string]interface{}
	Error    error
}

// ChunkResult represents a single chunk with its embedding.
type ChunkResult struct {
	Chunk     *models.Chunk
	Embedding *models.Embedding
	Error     error
}

// Importer defines the interface for importing content from external sources.
type Importer interface {
	// Import fetches content from a source and creates download records
	Import(ctx context.Context, sourceURL string, db *sql.DB) (*ImportResult, error)

	// GetSourceType returns the type of source this importer handles
	GetSourceType() string

	// ValidateSource checks if the source URL is valid for this importer
	ValidateSource(sourceURL string) error
}

// Transformer defines the interface for transforming downloads into documents.
type Transformer interface {
	// Transform converts a download into a structured document
	Transform(ctx context.Context, download *models.Download, db *sql.DB) (*TransformResult, error)

	// GetSourceType returns the type of source this transformer handles
	GetSourceType() string

	// CanTransform checks if this transformer can handle the given download
	CanTransform(download *models.Download) bool
}

// Chunker defines the interface for breaking documents into chunks.
type Chunker interface {
	// ChunkDocument splits a document into manageable chunks
	ChunkDocument(content string, maxTokens int) ([]*models.Chunk, error)

	// GetChunkingStrategy returns the strategy name used by this chunker
	GetChunkingStrategy() string
}

// Embedder defines the interface for generating vector embeddings.
type Embedder interface {
	// GenerateEmbedding creates a vector embedding for the given content
	GenerateEmbedding(ctx context.Context, content string) ([]float32, error)

	// GetModelName returns the name of the embedding model
	GetModelName() string

	// GetDimension returns the dimension of the embedding vectors
	GetDimension() int

	// GetMaxTokens returns the maximum number of tokens this embedder can handle
	GetMaxTokens() int
}

// UpdateResult represents the result of an update operation.
type UpdateResult struct {
	SourceID     string
	Updated      bool
	NewItems     int
	UpdatedItems int
	Error        error
}

// Updater defines the interface for detecting and handling content changes.
type Updater interface {
	// CheckForUpdates scans sources for new or changed content
	CheckForUpdates(ctx context.Context, db *sql.DB) ([]*UpdateResult, error)

	// UpdateSource processes updates for a specific source
	UpdateSource(ctx context.Context, sourceID string, db *sql.DB) (*UpdateResult, error)

	// GetSourceType returns the type of source this updater handles
	GetSourceType() string
}

// ProcessingOptions contains configuration for processing pipelines.
type ProcessingOptions struct {
	MaxTokens      int
	ChunkStrategy  string
	EmbeddingModel string
	Concurrency    int
	Timeout        time.Duration
}

// ProcessingEngine orchestrates the complete import/transform/chunk/embed pipeline.
type ProcessingEngine interface {
	// ProcessSource runs the complete pipeline for a source
	ProcessSource(ctx context.Context, sourceURL string, options *ProcessingOptions, db *sql.DB) error

	// ProcessDocument runs transform/chunk/embed for an existing download
	ProcessDocument(ctx context.Context, downloadID string, options *ProcessingOptions, db *sql.DB) error

	// RegisterImporter adds a new importer to the engine
	RegisterImporter(importer Importer) error

	// RegisterTransformer adds a new transformer to the engine
	RegisterTransformer(transformer Transformer) error

	// RegisterChunker adds a new chunker to the engine
	RegisterChunker(chunker Chunker) error

	// RegisterEmbedder adds a new embedder to the engine
	RegisterEmbedder(embedder Embedder) error

	// RegisterUpdater adds a new updater to the engine
	RegisterUpdater(updater Updater) error
}
