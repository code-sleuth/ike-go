package cmd

import (
	"context"
	"database/sql"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/internal/manager/services"
	"github.com/code-sleuth/ike-go/pkg/db"
	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var downloadID string

// transformCmd represents the transform command.
var transformCmd = &cobra.Command{
	Use:   "transform",
	Short: "Transform existing downloads into documents and chunks",
	Long: `Transform existing download records into structured documents, chunk them, and generate embeddings.
	
Examples:
  # Transform a specific download
  ike-go transform --download-id "123e4567-e89b-12d3-a456-426614174000"
  
  # Transform with custom embedding model
  ike-go transform --download-id "123e4567-e89b-12d3-a456-426614174000" --model "text-embedding-3-large"`,
	Run: runTransform,
}

func init() {
	rootCmd.AddCommand(transformCmd)

	var (
		maxTokens   = 8191
		concurrency = 5
		timeout     = 5 * time.Minute
	)

	// Add flags
	transformCmd.Flags().StringVarP(&downloadID, "download-id", "d", "", "Download ID to transform (required)")
	transformCmd.Flags().StringVarP(&embeddingModel, "model", "m", "text-embedding-3-small", "Embedding model to use")
	transformCmd.Flags().
		StringVarP(&chunkStrategy, "strategy", "s", "token", "Chunking strategy (token, heading, recursive)")
	transformCmd.Flags().IntVarP(&maxTokens, "tokens", "t", maxTokens, "Maximum tokens per chunk")
	transformCmd.Flags().IntVarP(&concurrency, "concurrency", "c", concurrency, "Number of concurrent operations")
	transformCmd.Flags().DurationVar(&timeout, "timeout", timeout, "Timeout for the entire operation")

	// Mark required flags
	err := transformCmd.MarkFlagRequired("download-id")
	if err != nil {
		return
	}
}

func runTransform(_ *cobra.Command, _ []string) {
	logger := util.NewLogger(zerolog.InfoLevel)
	logger.Info().Str("download_id", downloadID).Msg("Starting transformation")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Connect to database
	database, err := db.Connect()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer func(database *sql.DB) {
		err := database.Close()
		if err != nil {
			return
		}
	}(database)

	// Create processing engine
	engine := services.NewProcessingEngine()

	// Register components
	if err := registerTransformers(engine); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register transformers")
	}

	if err := registerChunkers(engine); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register chunkers")
	}

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

	// Run the transformation
	if err := engine.ProcessDocument(ctx, downloadID, options, database); err != nil {
		logger.Fatal().Err(err).Msg("Transformation failed")
	}

	logger.Info().Msg("Transformation completed successfully!")
}
