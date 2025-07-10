package importers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const (
	// HTTP client timeout in seconds.
	defaultHTTPTimeout = 30
	// Default maximum file size in bytes (1MB).
	defaultMaxFileSize = 1024 * 1024
	// Minimum required URL parts for GitHub URL.
	minURLParts = 2
	// HTTP OK status code.
	httpOKStatus = 200
	// Default file format for sources.
	formatJSON = "json"
)

var (
	ErrInvalidGitHubURL       = errors.New("invalid GitHub URL: missing owner or repository")
	ErrImportCompleted        = errors.New("import completed with errors")
	ErrNoFilesImported        = errors.New("no files were successfully imported")
	ErrNotGitHubURL           = errors.New("not a GitHub URL")
	ErrInvalidGitHubURLFormat = errors.New("invalid GitHub URL format")
	ErrGitHubAPIRequestFailed = errors.New("GitHub API request failed")
)

// GitHubImporter handles importing content from GitHub repositories.
type GitHubImporter struct {
	client        *http.Client
	token         string
	exclusions    []string
	maxFileSize   int64
	supportedExts []string
	logger        zerolog.Logger
}

// GitHubRepoInfo represents repository information.
type GitHubRepoInfo struct {
	Owner string
	Repo  string
	Ref   string // branch, tag, or commit SHA
}

// GitHubTreeResponse represents the response from GitHub's tree API.
type GitHubTreeResponse struct {
	Tree []GitHubTreeItem `json:"tree"`
}

// GitHubTreeItem represents a single item in the repository tree.
type GitHubTreeItem struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// GitHubFileResponse represents the response from GitHub's contents API.
type GitHubFileResponse struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	GitURL      string `json:"git_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
}

// NewGitHubImporter creates a new GitHub repository importer.
func NewGitHubImporter() *GitHubImporter {
	logger := util.NewLogger(zerolog.InfoLevel)
	return &GitHubImporter{
		client: &http.Client{
			Timeout: defaultHTTPTimeout * time.Second,
		},
		token:       os.Getenv("GITHUB_TOKEN"),
		maxFileSize: defaultMaxFileSize,
		supportedExts: []string{
			".md",
			".txt",
			".rst",
			".py",
			".js",
			".go",
			".java",
			".cpp",
			".c",
			".h",
			".hpp",
			".css",
			".html",
			".xml",
			".json",
			".yaml",
			".yml",
			".toml",
			".ini",
			".cfg",
			".conf",
		},
		exclusions: []string{
			".git",
			"node_modules",
			".next",
			".nuxt",
			"dist",
			"build",
			".vscode",
			".idea",
			"__pycache__",
			".pytest_cache",
			".coverage",
			".DS_Store",
		},
		logger: logger,
	}
}

const sourceTypeGitHub = "github"

// GetSourceType returns the source type this importer handles.
func (g *GitHubImporter) GetSourceType() string {
	return sourceTypeGitHub
}

// ValidateSource checks if the source URL is valid for this importer.
func (g *GitHubImporter) ValidateSource(sourceURL string) error {
	repoInfo, err := g.parseGitHubURL(sourceURL)
	if err != nil {
		g.logger.Warn().Err(err).Msg("Failed to parse GitHub URL")
		return err
	}

	if strings.EqualFold(repoInfo.Owner, "") || strings.EqualFold(repoInfo.Repo, "") {
		g.logger.Warn().Msg("GitHub URL is missing owner or repository")
		return ErrInvalidGitHubURL
	}

	return nil
}

// Import fetches content from a GitHub repository.
func (g *GitHubImporter) Import(ctx context.Context, sourceURL string, db *sql.DB) (*interfaces.ImportResult, error) {
	if err := g.ValidateSource(sourceURL); err != nil {
		g.logger.Warn().Err(err).Msg("Source validation failed")
		return nil, err
	}

	repoInfo, err := g.parseGitHubURL(sourceURL)
	if err != nil {
		g.logger.Warn().Err(err).Msg("Failed to parse GitHub URL")
		return nil, err
	}

	g.logger.Info().Str("owner", repoInfo.Owner).Str("repo", repoInfo.Repo).Msg("Starting GitHub import")

	// Get repository tree
	tree, err := g.getRepoTree(ctx, repoInfo)
	if err != nil {
		g.logger.Warn().Err(err).Msg("Failed to get repository tree")
		return nil, fmt.Errorf("failed to get repository tree: %w", err)
	}

	// Filter files based on exclusions and supported extensions
	filteredFiles := g.filterFiles(tree.Tree)

	g.logger.Info().Int("file_count", len(filteredFiles)).Msg("Found files to import after filtering")

	// Process files
	var lastResult *interfaces.ImportResult
	var errorsList []error

	for _, file := range filteredFiles {
		result, err := g.importFile(ctx, repoInfo, file, db)
		if err != nil {
			errorsList = append(errorsList, err)
			g.logger.Error().Err(err).Str("file_path", file.Path).Msg("Failed to import file")
		} else {
			lastResult = result
		}
	}

	if len(errorsList) > 0 {
		g.logger.Warn().
			Int("error_count", len(errorsList)).
			Int("total_files", len(filteredFiles)).
			Msg("GitHub import completed with errorsList")
		if lastResult != nil {
			g.logger.Warn().Err(errorsList[0]).Msg("Last error")
			lastResult.Error = ErrImportCompleted
		} else {
			g.logger.Warn().Err(errorsList[0]).Msg("Last error")
			return nil, err
		}
	}

	g.logger.Info().
		Int("successful_files", len(filteredFiles)-len(errorsList)).
		Msg("GitHub import completed successfully")

	if lastResult != nil {
		return lastResult, nil
	}

	return nil, ErrNoFilesImported
}

// parseGitHubURL parses a GitHub URL and extracts repository information.
func (g *GitHubImporter) parseGitHubURL(sourceURL string) (*GitHubRepoInfo, error) {
	parsedURL, err := url.Parse(sourceURL)
	if err != nil {
		g.logger.Warn().Err(err).Msg("Failed to parse URL")
		return nil, err
	}

	if parsedURL.Host != "github.com" && parsedURL.Host != "api.github.com" {
		g.logger.Warn().Msg("Not a GitHub URL")
		return nil, ErrNotGitHubURL
	}

	// Handle different GitHub URL formats
	parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")

	if len(parts) < minURLParts {
		g.logger.Warn().Msg("Invalid GitHub URL format")
		return nil, ErrInvalidGitHubURLFormat
	}

	repoInfo := &GitHubRepoInfo{
		Owner: parts[0],
		Repo:  parts[1],
		Ref:   "main", // usually the Default branch, could be different for some repos
	}

	// Check for specific branch/tag/commit in URL
	if len(parts) >= 4 && parts[2] == "tree" {
		repoInfo.Ref = parts[3]
	}

	return repoInfo, nil
}

// getRepoTree fetches the repository tree from GitHub API.
func (g *GitHubImporter) getRepoTree(ctx context.Context, repoInfo *GitHubRepoInfo) (*GitHubTreeResponse, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
		repoInfo.Owner, repoInfo.Repo, repoInfo.Ref)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		g.logger.Warn().Err(err).Msg("Failed to create request")
		return nil, err
	}

	// Add authentication if token is available
	if g.token != "" {
		// g.logger.Info().Str("token", g.token).Msg("Adding authentication")
		req.Header.Set("Authorization", fmt.Sprintf("token %s", g.token))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.client.Do(req)
	if err != nil {
		g.logger.Error().Err(err).Msg("Request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		g.logger.Error().Int("status_code", resp.StatusCode).Msg("GitHub API request failed")
		return nil, ErrGitHubAPIRequestFailed
	}

	var tree GitHubTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		g.logger.Error().Err(err).Msg("Failed to decode response")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &tree, nil
}

// filterFiles filters files based on exclusions and supported extensions.
func (g *GitHubImporter) filterFiles(items []GitHubTreeItem) []GitHubTreeItem {
	filtered := make([]GitHubTreeItem, 0, len(items))

	for _, item := range items {
		// Skip if not a file (blob)
		if item.Type != "blob" {
			continue
		}

		// Skip if file is too large
		if item.Size > g.maxFileSize {
			continue
		}

		// Check exclusions
		if g.isExcluded(item.Path) {
			continue
		}

		// Check supported extensions
		if !g.isSupportedFile(item.Path) {
			continue
		}

		filtered = append(filtered, item)
	}

	return filtered
}

// isExcluded checks if a file path should be excluded.
func (g *GitHubImporter) isExcluded(path string) bool {
	for _, exclusion := range g.exclusions {
		if strings.Contains(path, exclusion) {
			return true
		}
	}
	return false
}

// isSupportedFile checks if a file has a supported extension.
func (g *GitHubImporter) isSupportedFile(path string) bool {
	ext := filepath.Ext(path)
	for _, supportedExt := range g.supportedExts {
		if strings.EqualFold(ext, supportedExt) {
			return true
		}
	}
	return false
}

// importFile imports a single file from the repository.
func (g *GitHubImporter) importFile(
	ctx context.Context,
	repoInfo *GitHubRepoInfo,
	file GitHubTreeItem,
	db *sql.DB,
) (*interfaces.ImportResult, error) {
	// Build URL for the file
	fileURL := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s",
		repoInfo.Owner, repoInfo.Repo, repoInfo.Ref, file.Path)

	// Get file content
	content, err := g.getFileContent(ctx, repoInfo, file.Path)
	if err != nil {
		g.logger.Error().Err(err).Str("file_path", file.Path).Msg("Failed to get file content")
		return nil, err
	}

	// Create source record
	sourceID, err := g.createSource(ctx, fileURL, repoInfo, file, db)
	if err != nil {
		g.logger.Error().Err(err).Str("file_path", file.Path).Msg("Failed to create source")
		return nil, err
	}

	// Create download record
	downloadID, err := g.createDownload(ctx, sourceID, content, file, db)
	if err != nil {
		g.logger.Error().Err(err).Str("file_path", file.Path).Msg("Failed to create download")
		return nil, err
	}

	return &interfaces.ImportResult{
		SourceID:   sourceID,
		DownloadID: downloadID,
	}, nil
}

// getFileContent fetches the content of a file from GitHub.
func (g *GitHubImporter) getFileContent(ctx context.Context, repoInfo *GitHubRepoInfo, path string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		repoInfo.Owner, repoInfo.Repo, path, repoInfo.Ref)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		g.logger.Error().Err(err).Str("file_path", path).Msg("Failed to create request")
		return "", err
	}

	// Add authentication if token is available
	if g.token != "" {
		// g.logger.Info().Str("token", g.token).Msg("Adding authentication")
		req.Header.Set("Authorization", fmt.Sprintf("token %s", g.token))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.client.Do(req)
	if err != nil {
		g.logger.Error().Err(err).Str("file_path", path).Msg("Request failed")
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		g.logger.Error().Int("status_code", resp.StatusCode).Str("file_path", path).Msg("GitHub API request failed")
		return "", err
	}

	var file GitHubFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		g.logger.Error().Err(err).Str("file_path", path).Msg("Failed to decode response")
		return "", err
	}

	// GitHub returns base64-encoded content
	if file.Encoding == "base64" {
		// For simplicity, I'm storing the raw content as is.
		// might want to decode it for production use
		return file.Content, nil
	}

	return file.Content, nil
}

// createSource creates a source record in the database.
func (g *GitHubImporter) createSource(
	ctx context.Context,
	fileURL string,
	_ *GitHubRepoInfo,
	file GitHubTreeItem,
	db *sql.DB,
) (string, error) {
	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		g.logger.Error().Err(err).Str("file_path", file.Path).Msg("Failed to parse URL")
		return "", err
	}

	sourceID := uuid.New().String()
	now := time.Now().Format(time.RFC3339)

	query := `INSERT INTO sources (id, raw_url, scheme, host, path, query, active_domain, format, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// Determine format based on file extension
	ext := filepath.Ext(file.Path)
	format := formatJSON // default to json for unsupported types
	switch ext {
	case ".yaml", ".yml":
		format = "yaml"
	}

	_, err = db.ExecContext(ctx, query, sourceID, fileURL, parsedURL.Scheme, parsedURL.Host,
		parsedURL.Path, parsedURL.RawQuery, 1, format, now, now)
	if err != nil {
		g.logger.Error().Err(err).Str("file_path", file.Path).Msg("Failed to insert source")
		return "", err
	}

	return sourceID, nil
}

// createDownload creates a download record in the database.
func (g *GitHubImporter) createDownload(
	ctx context.Context,
	sourceID string,
	content string,
	file GitHubTreeItem,
	db *sql.DB,
) (string, error) {
	downloadID := uuid.New().String()
	now := time.Now().Format(time.RFC3339)

	// Create a simple headers structure
	headers := map[string][]string{
		"Content-Type": {"text/plain"},
		"X-GitHub-SHA": {file.SHA},
	}

	headersJSON, err := json.Marshal(headers)
	if err != nil {
		g.logger.Error().Err(err).Str("file_path", file.Path).Msg("Failed to marshal headers")
		return "", err
	}

	query := `INSERT INTO downloads (id, source_id, attempted_at, downloaded_at, status_code, headers, body)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err = db.ExecContext(ctx, query, downloadID, sourceID, now, now, httpOKStatus,
		string(headersJSON), content)
	if err != nil {
		g.logger.Error().Err(err).Str("file_path", file.Path).Msg("Failed to insert download")
		return "", err
	}

	return downloadID, nil
}

// SetExclusions sets the list of paths/patterns to exclude.
func (g *GitHubImporter) SetExclusions(exclusions []string) {
	g.exclusions = exclusions
}

// SetSupportedExtensions sets the list of supported file extensions.
func (g *GitHubImporter) SetSupportedExtensions(extensions []string) {
	g.supportedExts = extensions
}

// SetMaxFileSize sets the maximum file size to import.
func (g *GitHubImporter) SetMaxFileSize(size int64) {
	g.maxFileSize = size
}

// SetToken sets the GitHub API token.
func (g *GitHubImporter) SetToken(token string) {
	g.token = token
}
