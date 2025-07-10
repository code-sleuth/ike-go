package embedders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/rs/zerolog"
)

var timeout = 30 * time.Second

// TogetherAIEmbedder implements embedding using Together AI's API.
type TogetherAIEmbedder struct {
	apiKey     string
	model      string
	dimension  int
	maxTokens  int
	httpClient *http.Client
	apiURL     string
	logger     zerolog.Logger
}

// TogetherAIEmbeddingRequest represents the request structure for Together AI embeddings API.
type TogetherAIEmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

// TogetherAIEmbeddingResponse represents the response structure from Together AI embeddings API.
type TogetherAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
		Object    string    `json:"object"`
	} `json:"data"`
	Model  string `json:"model"`
	Object string `json:"object"`
}

// NewTogetherAIEmbedder creates a new Together AI embedder.
func NewTogetherAIEmbedder(model string) (*TogetherAIEmbedder, error) {
	return NewTogetherAIEmbedderWithClient(model, nil, "")
}

// NewTogetherAIEmbedderWithClient creates a new Together AI embedder with custom HTTP client and API URL.
func NewTogetherAIEmbedderWithClient(
	model string,
	httpClient *http.Client,
	apiURL string,
) (*TogetherAIEmbedder, error) {
	logger := util.NewLogger(zerolog.ErrorLevel)
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if strings.EqualFold(apiKey, "") {
		logger.Error().Msg("TOGETHER_API_KEY env variable not set")
		return nil, ErrAPIKeyNotSet
	}

	// Set dimension and max tokens based on model
	var dimension, maxTokens int
	switch model {
	case "togethercomputer/m2-bert-80M-8k-retrieval":
		dimension = 768
		maxTokens = 8192
	case "togethercomputer/m2-bert-80M-32k-retrieval":
		dimension = 768
		maxTokens = 32768
	default:
		logger.Error().Str("unsupported model", model).Err(ErrUnsupportedModel)
		return nil, ErrUnsupportedModel
	}

	// Use provided HTTP client or create default one
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}

	// Use provided API URL or default one
	if apiURL == "" {
		apiURL = "https://api.together.xyz/v1/embeddings"
	}

	return &TogetherAIEmbedder{
		apiKey:     apiKey,
		model:      model,
		dimension:  dimension,
		maxTokens:  maxTokens,
		httpClient: httpClient,
		apiURL:     apiURL,
		logger:     logger,
	}, nil
}

// GenerateEmbedding creates a vector embedding for the given content.
func (t *TogetherAIEmbedder) GenerateEmbedding(ctx context.Context, content string) ([]float32, error) {
	if strings.EqualFold(content, "") {
		return nil, ErrContentEmpty
	}

	// Clean the content (remove newlines and extra spaces)
	cleanContent := strings.ReplaceAll(content, "\n", " ")
	cleanContent = strings.TrimSpace(cleanContent)

	// Prepare the request
	request := TogetherAIEmbeddingRequest{
		Input: cleanContent,
		Model: t.model,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		t.logger.Err(err).Msg("failed to marshal request")
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		t.apiURL,
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		t.logger.Err(err).Msg("failed to create request")
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))

	// Make the request
	resp, err := t.httpClient.Do(req)
	if err != nil {
		t.logger.Err(err).Msg("failed to make request")
		return nil, err
	}
	defer func() {
		if resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				t.logger.Error().Err(err).Msg("Failed to close response body")
			}
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.logger.Error().Int("status_code", resp.StatusCode).Msg("API request failed")
		return nil, ErrAPIRequestFailed
	}

	// Parse the response
	var response TogetherAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.logger.Err(err).Msg("failed to decode response")
		return nil, err
	}

	if len(response.Data) == 0 {
		t.logger.Warn().Msg("no embedding data in response")
		return nil, ErrNoEmbeddingData
	}

	t.logger.Debug().Str("model", t.model).Msg("Generated embedding")
	return response.Data[0].Embedding, nil
}

// GetModelName returns the name of the embedding model.
func (t *TogetherAIEmbedder) GetModelName() string {
	return t.model
}

// GetDimension returns the dimension of the embedding vectors.
func (t *TogetherAIEmbedder) GetDimension() int {
	return t.dimension
}

// GetMaxTokens returns the maximum number of tokens this embedder can handle.
func (t *TogetherAIEmbedder) GetMaxTokens() int {
	return t.maxTokens
}
