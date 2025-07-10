package embedders

import "errors"

var (
	ErrAPIKeyNotSet     = errors.New("API key not set")
	ErrUnsupportedModel = errors.New("unsupported model")
	ErrContentEmpty     = errors.New("content is empty")
	ErrAPIRequestFailed = errors.New("API request failed")
	ErrNoEmbeddingData  = errors.New("no embedding data in response")
)
