package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const (
	// Embedding dimensions.
	embeddingDim768  = 768
	embeddingDim1536 = 1536
	embeddingDim3072 = 3072
)

var (
	// Registration errors.
	ErrImporterAlreadyRegistered    = errors.New("importer already registered for source type")
	ErrTransformerAlreadyRegistered = errors.New("transformer already registered for source type")
	ErrChunkerAlreadyRegistered     = errors.New("chunker already registered for strategy")
	ErrEmbedderAlreadyRegistered    = errors.New("embedder already registered for model")
	ErrUpdaterAlreadyRegistered     = errors.New("updater already registered for source type")

	// Processing errors.
	ErrNoImporterRegistered      = errors.New("no importer registered for source type")
	ErrNoTransformerRegistered   = errors.New("no transformer registered for source type")
	ErrNoChunkerRegistered       = errors.New("no chunker registered for strategy")
	ErrNoEmbedderRegistered      = errors.New("no embedder registered for model")
	ErrNoImporterCanHandle       = errors.New("no importer can handle URL")
	ErrCannotDetermineSourceType = errors.New("cannot determine source type from source")
	ErrChunkProcessingFailed     = errors.New("chunk processing failed")
	ErrUnsupportedEmbeddingDim   = errors.New("unsupported embedding dimension")
	ErrNoEmbeddingVector         = errors.New("no embedding vector found")
)

// ProcessingEngine implements the main processing pipeline.
type ProcessingEngine struct {
	importers    map[string]interfaces.Importer
	transformers map[string]interfaces.Transformer
	chunkers     map[string]interfaces.Chunker
	embedders    map[string]interfaces.Embedder
	updaters     map[string]interfaces.Updater
	logger       zerolog.Logger
	mu           sync.RWMutex
}

// NewProcessingEngine creates a new processing engine.
func NewProcessingEngine() *ProcessingEngine {
	return &ProcessingEngine{
		importers:    make(map[string]interfaces.Importer),
		transformers: make(map[string]interfaces.Transformer),
		chunkers:     make(map[string]interfaces.Chunker),
		embedders:    make(map[string]interfaces.Embedder),
		updaters:     make(map[string]interfaces.Updater),
		logger:       util.NewLogger(zerolog.ErrorLevel),
	}
}

// RegisterImporter adds a new importer to the engine.
func (e *ProcessingEngine) RegisterImporter(importer interfaces.Importer) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var err error

	sourceType := importer.GetSourceType()
	if _, exists := e.importers[sourceType]; exists {
		e.logger.Error().Str("source_type", sourceType).Msg("Importer already registered")
		err = ErrImporterAlreadyRegistered
		return err
	}

	e.importers[sourceType] = importer
	e.logger.Info().Str("source_type", sourceType).Msg("Registered importer")
	return err
}

// RegisterTransformer adds a new transformer to the engine.
func (e *ProcessingEngine) RegisterTransformer(transformer interfaces.Transformer) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var err error

	sourceType := transformer.GetSourceType()
	if _, exists := e.transformers[sourceType]; exists {
		e.logger.Error().Str("source_type", sourceType).Msg("Transformer already registered")
		err = ErrTransformerAlreadyRegistered
		return err
	}

	e.transformers[sourceType] = transformer
	e.logger.Info().Str("source_type", sourceType).Msg("Registered transformer")
	return err
}

// RegisterChunker adds a new chunker to the engine.
func (e *ProcessingEngine) RegisterChunker(chunker interfaces.Chunker) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var err error

	strategy := chunker.GetChunkingStrategy()
	if _, exists := e.chunkers[strategy]; exists {
		e.logger.Error().Str("strategy", strategy).Msg("Chunker already registered")
		err = ErrChunkerAlreadyRegistered
		return err
	}

	e.chunkers[strategy] = chunker
	e.logger.Info().Str("strategy", strategy).Msg("Registered chunker")
	return err
}

// RegisterEmbedder adds a new embedder to the engine.
func (e *ProcessingEngine) RegisterEmbedder(embedder interfaces.Embedder) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var err error

	modelName := embedder.GetModelName()
	if _, exists := e.embedders[modelName]; exists {
		e.logger.Error().Str("model_name", modelName).Msg("Embedder already registered")
		err = ErrEmbedderAlreadyRegistered
		return err
	}

	e.embedders[modelName] = embedder
	e.logger.Info().Str("model_name", modelName).Msg("Registered embedder")
	return err
}

// RegisterUpdater adds a new updater to the engine.
func (e *ProcessingEngine) RegisterUpdater(updater interfaces.Updater) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var err error

	sourceType := updater.GetSourceType()
	if _, exists := e.updaters[sourceType]; exists {
		e.logger.Error().Str("source_type", sourceType).Msg("Updater already registered")
		err = ErrUpdaterAlreadyRegistered
		return err
	}

	e.updaters[sourceType] = updater
	e.logger.Info().Str("source_type", sourceType).Msg("Registered updater")
	return err
}

// ProcessSource runs the complete pipeline for a source.
func (e *ProcessingEngine) ProcessSource(
	ctx context.Context,
	sourceURL string,
	options *interfaces.ProcessingOptions,
	db *sql.DB,
) error {
	// Determine source type from URL
	sourceType, err := e.determineSourceType(sourceURL)
	if err != nil {
		e.logger.Error().Err(err).Str("source_url", sourceURL).Msg("Failed to determine source type")
		return err
	}

	// Get the appropriate importer
	e.mu.RLock()
	importer, exists := e.importers[sourceType]
	e.mu.RUnlock()

	if !exists {
		e.logger.Error().Str("source_url", sourceURL).Msgf("No importer registered for source type: %s", sourceType)
		return ErrNoImporterRegistered
	}

	// Import the content
	e.logger.Info().Str("source_url", sourceURL).Str("source_type", sourceType).Msg("Starting import")
	importResult, err := importer.Import(ctx, sourceURL, db)
	if err != nil {
		e.logger.Error().Err(err).Str("source_url", sourceURL).Msg("Import failed")
		return err
	}

	// Process the imported content
	return e.ProcessDocument(ctx, importResult.DownloadID, options, db)
}

// ProcessDocument runs transform/chunk/embed for an existing download.
func (e *ProcessingEngine) ProcessDocument(
	ctx context.Context,
	downloadID string,
	options *interfaces.ProcessingOptions,
	db *sql.DB,
) error {
	// Get the download
	download, err := e.getDownload(ctx, downloadID, db)
	if err != nil {
		e.logger.Error().Err(err).Str("download_id", downloadID).Msg("Failed to get download")
		return err
	}

	// Get the source to determine type
	source, err := e.getSource(ctx, download.SourceID, db)
	if err != nil {
		e.logger.Error().Err(err).Str("download_id", downloadID).Msg("Failed to get source")
		return err
	}

	// Determine source type from source
	sourceType, err := e.determineSourceTypeFromSource(source)
	if err != nil {
		e.logger.Error().Err(err).Str("download_id", downloadID).Msg("Failed to determine source type")
		return err
	}

	// Get the appropriate transformer
	e.mu.RLock()
	transformer, exists := e.transformers[sourceType]
	e.mu.RUnlock()

	if !exists {
		e.logger.Error().
			Str("download_id", downloadID).
			Msgf("No transformer registered for source type: %s", sourceType)
		return ErrNoTransformerRegistered
	}

	// Transform the content
	e.logger.Info().Str("download_id", downloadID).Str("source_type", sourceType).Msg("Starting transformation")
	transformResult, err := transformer.Transform(ctx, download, db)
	if err != nil {
		e.logger.Error().Err(err).Str("download_id", downloadID).Msg("Transformation failed")
		return err
	}

	// Get the chunker
	e.mu.RLock()
	chunker, exists := e.chunkers[options.ChunkStrategy]
	e.mu.RUnlock()

	if !exists {
		e.logger.Error().
			Str("download_id", downloadID).
			Msgf("No chunker registered for strategy: %s", options.ChunkStrategy)
		return ErrNoChunkerRegistered
	}

	// Get the embedder
	e.mu.RLock()
	embedder, exists := e.embedders[options.EmbeddingModel]
	e.mu.RUnlock()

	if !exists {
		e.logger.Error().
			Str("download_id", downloadID).
			Msgf("No embedder registered for model: %s", options.EmbeddingModel)
		return ErrNoEmbedderRegistered
	}

	// Chunk the content
	e.logger.Info().
		Str("document_id", transformResult.Document.ID).
		Str("chunk_strategy", options.ChunkStrategy).
		Int("max_tokens", options.MaxTokens).
		Msg("Starting chunking")
	chunks, err := chunker.ChunkDocument(transformResult.Content, options.MaxTokens)
	if err != nil {
		e.logger.Error().Err(err).Str("document_id", transformResult.Document.ID).Msg("Chunking failed")
		return err
	}

	// Process chunks concurrently
	e.logger.Info().
		Int("chunk_count", len(chunks)).
		Str("embedding_model", options.EmbeddingModel).
		Int("concurrency", options.Concurrency).
		Msg("Starting embedding")
	return e.processChunks(ctx, chunks, transformResult.Document.ID, embedder, db, options.Concurrency)
}

// Helper methods

func (e *ProcessingEngine) determineSourceType(sourceURL string) (string, error) {
	// Check each importer to see if it can handle this URL
	e.mu.RLock()
	defer e.mu.RUnlock()

	for sourceType, importer := range e.importers {
		if err := importer.ValidateSource(sourceURL); err == nil {
			return sourceType, nil
		}
	}

	e.logger.Error().Str("source_url", sourceURL).Msg("No importer can handle this source")
	return "", ErrNoImporterCanHandle
}

func (e *ProcessingEngine) determineSourceTypeFromSource(source *models.Source) (string, error) {
	// For now, we'll use a simple heuristic based on the host
	// TODO: This could be extended to use a more sophisticated detection system
	if source.Host != nil {
		host := *source.Host
		if host == "github.com" || host == "api.github.com" {
			return "github", nil
		}
		// Default to wp-json for other hosts
		return "wp-json", nil
	}

	var sourceURL string
	if source.RawURL != nil {
		sourceURL = *source.RawURL
	}

	e.logger.Error().Str("source_url", sourceURL).Msg("Failed to determine source type from source")
	return "", ErrCannotDetermineSourceType
}

func (e *ProcessingEngine) getDownload(ctx context.Context, downloadID string, db *sql.DB) (*models.Download, error) {
	query := `SELECT id, source_id, attempted_at, downloaded_at, status_code, headers, body 
			 FROM downloads WHERE id = ?`

	row := db.QueryRowContext(ctx, query, downloadID)

	var download models.Download
	var attemptedAt, downloadedAt sql.NullString
	var statusCode sql.NullInt32
	var body sql.NullString

	err := row.Scan(&download.ID, &download.SourceID, &attemptedAt, &downloadedAt,
		&statusCode, &download.Headers, &body)
	if err != nil {
		e.logger.Error().Err(err).Str("download_id", downloadID).Msg("Failed to get download")
		return nil, err
	}

	// Handle nullable fields
	if attemptedAt.Valid {
		if t, err := time.Parse(time.RFC3339, attemptedAt.String); err == nil {
			e.logger.Debug().Str("download_id", downloadID).Str("attempted_at", attemptedAt.String).Msg("Attempted at")
			download.AttemptedAt = &t
		}
	}
	if downloadedAt.Valid {
		if t, err := time.Parse(time.RFC3339, downloadedAt.String); err == nil {
			e.logger.Debug().
				Str("download_id", downloadID).
				Str("downloaded_at", downloadedAt.String).
				Msg("Downloaded at")
			download.DownloadedAt = &t
		}
	}
	if statusCode.Valid {
		code := int(statusCode.Int32)
		download.StatusCode = &code
	}
	if body.Valid {
		download.Body = &body.String
	}

	return &download, nil
}

func (e *ProcessingEngine) getSource(ctx context.Context, sourceID string, db *sql.DB) (*models.Source, error) {
	query := `SELECT id, author_email, raw_url, scheme, host, path, query, active_domain, 
			 format, created_at, updated_at 
			 FROM sources WHERE id = ?`

	row := db.QueryRowContext(ctx, query, sourceID)

	var source models.Source
	var authorEmail, rawURL, scheme, host, path, queryParam, format sql.NullString
	var createdAtStr, updatedAtStr string

	err := row.Scan(&source.ID, &authorEmail, &rawURL, &scheme, &host, &path,
		&queryParam, &source.ActiveDomain, &format, &createdAtStr, &updatedAtStr)
	if err != nil {
		e.logger.Error().Err(err).Str("source_id", sourceID).Msg("Failed to get source")
		return nil, err
	}

	// Handle nullable fields
	if authorEmail.Valid {
		source.AuthorEmail = &authorEmail.String
	}
	if rawURL.Valid {
		source.RawURL = &rawURL.String
	}
	if scheme.Valid {
		source.Scheme = &scheme.String
	}
	if host.Valid {
		source.Host = &host.String
	}
	if path.Valid {
		source.Path = &path.String
	}
	if queryParam.Valid {
		source.Query = &queryParam.String
	}
	if format.Valid {
		source.Format = &format.String
	}

	// Parse timestamps
	if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		source.CreatedAt = createdAt
	}
	if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
		source.UpdatedAt = updatedAt
	}

	return &source, nil
}

func (e *ProcessingEngine) processChunks(
	ctx context.Context,
	chunks []*models.Chunk,
	documentID string,
	embedder interfaces.Embedder,
	db *sql.DB,
	concurrency int,
) error {
	// Channel for chunk processing
	chunkChan := make(chan *models.Chunk, len(chunks))
	resultChan := make(chan *interfaces.ChunkResult, len(chunks))

	// Start workers
	for i := 0; i < concurrency; i++ {
		go e.chunkWorker(ctx, chunkChan, resultChan, documentID, embedder, db)
	}

	// Send chunks to workers
	for _, chunk := range chunks {
		chunkChan <- chunk
	}
	close(chunkChan)

	// Collect results
	var errorsList []error
	for i := 0; i < len(chunks); i++ {
		result := <-resultChan
		if result.Error != nil {
			errorsList = append(errorsList, result.Error)
		}
	}

	var err error
	if len(errorsList) > 0 {
		e.logger.Error().Errs("errors", errorsList).Msg("Chunk processing failed")
		err = ErrChunkProcessingFailed
		return err
	}

	return err
}

func (e *ProcessingEngine) chunkWorker(
	ctx context.Context,
	chunkChan <-chan *models.Chunk,
	resultChan chan<- *interfaces.ChunkResult,
	documentID string,
	embedder interfaces.Embedder,
	db *sql.DB,
) {
	for chunk := range chunkChan {
		result := &interfaces.ChunkResult{
			Chunk: chunk,
		}

		// Set document ID and generate UUID
		chunk.DocumentID = documentID
		chunk.ID = uuid.New().String()

		// Generate embedding
		if chunk.Body != nil {
			embedding, err := embedder.GenerateEmbedding(ctx, *chunk.Body)
			if err != nil {
				result.Error = fmt.Errorf("embedding generation failed: %w", err)
				resultChan <- result
				continue
			}

			// Create embedding record
			modelName := embedder.GetModelName()
			result.Embedding = &models.Embedding{
				ID:         uuid.New().String(),
				Model:      &modelName,
				EmbeddedAt: time.Now(),
				ObjectID:   chunk.ID,
				ObjectType: "chunk",
			}

			// Set appropriate embedding field based on dimension
			switch embedder.GetDimension() {
			case embeddingDim768:
				result.Embedding.Embedding768 = embedding
			case embeddingDim1536:
				result.Embedding.Embedding1536 = embedding
			case embeddingDim3072:
				result.Embedding.Embedding3072 = embedding
			default:
				e.logger.Error().
					Str("model_name", modelName).
					Int("dimension", embedder.GetDimension()).
					Msg("Unsupported embedding dimension")
				result.Error = ErrUnsupportedEmbeddingDim
				resultChan <- result
				continue
			}
		}

		// Save chunk and embedding to database
		if err := e.saveChunkAndEmbedding(ctx, chunk, result.Embedding, db); err != nil {
			e.logger.Error().Err(err).Str("chunk_id", chunk.ID).Msg("Failed to save chunk and embedding")
			result.Error = err
		}

		resultChan <- result
	}
}

func (e *ProcessingEngine) saveChunkAndEmbedding(
	ctx context.Context,
	chunk *models.Chunk,
	embedding *models.Embedding,
	db *sql.DB,
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		e.logger.Error().Err(err).Msg("Failed to begin transaction")
		return err
	}
	defer func(tx *sql.Tx) {
		err := tx.Rollback()
		if err != nil {
			e.logger.Error().Err(err).Msg("Failed to rollback transaction")
		}
	}(tx)

	// Insert chunk
	chunkQuery := `INSERT INTO chunks (id, document_id, parent_chunk_id, left_chunk_id, right_chunk_id, 
					body, byte_size, tokenizer, token_count, natural_lang, code_lang)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.ExecContext(ctx, chunkQuery, chunk.ID, chunk.DocumentID, chunk.ParentChunkID,
		chunk.LeftChunkID, chunk.RightChunkID, chunk.Body, chunk.ByteSize, chunk.Tokenizer,
		chunk.TokenCount, chunk.NaturalLang, chunk.CodeLang)
	if err != nil {
		e.logger.Error().Err(err).Str("chunk_id", chunk.ID).Msg("Failed to insert chunk")
		return err
	}

	// Insert embedding
	if embedding != nil {
		var embeddingQuery string
		var embeddingValue []float32

		switch {
		case embedding.Embedding768 != nil:
			embeddingQuery = `INSERT INTO embeddings (id, embedding_768, model, embedded_at, object_id, object_type)
							VALUES (?, ?, ?, ?, ?, ?)`
			embeddingValue = embedding.Embedding768
		case embedding.Embedding1536 != nil:
			embeddingQuery = `INSERT INTO embeddings (id, embedding_1536, model, embedded_at, object_id, object_type)
							VALUES (?, ?, ?, ?, ?, ?)`
			embeddingValue = embedding.Embedding1536
		case embedding.Embedding3072 != nil:
			embeddingQuery = `INSERT INTO embeddings (id, embedding_3072, model, embedded_at, object_id, object_type)
							VALUES (?, ?, ?, ?, ?, ?)`
			embeddingValue = embedding.Embedding3072
		default:
			return ErrNoEmbeddingVector
		}

		// Convert embedding to string format for SQLite
		embeddingStr := fmt.Sprintf("[%v]", embeddingValue)

		modelName := ""
		if embedding.Model != nil {
			modelName = *embedding.Model
		}

		_, err = tx.ExecContext(ctx, embeddingQuery, embedding.ID, embeddingStr,
			modelName, embedding.EmbeddedAt.Format(time.RFC3339),
			embedding.ObjectID, embedding.ObjectType)
		if err != nil {
			e.logger.Error().Err(err).Str("embedding_id", embedding.ID).Msg("Failed to insert embedding")
			return err
		}
	}

	return tx.Commit()
}
