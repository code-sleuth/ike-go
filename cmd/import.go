package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/chunkers"
	"github.com/code-sleuth/ike-go/internal/manager/embedders"
	"github.com/code-sleuth/ike-go/internal/manager/importers"
	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/internal/manager/services"
	"github.com/code-sleuth/ike-go/internal/manager/transformers"
	"github.com/code-sleuth/ike-go/pkg/db"
	"github.com/code-sleuth/ike-go/pkg/util"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var ErrUnsupportedEmbeddingModel = errors.New("unsupported embedding model")

var (
	sourceURL      string
	embeddingModel string
	chunkStrategy  string
	maxTokens      int
	concurrency    int
	timeout        time.Duration
)

// importCmd represents the import command.
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import content from external sources",
	Long: `Import content from external sources like WordPress JSON API endpoints or GitHub repositories.
	
Examples:
  # Import from WordPress JSON API
  ike-go import --url "https://wsform.com/wp-json/wp/v2/knowledgebase"
  
  # Import from GitHub repository
  ike-go import --url "https://github.com/owner/repo" --model "text-embedding-3-small"
  
  # Import with custom settings
  ike-go import --url "https://example.com/wp-json/wp/v2/posts" --tokens 4096 --concurrency 10`,
	Run: runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)
	var (
		maxTokens   = 8191
		concurrency = 5
		timeout     = 5 * time.Minute
	)

	// Add flags
	importCmd.Flags().StringVarP(&sourceURL, "url", "u", "", "Source URL to import from (required)")
	importCmd.Flags().StringVarP(&embeddingModel, "model", "m", "text-embedding-3-small", "Embedding model to use")
	importCmd.Flags().
		StringVarP(&chunkStrategy, "strategy", "s", "token", "Chunking strategy (token, heading, recursive)")
	importCmd.Flags().IntVarP(&maxTokens, "tokens", "t", maxTokens, "Maximum tokens per chunk")
	importCmd.Flags().IntVarP(&concurrency, "concurrency", "c", concurrency, "Number of concurrent operations")
	importCmd.Flags().DurationVar(&timeout, "timeout", timeout, "Timeout for the entire operation")

	// Mark required flags
	err := importCmd.MarkFlagRequired("url")
	if err != nil {
		return
	}
}

func runImport(_ *cobra.Command, _ []string) {
	logger := util.NewLogger(zerolog.ErrorLevel)
	logger.Info().Str("source_url", sourceURL).Msg("Starting import")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Connect to database
	database, err := db.Connect()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer database.Close()

	// Create processing engine
	engine := services.NewProcessingEngine()

	// Register importers
	if err := registerImporters(engine); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register importers")
	}

	// Register transformers
	if err := registerTransformers(engine); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register transformers")
	}

	// Register chunkers
	if err := registerChunkers(engine); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register chunkers")
	}

	// Register embedders
	if err := registerEmbedders(engine); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register embedders")
	}

	// Configure processing options
	options := &interfaces.ProcessingOptions{
		MaxTokens:      maxTokens,
		ChunkStrategy:  chunkStrategy,
		EmbeddingModel: embeddingModel,
		Concurrency:    concurrency,
		Timeout:        timeout,
	}

	// Run the import
	if err := engine.ProcessSource(ctx, sourceURL, options, database); err != nil {
		logger.Fatal().Err(err).Msg("Import failed")
	}

	logger.Info().Msg("Import completed successfully!")
}

func registerImporters(engine *services.ProcessingEngine) error {
	// Register WP-JSON importer
	wpImporter := importers.NewWPJSONImporter()
	wpImporter.SetConcurrency(concurrency)
	if err := engine.RegisterImporter(wpImporter); err != nil {
		return fmt.Errorf("failed to register WP-JSON importer: %w", err)
	}

	// Register GitHub importer
	githubImporter := importers.NewGitHubImporter()
	if err := engine.RegisterImporter(githubImporter); err != nil {
		return fmt.Errorf("failed to register GitHub importer: %w", err)
	}

	return nil
}

func registerTransformers(engine *services.ProcessingEngine) error {
	// Register WP-JSON transformer
	wpTransformer := transformers.NewWPJSONTransformer()
	if err := engine.RegisterTransformer(wpTransformer); err != nil {
		return fmt.Errorf("failed to register WP-JSON transformer: %w", err)
	}

	// Register GitHub transformer
	githubTransformer := transformers.NewGitHubTransformer()
	if err := engine.RegisterTransformer(githubTransformer); err != nil {
		return fmt.Errorf("failed to register GitHub transformer: %w", err)
	}

	return nil
}

func registerChunkers(engine *services.ProcessingEngine) error {
	// Register token chunker
	tokenChunker, err := chunkers.NewTokenChunker()
	if err != nil {
		return fmt.Errorf("failed to create token chunker: %w", err)
	}
	if err := engine.RegisterChunker(tokenChunker); err != nil {
		return fmt.Errorf("failed to register token chunker: %w", err)
	}

	return nil
}

func registerEmbedders(engine *services.ProcessingEngine) error {
	// Determine which embedder to use based on model
	switch embeddingModel {
	case "text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002":
		// Register OpenAI embedder
		openaiEmbedder, err := embedders.NewOpenAIEmbedder(embeddingModel)
		if err != nil {
			return fmt.Errorf("failed to create OpenAI embedder: %w", err)
		}
		if err := engine.RegisterEmbedder(openaiEmbedder); err != nil {
			return fmt.Errorf("failed to register OpenAI embedder: %w", err)
		}
	case "togethercomputer/m2-bert-80M-8k-retrieval", "togethercomputer/m2-bert-80M-32k-retrieval":
		// Register Together AI embedder
		togetherEmbedder, err := embedders.NewTogetherAIEmbedder(embeddingModel)
		if err != nil {
			return fmt.Errorf("failed to create Together AI embedder: %w", err)
		}
		if err := engine.RegisterEmbedder(togetherEmbedder); err != nil {
			return fmt.Errorf("failed to register Together AI embedder: %w", err)
		}
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedEmbeddingModel, embeddingModel)
	}

	return nil
}
