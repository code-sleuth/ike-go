package chunkers

import (
	"errors"
	"os"
	"strconv"
	"strings"

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

const (
	maxTokensDefault     = 100
	overlapTokensDefault = 20
)

// TokenChunker implements token-based chunking using tiktoken.
type TokenChunker struct {
	encoding tokenizer.Codec
	logger   zerolog.Logger
}

// NewTokenChunker creates a new token-based chunker.
func NewTokenChunker() (*TokenChunker, error) {
	// Get log level from environment or default to error
	logLevel := getLogLevelFromEnv()
	logger := util.NewLogger(logLevel)

	// Get tokenizer from environment or default to cl100k_base
	tokenizerName := getTokenizerFromEnv()
	encoding, err := getTokenizerEncoding(tokenizerName)
	if err != nil {
		logger.Error().Err(err).Str("tokenizer", tokenizerName).Msg("failed to get tokenizer")
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
			Tokenizer:  stringPtr(getTokenizerFromEnv()),
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
			Tokenizer:  stringPtr(getTokenizerFromEnv()),
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

// getTokenizerFromEnv returns the tokenizer name from environment or default.
func getTokenizerFromEnv() string {
	tokenizerName := os.Getenv("CHUNKER_TOKENIZER")
	if tokenizerName == "" {
		return "cl100k_base"
	}
	return tokenizerName
}

// getTokenizerEncoding returns the tokenizer encoding for the given name.
func getTokenizerEncoding(name string) (tokenizer.Codec, error) {
	switch strings.ToLower(name) {
	case "cl100k_base":
		return tokenizer.Get(tokenizer.Cl100kBase)
	case "p50k_base":
		return tokenizer.Get(tokenizer.P50kBase)
	case "r50k_base":
		return tokenizer.Get(tokenizer.R50kBase)
	default:
		// Default to cl100k_base for unknown tokenizers
		return tokenizer.Get(tokenizer.Cl100kBase)
	}
}

// getLogLevelFromEnv returns the log level from environment or default.
func getLogLevelFromEnv() zerolog.Level {
	logLevel := os.Getenv("CHUNKER_LOG_LEVEL")
	switch strings.ToLower(logLevel) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.ErrorLevel
	}
}

// getIntFromEnv returns an integer from environment variable or default value.
func getIntFromEnv(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	if intValue, err := strconv.Atoi(value); err == nil {
		return intValue
	}
	return defaultValue
}

// GetDefaultMaxTokens returns the default max tokens from environment or default.
func GetDefaultMaxTokens() int {
	return getIntFromEnv("CHUNKER_DEFAULT_MAX_TOKENS", maxTokensDefault)
}

// GetDefaultOverlapTokens returns the default overlap tokens from environment or default.
func GetDefaultOverlapTokens() int {
	return getIntFromEnv("CHUNKER_DEFAULT_OVERLAP_TOKENS", overlapTokensDefault)
}
