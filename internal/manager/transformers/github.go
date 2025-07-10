package transformers

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/pkg/util"
	"github.com/rs/zerolog"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/google/uuid"
)

const (
	extJSON = ".json"
	extYAML = ".yaml"
	extYML  = ".yml"
)

var ErrCannotTransformDownload = errors.New("cannot transform this download, its not a valid GitHub file")

// GitHubTransformer handles transforming GitHub file downloads into documents.
type GitHubTransformer struct {
	markdownConverter *md.Converter
	logger            zerolog.Logger
}

// NewGitHubTransformer creates a new GitHub transformer.
func NewGitHubTransformer() *GitHubTransformer {
	converter := md.NewConverter("", true, nil)
	logger := util.NewLogger(zerolog.ErrorLevel)

	return &GitHubTransformer{
		markdownConverter: converter,
		logger:            logger,
	}
}

// GetSourceType returns the source type this transformer handles.
func (g *GitHubTransformer) GetSourceType() string {
	return "github"
}

// CanTransform checks if this transformer can handle the given download.
func (g *GitHubTransformer) CanTransform(download *models.Download) bool {
	if download.Body == nil {
		return false
	}

	// Check if the download has GitHub-specific headers
	var headers map[string][]string
	if err := json.Unmarshal([]byte(download.Headers), &headers); err != nil {
		g.logger.Error().Err(err).Msg("failed to unmarshal headers")
		return false
	}

	// Look for GitHub-specific headers
	_, hasGitHubSHA := headers["X-GitHub-SHA"]

	return hasGitHubSHA
}

// Transform converts a GitHub file download into a structured document.
func (g *GitHubTransformer) Transform(
	ctx context.Context,
	download *models.Download,
	db *sql.DB,
) (*interfaces.TransformResult, error) {
	if !g.CanTransform(download) {
		g.logger.Error().Msgf("cannot transform this download, its not a valid GitHub file: %+v", download)
		return nil, ErrCannotTransformDownload
	}

	g.logger.Info().Msgf("Starting GitHub transformation for download: %s", download.ID)

	// Get source information to determine file path and type
	source, err := g.getSource(ctx, download.SourceID, db)
	if err != nil {
		g.logger.Error().Err(err).Msgf("failed to get source for download: %s", download.ID)
		return nil, err
	}

	// Extract file path from source URL
	filePath := g.extractFilePath(source.RawURL)

	// Process content based on file type
	content := g.processContent(*download.Body, filePath)

	// Create document
	document := g.createDocument(download, filePath)

	// Detect language
	language := g.detectLanguage(content, filePath)

	// Extract metadata
	metadata := g.extractMetadata(source, filePath, content)

	// Save document to database
	if err := g.saveDocument(ctx, document, db); err != nil {
		g.logger.Error().Err(err).Msgf("failed to save document for download: %s", download.ID)
		return nil, err
	}

	// Save metadata to database
	if err := g.saveMetadata(ctx, document.ID, metadata, db); err != nil {
		g.logger.Error().Err(err).Msgf("failed to save metadata for download: %s", download.ID)
		return nil, err
	}

	g.logger.Info().Msgf("GitHub transformation completed for document: %s", document.ID)

	return &interfaces.TransformResult{
		Document: document,
		Content:  content,
		Language: language,
		Metadata: metadata,
	}, nil
}

// extractFilePath extracts the file path from a GitHub URL.
func (g *GitHubTransformer) extractFilePath(rawURL *string) string {
	if rawURL == nil {
		g.logger.Warn().Msg("rawURL is nil")
		return ""
	}

	// Extract path from GitHub URL
	// URL format: https://github.com/owner/repo/blob/branch/path/to/file
	url := *rawURL
	parts := strings.Split(url, "/")

	partsLength := 7
	if len(parts) < partsLength {
		g.logger.Warn().Msgf("parts, invalid GitHub URL: %s", url)
		return ""
	}

	// Find the blob part and extract everything after it
	for i, part := range parts {
		if part == "blob" && i+2 < len(parts) {
			// Join all parts after "blob/branch"
			return strings.Join(parts[i+2:], "/")
		}
	}

	return ""
}

// processContent processes the file content based on its type.
func (g *GitHubTransformer) processContent(body, filePath string) string {
	ext := filepath.Ext(filePath)

	// Handle base64 encoded content (GitHub API returns base64 for binary files)
	if strings.Contains(body, "base64") {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			g.logger.Error().Err(err).Msgf("failed to decode base64 content for download: %s", body)
			// If decoding fails, treat as plain text
			return body
		}
		body = string(decoded)
	}

	switch ext {
	case ".md":
		// Markdown files are already in the right format
		return body
	case ".html", ".htm":
		// Convert HTML to markdown
		markdown, err := g.markdownConverter.ConvertString(body)
		if err != nil {
			g.logger.Error().Err(err).Msgf("failed to convert HTML to markdown for download: %s", body)
			return body // Fallback to original content
		}
		return markdown
	case ".txt",
		".py",
		".js",
		".go",
		".java",
		".cpp",
		".c",
		".h",
		".hpp",
		".css",
		extJSON,
		extYAML,
		extYML,
		".toml",
		".ini",
		".cfg",
		".conf":
		// Code and text files - add code fences for syntax highlighting
		language := g.getLanguageFromExtension(ext)
		if language != "" {
			return fmt.Sprintf("```%s\n%s\n```", language, body)
		}
		return fmt.Sprintf("```\n%s\n```", body)
	default:
		// Unknown file type, treat as plain text
		return body
	}
}

// getLanguageFromExtension returns the language identifier for syntax highlighting.
func (g *GitHubTransformer) getLanguageFromExtension(ext string) string {
	languageMap := map[string]string{
		".py":    "python",
		".js":    "javascript",
		".ts":    "typescript",
		".go":    "go",
		".java":  "java",
		".cpp":   "cpp",
		".c":     "c",
		".h":     "c",
		".hpp":   "cpp",
		".css":   "css",
		".html":  "html",
		".htm":   "html",
		".xml":   "xml",
		extJSON:  "json",
		extYAML:  "yaml",
		extYML:   "yaml",
		".toml":  "toml",
		".ini":   "ini",
		".cfg":   "ini",
		".conf":  "ini",
		".sh":    "bash",
		".bash":  "bash",
		".zsh":   "zsh",
		".fish":  "fish",
		".ps1":   "powershell",
		".sql":   "sql",
		".r":     "r",
		".rb":    "ruby",
		".php":   "php",
		".swift": "swift",
		".kt":    "kotlin",
		".scala": "scala",
		".rs":    "rust",
		".dart":  "dart",
		".lua":   "lua",
		".pl":    "perl",
	}

	return languageMap[ext]
}

// createDocument creates a document record.
func (g *GitHubTransformer) createDocument(download *models.Download, filePath string) *models.Document {
	const (
		minChunkSize = 212
		maxChunkSize = 8191 // Default for OpenAI embeddings
	)
	document := &models.Document{
		ID:           uuid.New().String(),
		SourceID:     download.SourceID,
		DownloadID:   download.ID,
		MinChunkSize: minChunkSize,
		MaxChunkSize: maxChunkSize,
	}

	// Set format based on file extension
	ext := filepath.Ext(filePath)
	switch ext {
	case extJSON:
		document.Format = stringPtr("json")
	case extYAML, extYML:
		document.Format = stringPtr("yaml")
	default:
		// All other file types default to json
		document.Format = stringPtr("json")
	}

	// Set indexed time to now
	now := time.Now()
	document.IndexedAt = &now

	// For GitHub files, we don't have publication/modification dates from the API
	// These would need to be fetched from commit history if needed

	return document
}

// detectLanguage detects the language of the content.
func (g *GitHubTransformer) detectLanguage(content, filePath string) string {
	ext := filepath.Ext(filePath)

	// First, check if it's a code file
	if g.isCodeFile(ext) {
		// For code files, detect based on comments and keywords
		return g.detectCodeLanguage(content, ext)
	}

	// For text files, do simple natural language detection
	return g.detectNaturalLanguage(content)
}

// isCodeFile checks if the file extension indicates a code file.
func (g *GitHubTransformer) isCodeFile(ext string) bool {
	codeExts := []string{
		".py",
		".js",
		".ts",
		".go",
		".java",
		".cpp",
		".c",
		".h",
		".hpp",
		".css",
		".html",
		".htm",
		".xml",
		extJSON,
		extYAML,
		extYML,
		".toml",
		".ini",
		".cfg",
		".conf",
		".sh",
		".bash",
		".zsh",
		".fish",
		".ps1",
		".sql",
		".r",
		".rb",
		".php",
		".swift",
		".kt",
		".scala",
		".rs",
		".dart",
		".lua",
		".pl",
	}

	for _, codeExt := range codeExts {
		if ext == codeExt {
			return true
		}
	}

	return false
}

// detectCodeLanguage detects the programming language.
func (g *GitHubTransformer) detectCodeLanguage(_, ext string) string {
	// For code files, we return the programming language
	lang := g.getLanguageFromExtension(ext)

	// Only return languages that are allowed by the database schema
	allowedLangs := map[string]bool{
		"python":     true,
		"sql":        true,
		"javascript": true,
	}

	if allowedLangs[lang] {
		return lang
	}

	// Return empty string for unsupported languages
	return ""
}

// detectNaturalLanguage detects the natural language of text content.
func (g *GitHubTransformer) detectNaturalLanguage(content string) string {
	// Simple heuristic for now.
	// TODO: could be enhanced with actual language detection
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
	minFrenchCount := 3
	if frenchCount >= minFrenchCount {
		return "fr"
	}

	return "en"
}

// extractMetadata extracts metadata from the GitHub file.
func (g *GitHubTransformer) extractMetadata(source *models.Source, filePath, content string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Extract repository information from URL
	if source.RawURL != nil {
		repoInfo := g.extractRepoInfo(*source.RawURL)
		metadata["repository"] = repoInfo
	}

	// File information
	metadata["file_path"] = filePath
	metadata["file_name"] = filepath.Base(filePath)
	metadata["file_extension"] = filepath.Ext(filePath)
	metadata["file_size"] = len(content)

	// Content analysis
	metadata["line_count"] = strings.Count(content, "\n") + 1
	metadata["character_count"] = len(content)

	// Language detection
	if g.isCodeFile(filepath.Ext(filePath)) {
		metadata["content_type"] = "code"
		metadata["programming_language"] = g.getLanguageFromExtension(filepath.Ext(filePath))
	} else {
		metadata["content_type"] = "text"
		metadata["natural_language"] = g.detectNaturalLanguage(content)
	}

	// Directory information
	if dir := filepath.Dir(filePath); dir != "." {
		metadata["directory"] = dir
	}

	return metadata
}

// extractRepoInfo extracts repository information from GitHub URL.
func (g *GitHubTransformer) extractRepoInfo(rawURL string) map[string]string {
	parts := strings.Split(rawURL, "/")

	partsLength := 5
	if len(parts) < partsLength {
		g.logger.Warn().Msgf("parts, invalid GitHub URL: %s", rawURL)
		return nil
	}

	info := make(map[string]string)

	// Extract owner and repo from URL
	partsLengthOwnerRepo := 5
	if len(parts) >= partsLengthOwnerRepo {
		info["owner"] = parts[3]
		info["repo"] = parts[4]
	}

	// Extract branch if present
	partsLengthBranch := 7
	if len(parts) >= partsLengthBranch && parts[5] == "blob" {
		// For complex branch names with slashes, join all parts after index 6
		// until we find the file path
		var branchParts []string
		for i := 6; i < len(parts)-1; i++ { // -1 to exclude the file name
			branchParts = append(branchParts, parts[i])
		}
		if len(branchParts) > 0 {
			info["branch"] = strings.Join(branchParts, "/")
		}
	}

	return info
}

// getSource retrieves source information from the database.
func (g *GitHubTransformer) getSource(ctx context.Context, sourceID string, db *sql.DB) (*models.Source, error) {
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
		g.logger.Error().Err(err).Msgf("failed to get source for ID: %s", sourceID)
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

// saveDocument saves the document to the database.
func (g *GitHubTransformer) saveDocument(ctx context.Context, document *models.Document, db *sql.DB) error {
	query := `INSERT INTO documents (id, source_id, download_id, format, indexed_at, 
                       min_chunk_size, max_chunk_size, published_at, modified_at, wp_version)
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
	if err != nil {
		g.logger.Error().Err(err).Msgf("failed to save document for ID: %s", document.ID)
	}

	return err
}

// saveMetadata saves the metadata to the database.
func (g *GitHubTransformer) saveMetadata(
	ctx context.Context,
	documentID string,
	metadata map[string]interface{},
	db *sql.DB,
) error {
	for key, value := range metadata {
		metaJSON, err := json.Marshal(value)
		if err != nil {
			g.logger.Error().Err(err).Msgf("failed to marshal metadata for key %s: %v", key, err)
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
			g.logger.Error().Err(err).Msgf("failed to save metadata for key %s: %v", key, err)
			return err
		}
	}

	return nil
}
