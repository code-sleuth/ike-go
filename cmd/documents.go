package cmd

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"

	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/pkg/db"
	"github.com/code-sleuth/ike-go/pkg/util"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var documentsCmd = &cobra.Command{
	Use:   "documents",
	Short: "Manage documents",
	Long:  `Manage documents in the database - list and get.`,
}

var documentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all documents",
	Run: func(_ *cobra.Command, _ []string) {
		logger := util.NewLogger(zerolog.ErrorLevel)

		database, err := db.NewConnection()
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to connect to database")
		}
		defer database.Close()

		query := `
			SELECT id, source_id, download_id, format, indexed_at, min_chunk_size, max_chunk_size, 
			published_at, modified_at, wp_version
			FROM documents ORDER BY indexed_at DESC
		`
		rows, err := database.Query(query)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to query documents")
		}
		defer rows.Close()

		var documents []models.Document
		for rows.Next() {
			var doc models.Document
			err := rows.Scan(&doc.ID, &doc.SourceID, &doc.DownloadID, &doc.Format, &doc.IndexedAt,
				&doc.MinChunkSize, &doc.MaxChunkSize, &doc.PublishedAt, &doc.ModifiedAt,
				&doc.WPVersion)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to scan document")
				continue
			}
			documents = append(documents, doc)
		}

		if len(documents) == 0 {
			logger.Info().Msg("No documents found")
			return
		}

		jsonOutput, err := json.MarshalIndent(documents, "", "  ")
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to marshal JSON")
		}
		logger.Info().RawJSON("documents", jsonOutput).Msg("Documents retrieved successfully")
	},
}

var documentsGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get a document by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		logger := util.NewLogger(zerolog.InfoLevel)

		database, err := db.NewConnection()
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to connect to database")
		}
		defer func(database *db.DB) {
			if err := database.Close(); err != nil {
				logger.Error().Err(err).Msg("Failed to close database connection")
			}
		}(database)

		query := `
			SELECT id, source_id, download_id, format, indexed_at, min_chunk_size, max_chunk_size, 
			published_at, modified_at, wp_version
			FROM documents WHERE id = ?
		`
		row := database.QueryRow(query, args[0])

		var doc models.Document
		err = row.Scan(&doc.ID, &doc.SourceID, &doc.DownloadID, &doc.Format, &doc.IndexedAt,
			&doc.MinChunkSize, &doc.MaxChunkSize, &doc.PublishedAt, &doc.ModifiedAt,
			&doc.WPVersion)

		if errors.Is(err, sql.ErrNoRows) {
			logger.Error().Str("document_id", args[0]).Msg("Document not found")
			os.Exit(1)
		}
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to get document")
		}

		jsonOutput, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to marshal JSON")
		}
		logger.Info().RawJSON("document", jsonOutput).Str("document_id", args[0]).Msg("Document retrieved successfully")
	},
}

func init() {
	rootCmd.AddCommand(documentsCmd)
	documentsCmd.AddCommand(documentsListCmd)
	documentsCmd.AddCommand(documentsGetCmd)
}
