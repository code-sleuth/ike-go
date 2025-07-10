package importers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const (
	// HTTP client timeout in seconds.
	defaultWPHTTPTimeout = 30
	// Default posts per page.
	defaultPerPage = 100
	// Maximum pages to fetch (safety limit).
	maxPages = 1000
	// Default concurrency for processing.
	defaultConcurrency = 5
)

var (
	ErrNotWordPressAPI          = errors.New("not a WordPress JSON API endpoint")
	ErrWPImportCompleted        = errors.New("import completed with errors")
	ErrNoPostsImported          = errors.New("no posts were successfully imported")
	ErrUnexpectedStatusCode     = errors.New("unexpected status code")
	ErrUnexpectedPostStatusCode = errors.New("unexpected status code for post")
)

// WPJSONImporter handles importing content from WordPress JSON API endpoints.
type WPJSONImporter struct {
	client      *http.Client
	perPage     int
	maxPages    int
	concurrency int
	logger      zerolog.Logger
}

// NewWPJSONImporter creates a new WordPress JSON importer.
func NewWPJSONImporter() *WPJSONImporter {
	logger := util.NewLogger(zerolog.InfoLevel)
	return &WPJSONImporter{
		client: &http.Client{
			Timeout: defaultWPHTTPTimeout * time.Second,
		},
		perPage:     defaultPerPage,
		maxPages:    maxPages,
		concurrency: defaultConcurrency,
		logger:      logger,
	}
}

// GetSourceType returns the source type this importer handles.
func (w *WPJSONImporter) GetSourceType() string {
	return "wp-json"
}

// ValidateSource checks if the source URL is valid for this importer.
func (w *WPJSONImporter) ValidateSource(sourceURL string) error {
	parsedURL, err := url.Parse(sourceURL)
	if err != nil {
		w.logger.Error().Err(err).Msg("invalid URL")
		return err
	}

	// Check if it's a WordPress JSON API endpoint
	if !strings.Contains(parsedURL.Path, "/wp-json/") {
		w.logger.Error().Err(ErrNotWordPressAPI).Msg("not a WordPress JSON API endpoint")
		return ErrNotWordPressAPI
	}

	return nil
}

// Import fetches content from a WordPress JSON API endpoint.
func (w *WPJSONImporter) Import(ctx context.Context, sourceURL string, db *sql.DB) (*interfaces.ImportResult, error) {
	if err := w.ValidateSource(sourceURL); err != nil {
		w.logger.Error().Err(err).Msg("source validation failed")
		return nil, err
	}

	w.logger.Info().Str("Starting WP-JSON import for", sourceURL)

	// Get post IDs from the endpoint
	postIDs, err := w.getPostIDs(ctx, sourceURL)
	if err != nil {
		w.logger.Error().Err(err).Msg("failed to get post IDs")
		return nil, err
	}

	w.logger.Info().Int("Found posts to import", len(postIDs))

	// Process posts concurrently
	results := make(chan *interfaces.ImportResult, len(postIDs))
	semaphore := make(chan struct{}, w.concurrency)

	for _, postID := range postIDs {
		go func(id int) {
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			result := w.importPost(ctx, sourceURL, id, db)
			results <- result
		}(postID)
	}

	// Collect results
	var errorsList []error
	var lastResult *interfaces.ImportResult

	for i := 0; i < len(postIDs); i++ {
		result := <-results
		if result.Error != nil {
			errorsList = append(errorsList, result.Error)
		} else {
			lastResult = result
		}
	}

	if len(errorsList) > 0 {
		log.Printf("Import completed with %d errorsList out of %d posts", len(errorsList), len(postIDs))
		// Return the last successful result, but include error info
		if lastResult != nil {
			w.logger.Error().Int("import completed with total errors", len(errorsList))
			w.logger.Error().Err(errorsList[0]).Msg("import completed with errors")

			lastResult.Error = ErrWPImportCompleted
		} else {
			err := fmt.Errorf("all imports failed, first error: %w", errorsList[0])
			w.logger.Err(err).Msg("all imports failed")
			return nil, err
		}
	}

	w.logger.Info().Int("WP-JSON import completed successfully for %d posts", len(postIDs)-len(errorsList))

	// Return the last successful result (all posts are imported separately)
	if lastResult != nil {
		return lastResult, nil
	}

	return nil, ErrNoPostsImported
}

// getPostIDs fetches all post IDs from the WordPress JSON API.
func (w *WPJSONImporter) getPostIDs(ctx context.Context, baseURL string) ([]int, error) {
	var allPostIDs []int
	page := 1

	for page <= w.maxPages {
		// Build URL with pagination
		reqURL := fmt.Sprintf("%s?page=%d&per_page=%d", baseURL, page, w.perPage)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			w.logger.Error().Err(err).Msg("failed to create request")
			return nil, err
		}

		resp, err := w.client.Do(req)
		if err != nil {
			w.logger.Error().Err(err).Msg("request failed")
			return nil, err
		}
		defer resp.Body.Close()

		// WordPress returns 400 when no more pages
		if resp.StatusCode == http.StatusBadRequest {
			break
		}

		if resp.StatusCode != http.StatusOK {
			w.logger.Error().Int("status code", resp.StatusCode).Msg("unexpected status code")
			return nil, err
		}

		var posts []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
			w.logger.Error().Err(err).Msg("failed to decode response")
			return nil, err
		}

		if len(posts) == 0 {
			break
		}

		// Extract post IDs
		for _, post := range posts {
			if id, ok := post["id"].(float64); ok {
				allPostIDs = append(allPostIDs, int(id))
			}
		}

		page++
	}

	return allPostIDs, nil
}

// importPost imports a single post by ID.
func (w *WPJSONImporter) importPost(
	ctx context.Context,
	baseURL string,
	postID int,
	db *sql.DB,
) *interfaces.ImportResult {
	// Build URL for individual post
	postURL := fmt.Sprintf("%s/%d", baseURL, postID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, postURL, nil)
	if err != nil {
		w.logger.Error().Err(err).Int("failed to create request for post id", postID)
		return &interfaces.ImportResult{
			Error: err,
		}
	}

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Error().Err(err).Int("request failed for post id", postID)
		return &interfaces.ImportResult{
			Error: err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.logger.Error().Int("status code for post id", postID).Int("unexpected status code", resp.StatusCode)
		return &interfaces.ImportResult{
			Error: ErrUnexpectedPostStatusCode,
		}
	}

	// Read response body
	var postData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&postData); err != nil {
		w.logger.Error().Err(err).Int("failed to decode response for post id", postID)
		return &interfaces.ImportResult{
			Error: err,
		}
	}

	// Create source record
	sourceID, err := w.createSource(ctx, postURL, db)
	if err != nil {
		w.logger.Error().Err(err).Int("failed to create source for post id", postID)
		return &interfaces.ImportResult{
			Error: err,
		}
	}

	// Create download record
	downloadID, err := w.createDownload(ctx, sourceID, resp.StatusCode, resp.Header, postData, db)
	if err != nil {
		w.logger.Error().Err(err).Int("failed to create download for post id", postID)
		return &interfaces.ImportResult{
			Error: err,
		}
	}

	return &interfaces.ImportResult{
		SourceID:   sourceID,
		DownloadID: downloadID,
	}
}

// createSource creates a source record in the database.
func (w *WPJSONImporter) createSource(ctx context.Context, postURL string, db *sql.DB) (string, error) {
	parsedURL, err := url.Parse(postURL)
	if err != nil {
		w.logger.Error().Err(err).Str("post URL", postURL).Msg("failed to parse URL")
		return "", err
	}

	sourceID := uuid.New().String()
	now := time.Now().Format(time.RFC3339)

	query := `INSERT INTO sources (id, raw_url, scheme, host, path, query, active_domain, format, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = db.ExecContext(ctx, query, sourceID, postURL, parsedURL.Scheme, parsedURL.Host,
		parsedURL.Path, parsedURL.RawQuery, 1, "json", now, now)
	if err != nil {
		w.logger.Error().Err(err).Str("post URL", postURL).Msg("failed to insert source")
		return "", err
	}

	return sourceID, nil
}

// createDownload creates a download record in the database.
func (w *WPJSONImporter) createDownload(
	ctx context.Context,
	sourceID string,
	statusCode int,
	headers http.Header,
	body map[string]interface{},
	db *sql.DB,
) (string, error) {
	downloadID := uuid.New().String()
	now := time.Now().Format(time.RFC3339)

	// Convert headers to JSON
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		w.logger.Error().Err(err).Msg("failed to marshal headers")
		return "", err
	}

	// Convert body to JSON
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		w.logger.Error().Err(err).Msg("failed to marshal body")
		return "", err
	}

	query := `INSERT INTO downloads (id, source_id, attempted_at, downloaded_at, status_code, headers, body)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err = db.ExecContext(ctx, query, downloadID, sourceID, now, now, statusCode,
		string(headersJSON), string(bodyJSON))
	if err != nil {
		w.logger.Error().Err(err).Msg("failed to insert download")
		return "", err
	}

	return downloadID, nil
}

// SetConcurrency sets the number of concurrent requests.
func (w *WPJSONImporter) SetConcurrency(concurrency int) {
	w.concurrency = concurrency
}

// SetPerPage sets the number of posts to fetch per page.
func (w *WPJSONImporter) SetPerPage(perPage int) {
	w.perPage = perPage
}

// SetMaxPages sets the maximum number of pages to fetch.
func (w *WPJSONImporter) SetMaxPages(maxPages int) {
	w.maxPages = maxPages
}

// SetTimeout sets the HTTP client timeout.
func (w *WPJSONImporter) SetTimeout(timeout time.Duration) {
	w.client.Timeout = timeout
}
