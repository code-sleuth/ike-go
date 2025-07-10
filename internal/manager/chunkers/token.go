package chunkers

import (
	"errors"

	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/pkg/util"
	"github.com/rs/zerolog"

	"github.com/google/uuid"
	"github.com/tiktoken-go/tokenizer"
)

var (
	ErrContentEmpty     = errors.New("content cannot be empty")
	ErrInvalidMaxTokens = errors.New("maxTokens must be positive")
	ErrInvalidOverlap   = errors.New("overlapTokens must be between 0 and maxTokens")
)

// TokenChunker implements token-based chunking using tiktoken.
type TokenChunker struct {
	encoding tokenizer.Codec
	logger   zerolog.Logger
}

// NewTokenChunker creates a new token-based chunker.
func NewTokenChunker() (*TokenChunker, error) {
	logger := util.NewLogger(zerolog.ErrorLevel)
	// Using cl100k_base encoding because it's used by GPT-3.5-turbo and GPT-4
	encoding, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get tokenizer")
		return nil, err
	}

	return &TokenChunker{
		encoding: encoding,
		logger:   logger,
	}, nil
}

// GetChunkingStrategy returns the strategy name used by this chunker.
func (t *TokenChunker) GetChunkingStrategy() string {
	return "token"
}

// ChunkDocument splits a document into manageable chunks based on token count.
func (t *TokenChunker) ChunkDocument(content string, maxTokens int) ([]*models.Chunk, error) {
	if content == "" {
		t.logger.Warn().Msg("content is empty")
		return nil, ErrContentEmpty
	}

	if maxTokens <= 0 {
		t.logger.Warn().Msg("maxTokens must be positive")
		return nil, ErrInvalidMaxTokens
	}

	// Tokenize the entire content
	tokens, _, err := t.encoding.Encode(content)
	if err != nil {
		t.logger.Err(err).Msg("failed to tokenize content")
		return nil, err
	}

	totalTokens := len(tokens)

	// If content fits in one chunk, return it as is
	if totalTokens <= maxTokens {
		chunk := &models.Chunk{
			ID:         uuid.New().String(),
			Body:       &content,
			ByteSize:   intPtr(len([]byte(content))),
			Tokenizer:  stringPtr("cl100k_base"),
			TokenCount: &totalTokens,
		}
		return []*models.Chunk{chunk}, nil
	}

	// Split into multiple chunks
	var chunks []*models.Chunk
	var previousChunkID *string

	for i := 0; i < totalTokens; i += maxTokens {
		end := i + maxTokens
		if end > totalTokens {
			end = totalTokens
		}

		// Get the token slice
		chunkTokens := tokens[i:end]

		// Decode back to text
		chunkText, err := t.encoding.Decode(chunkTokens)
		if err != nil {
			t.logger.Err(err).Msg("failed to decode chunk tokens")
			return nil, err
		}

		// Create chunk
		chunkID := uuid.New().String()
		chunk := &models.Chunk{
			ID:          chunkID,
			Body:        &chunkText,
			ByteSize:    intPtr(len([]byte(chunkText))),
			Tokenizer:   stringPtr("cl100k_base"),
			TokenCount:  intPtr(len(chunkTokens)),
			LeftChunkID: previousChunkID,
		}

		// Update previous chunk's right pointer
		if len(chunks) > 0 {
			chunks[len(chunks)-1].RightChunkID = &chunkID
		}

		chunks = append(chunks, chunk)
		previousChunkID = &chunkID
	}

	return chunks, nil
}

// ChunkDocumentWithOverlap splits a document with overlapping chunks for better context.
func (t *TokenChunker) ChunkDocumentWithOverlap(
	content string,
	maxTokens int,
	overlapTokens int,
) ([]*models.Chunk, error) {
	if content == "" {
		t.logger.Warn().Msg("content is empty")
		return nil, ErrContentEmpty
	}

	if maxTokens <= 0 {
		t.logger.Warn().Msg("maxTokens must be positive")
		return nil, ErrInvalidMaxTokens
	}

	if overlapTokens < 0 || overlapTokens >= maxTokens {
		t.logger.Warn().Msg("overlapTokens must be between 0 and maxTokens")
		return nil, ErrInvalidOverlap
	}

	// Tokenize the entire content
	tokens, _, err := t.encoding.Encode(content)
	if err != nil {
		t.logger.Err(err).Msg("failed to tokenize content")
		return nil, err
	}

	totalTokens := len(tokens)

	// If content fits in one chunk, return it as-is
	if totalTokens <= maxTokens {
		chunk := &models.Chunk{
			ID:         uuid.New().String(),
			Body:       &content,
			ByteSize:   intPtr(len([]byte(content))),
			Tokenizer:  stringPtr("cl100k_base"),
			TokenCount: &totalTokens,
		}
		return []*models.Chunk{chunk}, nil
	}

	// Split into overlapping chunks
	var chunks []*models.Chunk
	var previousChunkID *string
	stepSize := maxTokens - overlapTokens

	for i := 0; i < totalTokens; i += stepSize {
		end := i + maxTokens
		if end > totalTokens {
			end = totalTokens
		}

		// Get the token slice
		chunkTokens := tokens[i:end]

		// Decode back to text
		chunkText, err := t.encoding.Decode(chunkTokens)
		if err != nil {
			t.logger.Err(err).Msg("failed to decode chunk tokens")
			return nil, err
		}

		// Create chunk
		chunkID := uuid.New().String()
		chunk := &models.Chunk{
			ID:          chunkID,
			Body:        &chunkText,
			ByteSize:    intPtr(len([]byte(chunkText))),
			Tokenizer:   stringPtr("cl100k_base"),
			TokenCount:  intPtr(len(chunkTokens)),
			LeftChunkID: previousChunkID,
		}

		// Update previous chunk's right pointer
		if len(chunks) > 0 {
			chunks[len(chunks)-1].RightChunkID = &chunkID
		}

		chunks = append(chunks, chunk)
		previousChunkID = &chunkID

		// If we've processed all tokens, break
		if end >= totalTokens {
			break
		}
	}

	return chunks, nil
}

// CountTokens returns the number of tokens in the given text.
func (t *TokenChunker) CountTokens(text string) (int, error) {
	tokens, _, err := t.encoding.Encode(text)
	if err != nil {
		t.logger.Err(err).Msg("failed to tokenize text")
		return 0, err
	}
	return len(tokens), nil
}

// Helper functions.
func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}
