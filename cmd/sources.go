package cmd

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/internal/manager/repository"
	"github.com/code-sleuth/ike-go/pkg/db"
	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var sourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "Manage sources",
	Long:  `Manage sources in the database, create, list, get, update, and delete.`,
}

var sourcesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sources",
	Run: func(_ *cobra.Command, _ []string) {
		logger := util.NewLogger(zerolog.ErrorLevel)

		database, err := db.NewConnection()
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to connect to database")
		}
		defer func(database *db.DB) {
			err := database.Close()
			if err != nil {
				logger.Fatal().Err(err).Msgf("Failed to close database: %v\n", err)
			}
		}(database)

		repo := repository.NewSourceRepository(database)
		sources, err := repo.List()
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to list sources: %v\n", err)
		}

		if len(sources) == 0 {
			logger.Error().Msg("No sources found")
			return
		}

		jsonOutput, err := json.MarshalIndent(sources, "", "  ")
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to marshal JSON: %v\n", err)
		}
		logger.Info().Msg(string(jsonOutput))
	},
}

var sourcesGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get a source by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		logger := util.NewLogger(zerolog.ErrorLevel)
		database, err := db.NewConnection()
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to connect to database: %v\n", err)
		}
		defer database.Close()

		repo := repository.NewSourceRepository(database)
		source, err := repo.GetByID(args[0])
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to get source: %v\n", err)
		}

		jsonOutput, err := json.MarshalIndent(source, "", "  ")
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to marshal JSON: %v\n", err)
		}
		logger.Info().Msg(string(jsonOutput))
	},
}

var sourcesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new source",
	Run: func(cmd *cobra.Command, _ []string) {
		logger := util.NewLogger(zerolog.ErrorLevel)
		database, err := db.NewConnection()
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to connect to database: %v\n", err)
		}
		defer database.Close()

		id, _ := cmd.Flags().GetString("id")
		rawURL, _ := cmd.Flags().GetString("url")
		authorEmail, _ := cmd.Flags().GetString("author-email")
		activeDomain, _ := cmd.Flags().GetInt("active-domain")
		format, _ := cmd.Flags().GetString("format")

		if strings.EqualFold(id, "") || strings.EqualFold(rawURL, "") {
			logger.Fatal().Err(err).Msgf("ID and URL are required")
		}

		source := &models.Source{
			ID:           id,
			RawURL:       &rawURL,
			ActiveDomain: activeDomain,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if !strings.EqualFold(authorEmail, "") {
			source.AuthorEmail = &authorEmail
		}
		if !strings.EqualFold(format, "") {
			source.Format = &format
		}

		repo := repository.NewSourceRepository(database)
		err = repo.Create(source)
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to create source: %v\n", err)
		}

		logger.Info().Msgf("Source created successfully with ID: %s\n", id)
	},
}

var sourcesDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a source by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		logger := util.NewLogger(zerolog.ErrorLevel)
		database, err := db.NewConnection()
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to connect to database: %v\n", err)
		}
		defer database.Close()

		repo := repository.NewSourceRepository(database)
		err = repo.Delete(args[0])
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to delete source: %v\n", err)
		}

		logger.Info().Msgf("Source deleted successfully: %s\n", args[0])
	},
}

func init() {
	rootCmd.AddCommand(sourcesCmd)
	sourcesCmd.AddCommand(sourcesListCmd)
	sourcesCmd.AddCommand(sourcesGetCmd)
	sourcesCmd.AddCommand(sourcesCreateCmd)
	sourcesCmd.AddCommand(sourcesDeleteCmd)

	sourcesCreateCmd.Flags().String("id", "", "Source ID (required)")
	sourcesCreateCmd.Flags().String("url", "", "Raw URL (required)")
	sourcesCreateCmd.Flags().String("author-email", "", "Author email")
	sourcesCreateCmd.Flags().Int("active-domain", 1, "Active domain (0 or 1)")
	sourcesCreateCmd.Flags().String("format", "", "Format (json, yml, yaml)")
}
