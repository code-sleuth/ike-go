package embedders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/rs/zerolog"
)

// OpenAIEmbedder implements embedding using OpenAI's API.
type OpenAIEmbedder struct {
	apiKey     string
	model      string
	dimension  int
	maxTokens  int
	httpClient *http.Client
	apiURL     string
	logger     zerolog.Logger
}

// OpenAIEmbeddingRequest represents the request structure for OpenAI embeddings API.
type OpenAIEmbeddingRequest struct {
	Input          string `json:"input"`
	Model          string `json:"model"`
	EncodingFormat string `json:"encoding_format"`
}

// OpenAIEmbeddingResponse represents the response structure from OpenAI embeddings API.
type OpenAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
		Object    string    `json:"object"`
	} `json:"data"`
	Model  string `json:"model"`
	Object string `json:"object"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// NewOpenAIEmbedder creates a new OpenAI embedder.
func NewOpenAIEmbedder(model string) (*OpenAIEmbedder, error) {
	return NewOpenAIEmbedderWithClient(model, nil, "")
}

// NewOpenAIEmbedderWithClient creates a new OpenAI embedder with custom HTTP client and API URL.
func NewOpenAIEmbedderWithClient(model string, httpClient *http.Client, apiURL string) (*OpenAIEmbedder, error) {
	logger := util.NewLogger(zerolog.ErrorLevel)
	apiKey := os.Getenv("OPENAI_API_KEY")
	if strings.EqualFold(apiKey, "") {
		logger.Error().Msg("OPENAI_API_KEY env variable not set")
		return nil, ErrAPIKeyNotSet
	}

	// Set dimension and max tokens based on model
	var dimension, maxTokens int
	switch model {
	case "text-embedding-3-small":
		dimension = 1536
		maxTokens = 8191
	case "text-embedding-3-large":
		dimension = 3072
		maxTokens = 8191
	case "text-embedding-ada-002":
		dimension = 1536
		maxTokens = 8191
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
		apiURL = "https://api.openai.com/v1/embeddings"
	}

	return &OpenAIEmbedder{
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
func (o *OpenAIEmbedder) GenerateEmbedding(ctx context.Context, content string) ([]float32, error) {
	if strings.EqualFold(content, "") {
		o.logger.Warn().Msg("content is empty")
		return nil, ErrContentEmpty
	}

	// Clean the content (remove newlines and extra spaces)
	cleanContent := strings.ReplaceAll(content, "\n", " ")
	cleanContent = strings.TrimSpace(cleanContent)

	// Prepare the request
	request := OpenAIEmbeddingRequest{
		Input:          cleanContent,
		Model:          o.model,
		EncodingFormat: "float",
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		o.logger.Err(err).Msg("failed to marshal request")
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		o.apiURL,
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		o.logger.Err(err).Msg("failed to create request")
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.apiKey))

	// Make the request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		o.logger.Err(err).Msg("failed to make request")
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			o.logger.Error().Err(err).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		o.logger.Error().Int("status_code", resp.StatusCode).Msg("API request failed")
		return nil, ErrAPIRequestFailed
	}

	// Parse the response
	var response OpenAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		o.logger.Err(err).Msg("failed to decode response")
		return nil, err
	}

	if len(response.Data) == 0 {
		return nil, ErrNoEmbeddingData
	}

	o.logger.Debug().Str("model", o.model).Int("tokens_used", response.Usage.TotalTokens).Msg("Generated embedding")
	return response.Data[0].Embedding, nil
}

// GetModelName returns the name of the embedding model.
func (o *OpenAIEmbedder) GetModelName() string {
	return o.model
}

// GetDimension returns the dimension of the embedding vectors.
func (o *OpenAIEmbedder) GetDimension() int {
	return o.dimension
}

// GetMaxTokens returns the maximum number of tokens this embedder can handle.
func (o *OpenAIEmbedder) GetMaxTokens() int {
	return o.maxTokens
}
