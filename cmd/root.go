package cmd

import (
	"github.com/code-sleuth/ike-go/pkg/util"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ike-go",
	Short: "A CLI tool for managing document indexing and embeddings",
	Long:  `ike-go is a CLI application for managing sources: documents, chunks, and embeddings.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger := util.NewLogger(zerolog.ErrorLevel)
		logger.Fatal().Err(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	logger := util.NewLogger(zerolog.ErrorLevel)
	err := godotenv.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("No .env file found")
	}
}
