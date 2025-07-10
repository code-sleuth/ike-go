package cmd

import (
	"io"
	"os"

	"github.com/code-sleuth/ike-go/pkg/db"
	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Run database migrations to set up the schema in your Turso database.`,
	Run: func(_ *cobra.Command, _ []string) {
		logger := util.NewLogger(zerolog.ErrorLevel)

		database, err := db.NewConnection()
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to connect to database")
		}
		defer func(database *db.DB) {
			if err := database.Close(); err != nil {
				logger.Error().Err(err).Msg("Failed to close database connection")
			}
		}(database)

		migrationFile := "pkg/migrations/init_schema.sql"
		content, err := os.Open(migrationFile)
		if err != nil {
			logger.Fatal().Err(err).Str("migration_file", migrationFile).Msg("Failed to open migration file")
		}
		defer func(content *os.File) {
			if err := content.Close(); err != nil {
				logger.Error().Err(err).Msg("Failed to close migration file")
			}
		}(content)

		sqlBytes, err := io.ReadAll(content)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to read migration file")
		}

		_, err = database.Exec(string(sqlBytes))
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to execute migration")
		}

		logger.Info().Msg("Database migration completed successfully!")
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
