package transformers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/pkg/util"
	"github.com/rs/zerolog"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/google/uuid"
)

var (
	ErrCannotTransformWPDownload = errors.New("cannot transform this download, not a valid WordPress JSON response")
	ErrNoContentField            = errors.New("no content field found")
	ErrContentFieldNotObject     = errors.New("content field is not an object")
	ErrNoRenderedContent         = errors.New("no rendered content found")
	ErrRenderedContentNotString  = errors.New("rendered content is not a string")
)

// WPJSONTransformer handles transforming WordPress JSON API downloads into documents.
type WPJSONTransformer struct {
	markdownConverter *md.Converter
	logger            zerolog.Logger
}

// NewWPJSONTransformer creates a new WordPress JSON transformer.
func NewWPJSONTransformer() *WPJSONTransformer {
	converter := md.NewConverter("", true, nil)
	logger := util.NewLogger(zerolog.ErrorLevel)

	// Configure the converter for better markdown output
	// Use default configuration for now

	return &WPJSONTransformer{
		markdownConverter: converter,
		logger:            logger,
	}
}

// GetSourceType returns the source type this transformer handles.
func (w *WPJSONTransformer) GetSourceType() string {
	return "wp-json"
}

// CanTransform checks if this transformer can handle the given download.
func (w *WPJSONTransformer) CanTransform(download *models.Download) bool {
	if download.Body == nil {
		return false
	}

	// Try to parse as JSON and check for WordPress-specific fields
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*download.Body), &data); err != nil {
		w.logger.Error().Err(err).Msg("failed to parse JSON body")
		return false
	}

	// Check for WordPress-specific fields
	_, hasContent := data["content"]
	_, hasTitle := data["title"]
	_, hasDate := data["date_gmt"]
	_, hasModified := data["modified_gmt"]

	return hasContent && hasTitle && hasDate && hasModified
}

// Transform converts a WordPress JSON download into a structured document.
func (w *WPJSONTransformer) Transform(
	ctx context.Context,
	download *models.Download,
	db *sql.DB,
) (*interfaces.TransformResult, error) {
	if !w.CanTransform(download) {
		w.logger.Error().Msgf("cannot transform this download, its not a valid WordPress JSON response: %+v", download)
		return nil, ErrCannotTransformWPDownload
	}

	w.logger.Info().Msgf("Starting WP-JSON transformation for download: %s", download.ID)

	// Parse the JSON body
	var wpData map[string]interface{}
	if err := json.Unmarshal([]byte(*download.Body), &wpData); err != nil {
		w.logger.Error().Err(err).Msg("failed to parse JSON body")
		return nil, err
	}

	// Extract content and convert to markdown
	content, err := w.extractContent(wpData)
	if err != nil {
		w.logger.Error().Err(err).Msg("failed to extract content")
		return nil, err
	}

	// Extract document metadata
	document, err := w.extractDocument(wpData, download)
	if err != nil {
		w.logger.Error().Err(err).Msg("failed to extract document data")
		return nil, err
	}

	// Detect language
	language := w.detectLanguage(content)

	// Extract metadata
	metadata := w.extractMetadata(wpData, content)

	// Save document to database
	if err := w.saveDocument(ctx, document, db); err != nil {
		w.logger.Error().Err(err).Msg("failed to save document")
		return nil, err
	}

	// Save metadata to database
	if err := w.saveMetadata(ctx, document.ID, metadata, db); err != nil {
		w.logger.Error().Err(err).Msg("failed to save metadata")
		return nil, err
	}

	w.logger.Info().Msgf("WP-JSON transformation completed for document: %s", document.ID)

	return &interfaces.TransformResult{
		Document: document,
		Content:  content,
		Language: language,
		Metadata: metadata,
	}, nil
}

// extractContent extracts and converts the content to markdown.
func (w *WPJSONTransformer) extractContent(wpData map[string]interface{}) (string, error) {
	contentObj, exists := wpData["content"]
	var err error
	if !exists {
		w.logger.Error().Msg("no content field found")
		err = ErrNoContentField
		return "", err
	}

	contentMap, ok := contentObj.(map[string]interface{})
	if !ok {
		w.logger.Error().Msg("content field is not an object")
		err = ErrContentFieldNotObject
		return "", err
	}

	rendered, exists := contentMap["rendered"]
	if !exists {
		w.logger.Error().Msg("no rendered content found")
		err = ErrNoRenderedContent
		return "", err
	}

	htmlContent, ok := rendered.(string)
	if !ok {
		w.logger.Error().Msg("rendered content is not a string")
		err = ErrRenderedContentNotString
		return "", err
	}

	// Convert HTML to markdown
	markdown, err := w.markdownConverter.ConvertString(htmlContent)
	if err != nil {
		w.logger.Error().Err(err).Msg("failed to convert HTML to markdown")
		return "", err
	}

	return markdown, nil
}

// extractDocument extracts document metadata and creates a document record.
func (w *WPJSONTransformer) extractDocument(
	wpData map[string]interface{},
	download *models.Download,
) (*models.Document, error) {
	const (
		minChunkSize = 212
		maxChunkSize = 8191 // Default for OpenAI embeddings
	)
	document := &models.Document{
		ID:           uuid.New().String(),
		SourceID:     download.SourceID,
		DownloadID:   download.ID,
		Format:       stringPtr("md"),
		MinChunkSize: minChunkSize,
		MaxChunkSize: maxChunkSize,
	}

	// Extract and parse dates
	if dateGMT, exists := wpData["date_gmt"].(string); exists {
		parsed, err := time.Parse("2006-01-02T15:04:05", dateGMT)
		if err != nil {
			w.logger.Error().Err(err).Msgf("failed to parse date: %s", dateGMT)
			return nil, err
		}
		document.PublishedAt = &parsed
	}

	if modifiedGMT, exists := wpData["modified_gmt"].(string); exists {
		parsed, err := time.Parse("2006-01-02T15:04:05", modifiedGMT)
		if err != nil {
			w.logger.Error().Err(err).Msgf("failed to parse modified date: %s", modifiedGMT)
			return nil, err
		}
		document.ModifiedAt = &parsed
	}

	// Set indexed time to now
	now := time.Now()
	document.IndexedAt = &now

	return document, nil
}

// extractMetadata extracts various metadata fields from WordPress data.
func (w *WPJSONTransformer) extractMetadata(wpData map[string]interface{}, content string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Extract title
	if titleObj, exists := wpData["title"]; exists {
		if titleMap, ok := titleObj.(map[string]interface{}); ok {
			if rendered, exists := titleMap["rendered"].(string); exists {
				// Convert HTML to markdown for title
				if titleMD, err := w.markdownConverter.ConvertString(rendered); err == nil {
					metadata["document_title"] = strings.TrimSpace(titleMD)
				}
			}
		}
	}

	// Extract excerpt/description
	if excerptObj, exists := wpData["excerpt"]; exists {
		if excerptMap, ok := excerptObj.(map[string]interface{}); ok {
			if rendered, exists := excerptMap["rendered"].(string); exists {
				// Convert HTML to markdown for excerpt
				if excerptMD, err := w.markdownConverter.ConvertString(rendered); err == nil {
					metadata["document_description"] = strings.TrimSpace(excerptMD)
				}
			}
		}
	}

	// Count links in content
	metadata["links_count"] = w.countLinks(content)

	// Extract canonical URL
	if link, exists := wpData["link"].(string); exists {
		metadata["canonical_url"] = link
	}

	// Extract author information
	if author, exists := wpData["author"].(float64); exists {
		metadata["author_id"] = int(author)
	}

	// Extract status
	if status, exists := wpData["status"].(string); exists {
		metadata["status"] = status
	}

	// Extract type
	if postType, exists := wpData["type"].(string); exists {
		metadata["post_type"] = postType
	}

	// Extract slug
	if slug, exists := wpData["slug"].(string); exists {
		metadata["slug"] = slug
	}

	// Extract featured media
	if featuredMedia, exists := wpData["featured_media"].(float64); exists {
		metadata["featured_media"] = int(featuredMedia)
	}

	// Extract categories and tags if present
	if categories, exists := wpData["categories"].([]interface{}); exists {
		metadata["categories"] = categories
	}

	if tags, exists := wpData["tags"].([]interface{}); exists {
		metadata["tags"] = tags
	}

	return metadata
}

// detectLanguage attempts to detect the language of the content.
func (w *WPJSONTransformer) detectLanguage(content string) string {
	// Simple heuristic for now - could be enhanced with actual language detection
	// For now, assume English unless we find French indicators
	content = strings.ToLower(content)

	frenchIndicators := []string{
		"le ", "la ", "les ", "un ", "une ", "des ",
		"et ", "ou ", "mais ", "donc ", "car ", "ni ",
		"que ", "qui ", "quoi ", "dont ", "où ",
		"avec ", "sans ", "pour ", "par ", "sur ",
		"dans ", "de ", "du ", "des ", "au ", "aux ",
	}

	frenchCount := 0
	for _, indicator := range frenchIndicators {
		if strings.Contains(content, indicator) {
			frenchCount++
		}
	}

	// If we find several French indicators, assume French
	frCount := 3
	if frenchCount >= frCount {
		return "fr"
	}

	return "en"
}

// countLinks counts the number of markdown links in the content.
func (w *WPJSONTransformer) countLinks(content string) int {
	// Regex pattern for markdown links: [text](url)
	linkPattern := regexp.MustCompile(`\[([^\]]*)\]\([^\)]*\)`)
	matches := linkPattern.FindAllString(content, -1)
	return len(matches)
}

// saveDocument saves the document to the database.
func (w *WPJSONTransformer) saveDocument(ctx context.Context, document *models.Document, db *sql.DB) error {
	query := `INSERT INTO documents (id, source_id, download_id, format, indexed_at, min_chunk_size, 
                       max_chunk_size, published_at, modified_at, wp_version)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var indexedAtStr, publishedAtStr, modifiedAtStr *string

	if document.IndexedAt != nil {
		str := document.IndexedAt.Format(time.RFC3339)
		indexedAtStr = &str
	}
	if document.PublishedAt != nil {
		str := document.PublishedAt.Format(time.RFC3339)
		publishedAtStr = &str
	}
	if document.ModifiedAt != nil {
		str := document.ModifiedAt.Format(time.RFC3339)
		modifiedAtStr = &str
	}

	_, err := db.ExecContext(ctx, query, document.ID, document.SourceID, document.DownloadID,
		document.Format, indexedAtStr, document.MinChunkSize, document.MaxChunkSize,
		publishedAtStr, modifiedAtStr, document.WPVersion)

	return err
}

// saveMetadata saves the metadata to the database.
func (w *WPJSONTransformer) saveMetadata(
	ctx context.Context,
	documentID string,
	metadata map[string]interface{},
	db *sql.DB,
) error {
	for key, value := range metadata {
		metaJSON, err := json.Marshal(value)
		if err != nil {
			w.logger.Error().Err(err).Msgf("failed to marshal metadata for key %s: %v", key, value)
			continue
		}

		query := `INSERT INTO document_meta (id, document_id, key, meta, created_at)
				  VALUES (?, ?, ?, ?, ?)
				  ON CONFLICT(document_id, key) DO UPDATE SET
				  	meta = excluded.meta,
				  	created_at = excluded.created_at`

		_, err = db.ExecContext(ctx, query, uuid.New().String(), documentID, key,
			string(metaJSON), time.Now().Format(time.RFC3339))
		if err != nil {
			w.logger.Error().Err(err).Msgf("failed to save metadata for key %s: %v", key, value)
			return err
		}
	}

	return nil
}

// Helper function to create string pointers.
func stringPtr(s string) *string {
	return &s
}
