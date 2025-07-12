package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/code-sleuth/ike-go/internal/manager/interfaces"
	"github.com/code-sleuth/ike-go/internal/manager/models"
)

// Mock implementations for testing

type mockImporter struct {
	sourceType    string
	importResult  *interfaces.ImportResult
	importError   error
	validateError error
}

func (m *mockImporter) Import(ctx context.Context, sourceURL string, db *sql.DB) (*interfaces.ImportResult, error) {
	return m.importResult, m.importError
}

func (m *mockImporter) GetSourceType() string {
	return m.sourceType
}

func (m *mockImporter) ValidateSource(sourceURL string) error {
	return m.validateError
}

type mockTransformer struct {
	sourceType      string
	transformResult *interfaces.TransformResult
	transformError  error
	canTransform    bool
}

func (m *mockTransformer) Transform(
	ctx context.Context,
	download *models.Download,
	db *sql.DB,
) (*interfaces.TransformResult, error) {
	return m.transformResult, m.transformError
}

func (m *mockTransformer) GetSourceType() string {
	return m.sourceType
}

func (m *mockTransformer) CanTransform(download *models.Download) bool {
	return m.canTransform
}

type mockChunker struct {
	strategy   string
	chunks     []*models.Chunk
	chunkError error
}

func (m *mockChunker) ChunkDocument(content string, maxTokens int) ([]*models.Chunk, error) {
	return m.chunks, m.chunkError
}

func (m *mockChunker) GetChunkingStrategy() string {
	return m.strategy
}

type mockEmbedder struct {
	modelName  string
	dimension  int
	maxTokens  int
	embedding  []float32
	embedError error
}

func (m *mockEmbedder) GenerateEmbedding(ctx context.Context, content string) ([]float32, error) {
	return m.embedding, m.embedError
}

func (m *mockEmbedder) GetModelName() string {
	return m.modelName
}

func (m *mockEmbedder) GetDimension() int {
	return m.dimension
}

func (m *mockEmbedder) GetMaxTokens() int {
	return m.maxTokens
}

type mockUpdater struct {
	sourceType string
}

func (m *mockUpdater) CheckForUpdates(ctx context.Context, db *sql.DB) ([]*interfaces.UpdateResult, error) {
	return nil, nil
}

func (m *mockUpdater) UpdateSource(ctx context.Context, sourceID string, db *sql.DB) (*interfaces.UpdateResult, error) {
	return nil, nil
}

func (m *mockUpdater) GetSourceType() string {
	return m.sourceType
}

// Test NewProcessingEngine
func TestNewProcessingEngine(t *testing.T) {
	engine := NewProcessingEngine()

	if engine == nil {
		t.Fatal("Expected non-nil processing engine")
	}

	if engine.importers == nil {
		t.Error("Expected importers map to be initialized")
	}
	if engine.transformers == nil {
		t.Error("Expected transformers map to be initialized")
	}
	if engine.chunkers == nil {
		t.Error("Expected chunkers map to be initialized")
	}
	if engine.embedders == nil {
		t.Error("Expected embedders map to be initialized")
	}
	if engine.updaters == nil {
		t.Error("Expected updaters map to be initialized")
	}
}

// Test RegisterImporter
func TestProcessingEngine_RegisterImporter(t *testing.T) {
	tests := []struct {
		name          string
		sourceType    string
		expectError   bool
		expectedError error
		description   string
	}{
		{
			name:        "successful registration",
			sourceType:  "github",
			expectError: false,
			description: "should register importer successfully",
		},
		{
			name:          "duplicate registration",
			sourceType:    "github",
			expectError:   true,
			expectedError: ErrImporterAlreadyRegistered,
			description:   "should fail when registering duplicate importer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewProcessingEngine()

			// Register first importer
			importer1 := &mockImporter{sourceType: "github"}
			err := engine.RegisterImporter(importer1)
			if err != nil {
				t.Fatalf("Failed to register first importer: %v", err)
			}

			// For duplicate test, try to register second importer with same type
			if tt.name == "duplicate registration" {
				importer2 := &mockImporter{sourceType: "github"}
				err = engine.RegisterImporter(importer2)
			}

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if tt.expectError && err != tt.expectedError {
				t.Errorf("Expected error %v, got %v", tt.expectedError, err)
			}
		})
	}
}

// Test RegisterTransformer
func TestProcessingEngine_RegisterTransformer(t *testing.T) {
	tests := []struct {
		name          string
		sourceType    string
		expectError   bool
		expectedError error
		description   string
	}{
		{
			name:        "successful registration",
			sourceType:  "wp-json",
			expectError: false,
			description: "should register transformer successfully",
		},
		{
			name:          "duplicate registration",
			sourceType:    "wp-json",
			expectError:   true,
			expectedError: ErrTransformerAlreadyRegistered,
			description:   "should fail when registering duplicate transformer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewProcessingEngine()

			// Register first transformer
			transformer1 := &mockTransformer{sourceType: "wp-json"}
			err := engine.RegisterTransformer(transformer1)
			if err != nil {
				t.Fatalf("Failed to register first transformer: %v", err)
			}

			// For duplicate test, try to register second transformer with same type
			if tt.name == "duplicate registration" {
				transformer2 := &mockTransformer{sourceType: "wp-json"}
				err = engine.RegisterTransformer(transformer2)
			}

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if tt.expectError && err != tt.expectedError {
				t.Errorf("Expected error %v, got %v", tt.expectedError, err)
			}
		})
	}
}

// Test RegisterChunker
func TestProcessingEngine_RegisterChunker(t *testing.T) {
	tests := []struct {
		name          string
		strategy      string
		expectError   bool
		expectedError error
		description   string
	}{
		{
			name:        "successful registration",
			strategy:    "token",
			expectError: false,
			description: "should register chunker successfully",
		},
		{
			name:          "duplicate registration",
			strategy:      "token",
			expectError:   true,
			expectedError: ErrChunkerAlreadyRegistered,
			description:   "should fail when registering duplicate chunker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewProcessingEngine()

			// Register first chunker
			chunker1 := &mockChunker{strategy: "token"}
			err := engine.RegisterChunker(chunker1)
			if err != nil {
				t.Fatalf("Failed to register first chunker: %v", err)
			}

			// For duplicate test, try to register second chunker with same strategy
			if tt.name == "duplicate registration" {
				chunker2 := &mockChunker{strategy: "token"}
				err = engine.RegisterChunker(chunker2)
			}

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if tt.expectError && err != tt.expectedError {
				t.Errorf("Expected error %v, got %v", tt.expectedError, err)
			}
		})
	}
}

// Test RegisterEmbedder
func TestProcessingEngine_RegisterEmbedder(t *testing.T) {
	tests := []struct {
		name          string
		modelName     string
		expectError   bool
		expectedError error
		description   string
	}{
		{
			name:        "successful registration",
			modelName:   "text-embedding-ada-002",
			expectError: false,
			description: "should register embedder successfully",
		},
		{
			name:          "duplicate registration",
			modelName:     "text-embedding-ada-002",
			expectError:   true,
			expectedError: ErrEmbedderAlreadyRegistered,
			description:   "should fail when registering duplicate embedder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewProcessingEngine()

			// Register first embedder
			embedder1 := &mockEmbedder{modelName: "text-embedding-ada-002"}
			err := engine.RegisterEmbedder(embedder1)
			if err != nil {
				t.Fatalf("Failed to register first embedder: %v", err)
			}

			// For duplicate test, try to register second embedder with same model
			if tt.name == "duplicate registration" {
				embedder2 := &mockEmbedder{modelName: "text-embedding-ada-002"}
				err = engine.RegisterEmbedder(embedder2)
			}

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if tt.expectError && err != tt.expectedError {
				t.Errorf("Expected error %v, got %v", tt.expectedError, err)
			}
		})
	}
}

// Test RegisterUpdater
func TestProcessingEngine_RegisterUpdater(t *testing.T) {
	tests := []struct {
		name          string
		sourceType    string
		expectError   bool
		expectedError error
		description   string
	}{
		{
			name:        "successful registration",
			sourceType:  "github",
			expectError: false,
			description: "should register updater successfully",
		},
		{
			name:          "duplicate registration",
			sourceType:    "github",
			expectError:   true,
			expectedError: ErrUpdaterAlreadyRegistered,
			description:   "should fail when registering duplicate updater",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewProcessingEngine()

			// Register first updater
			updater1 := &mockUpdater{sourceType: "github"}
			err := engine.RegisterUpdater(updater1)
			if err != nil {
				t.Fatalf("Failed to register first updater: %v", err)
			}

			// For duplicate test, try to register second updater with same type
			if tt.name == "duplicate registration" {
				updater2 := &mockUpdater{sourceType: "github"}
				err = engine.RegisterUpdater(updater2)
			}

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if tt.expectError && err != tt.expectedError {
				t.Errorf("Expected error %v, got %v", tt.expectedError, err)
			}
		})
	}
}

// Test determineSourceType
func TestProcessingEngine_determineSourceType(t *testing.T) {
	tests := []struct {
		name          string
		sourceURL     string
		setupImporter func(engine *ProcessingEngine)
		expectError   bool
		expectedType  string
		description   string
	}{
		{
			name:      "GitHub URL detection",
			sourceURL: "https://github.com/owner/repo",
			setupImporter: func(engine *ProcessingEngine) {
				importer := &mockImporter{
					sourceType:    "github",
					validateError: nil,
				}
				engine.RegisterImporter(importer)
			},
			expectError:  false,
			expectedType: "github",
			description:  "should detect GitHub source type",
		},
		{
			name:      "WordPress URL detection",
			sourceURL: "https://example.com/wp-json/wp/v2/posts",
			setupImporter: func(engine *ProcessingEngine) {
				importer := &mockImporter{
					sourceType:    "wp-json",
					validateError: nil,
				}
				engine.RegisterImporter(importer)
			},
			expectError:  false,
			expectedType: "wp-json",
			description:  "should detect WordPress JSON source type",
		},
		{
			name:      "unsupported URL",
			sourceURL: "https://unsupported.com/content",
			setupImporter: func(engine *ProcessingEngine) {
				importer := &mockImporter{
					sourceType:    "github",
					validateError: errors.New("unsupported URL"),
				}
				engine.RegisterImporter(importer)
			},
			expectError: true,
			description: "should fail for unsupported URLs",
		},
		{
			name:      "no importers registered",
			sourceURL: "https://example.com",
			setupImporter: func(engine *ProcessingEngine) {
				// No importers registered
			},
			expectError: true,
			description: "should fail when no importers are registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewProcessingEngine()
			tt.setupImporter(engine)

			sourceType, err := engine.determineSourceType(tt.sourceURL)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if !tt.expectError && sourceType != tt.expectedType {
				t.Errorf("Expected source type %s, got %s", tt.expectedType, sourceType)
			}
		})
	}
}

// Test determineSourceTypeFromSource
func TestProcessingEngine_determineSourceTypeFromSource(t *testing.T) {
	githubHost := "github.com"
	apiGithubHost := "api.github.com"
	wordpressHost := "example.com"
	sourceURL := "https://example.com/test"

	tests := []struct {
		name         string
		source       *models.Source
		expectedType string
		expectError  bool
		description  string
	}{
		{
			name: "GitHub host detection",
			source: &models.Source{
				Host: &githubHost,
			},
			expectedType: "github",
			expectError:  false,
			description:  "should detect GitHub from host",
		},
		{
			name: "GitHub API host detection",
			source: &models.Source{
				Host: &apiGithubHost,
			},
			expectedType: "github",
			expectError:  false,
			description:  "should detect GitHub from API host",
		},
		{
			name: "WordPress host detection",
			source: &models.Source{
				Host: &wordpressHost,
			},
			expectedType: "wp-json",
			expectError:  false,
			description:  "should default to wp-json for other hosts",
		},
		{
			name: "no host with raw URL",
			source: &models.Source{
				Host:   nil,
				RawURL: &sourceURL,
			},
			expectError: true,
			description: "should fail when no host is available",
		},
		{
			name: "no host and no raw URL",
			source: &models.Source{
				Host:   nil,
				RawURL: nil,
			},
			expectError: true,
			description: "should fail when neither host nor raw URL are available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewProcessingEngine()

			sourceType, err := engine.determineSourceTypeFromSource(tt.source)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
			}
			if !tt.expectError && sourceType != tt.expectedType {
				t.Errorf("Expected source type %s, got %s", tt.expectedType, sourceType)
			}
		})
	}
}

// Test concurrent registration safety
func TestProcessingEngine_ConcurrentRegistration(t *testing.T) {
	engine := NewProcessingEngine()

	// Test concurrent importer registration
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			importer := &mockImporter{
				sourceType: "test-type",
			}
			err := engine.RegisterImporter(importer)
			// Only one should succeed, others should get ErrImporterAlreadyRegistered
			if err != nil && err != ErrImporterAlreadyRegistered {
				t.Errorf("Unexpected error: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify only one importer was registered
	engine.mu.RLock()
	if len(engine.importers) != 1 {
		t.Errorf("Expected 1 importer, got %d", len(engine.importers))
	}
	engine.mu.RUnlock()
}
