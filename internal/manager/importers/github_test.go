package importers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/code-sleuth/ike-go/internal/manager/testutil"
)

func TestNewGitHubImporter(t *testing.T) {
	importer := NewGitHubImporter()

	if importer == nil {
		t.Fatal("Expected non nil importer")
	}

	if importer.GetSourceType() != "github" {
		t.Errorf("Expected source type 'github', got %s", importer.GetSourceType())
	}
}

func TestGitHubImporter_ValidateSource(t *testing.T) {
	importer := NewGitHubImporter()

	tests := []struct {
		name        string
		sourceURL   string
		expectError bool
		expectedErr error
		description string
	}{
		{
			name:        "valid github repo URL",
			sourceURL:   "https://github.com/code-sleuth/outh",
			expectError: false,
			expectedErr: nil,
			description: "should accept valid GitHub repository URL",
		},
		{
			name:        "valid github repo with branch",
			sourceURL:   "https://github.com/code-sleuth/outh/tree/main",
			expectError: false,
			expectedErr: nil,
			description: "should accept GitHub repository URL with branch",
		},
		{
			name:        "valid github repo with feature branch",
			sourceURL:   "https://github.com/code-sleuth/outh/tree/feature/new-feature",
			expectError: false,
			expectedErr: nil,
			description: "should accept GitHub repository URL with feature branch",
		},
		{
			name:        "valid api.github.com URL",
			sourceURL:   "https://api.github.com/code-sleuth/outh/repository",
			expectError: false,
			expectedErr: nil,
			description: "should accept GitHub API URLs",
		},
		{
			name:        "invalid URL not github",
			sourceURL:   "https://gitlab.com/owner/repository",
			expectError: true,
			expectedErr: ErrNotGitHubURL,
			description: "should reject non GitHub URLs",
		},
		{
			name:        "invalid URL malformed",
			sourceURL:   "://invalid-url",
			expectError: true,
			expectedErr: nil, // URL parse error
			description: "should reject malformed URLs",
		},
		{
			name:        "invalid URL missing owner",
			sourceURL:   "https://github.com/repository",
			expectError: true,
			expectedErr: ErrInvalidGitHubURLFormat,
			description: "should reject URLs with missing owner",
		},
		{
			name:        "invalid URL - empty owner",
			sourceURL:   "https://github.com//repository",
			expectError: true,
			expectedErr: ErrInvalidGitHubURLFormat,
			description: "should reject URLs with empty owner",
		},
		{
			name:        "invalid URL - empty repository",
			sourceURL:   "https://github.com/owner/",
			expectError: true,
			expectedErr: ErrInvalidGitHubURLFormat,
			description: "should reject URLs with empty repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := importer.ValidateSource(tt.sourceURL)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if tt.expectedErr != nil && err != tt.expectedErr {
				t.Errorf("Expected error %v, got %v for test: %s", tt.expectedErr, err, tt.description)
			}
		})
	}
}

func TestGitHubImporter_ParseGitHubURL(t *testing.T) {
	importer := NewGitHubImporter()

	tests := []struct {
		name          string
		sourceURL     string
		expectError   bool
		expectedOwner string
		expectedRepo  string
		expectedRef   string
		description   string
	}{
		{
			name:          "basic github repo URL",
			sourceURL:     "https://github.com/code-sleuth/outh",
			expectError:   false,
			expectedOwner: "code-sleuth",
			expectedRepo:  "outh",
			expectedRef:   "main",
			description:   "should parse basic GitHub repository URL",
		},
		{
			name:          "github repo with main branch",
			sourceURL:     "https://github.com/code-sleuth/outh/tree/main",
			expectError:   false,
			expectedOwner: "code-sleuth",
			expectedRepo:  "outh",
			expectedRef:   "main",
			description:   "should parse GitHub repository URL with main branch",
		},
		{
			name:          "github repo with feature branch",
			sourceURL:     "https://github.com/code-sleuth/outh/tree/feature-branch",
			expectError:   false,
			expectedOwner: "code-sleuth",
			expectedRepo:  "outh",
			expectedRef:   "feature-branch",
			description:   "should parse GitHub repository URL with feature branch",
		},
		{
			name:          "github repo with tag",
			sourceURL:     "https://github.com/code-sleuth/outh/tree/v1.0.0",
			expectError:   false,
			expectedOwner: "code-sleuth",
			expectedRepo:  "outh",
			expectedRef:   "v1.0.0",
			description:   "should parse GitHub repository URL with tag",
		},
		{
			name:          "api.github.com URL",
			sourceURL:     "https://api.github.com/repos/owner/repository",
			expectError:   false,
			expectedOwner: "repos",
			expectedRepo:  "owner",
			expectedRef:   "main",
			description:   "should parse GitHub API URL",
		},
		{
			name:        "invalid URL - not github",
			sourceURL:   "https://gitlab.com/owner/repository",
			expectError: true,
			description: "should reject non-GitHub URLs",
		},
		{
			name:        "invalid URL - too few parts",
			sourceURL:   "https://github.com/owner",
			expectError: true,
			description: "should reject URLs with insufficient parts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoInfo, err := importer.parseGitHubURL(tt.sourceURL)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if !tt.expectError {
				if repoInfo.Owner != tt.expectedOwner {
					t.Errorf("Expected owner %s, got %s for test: %s", tt.expectedOwner, repoInfo.Owner, tt.description)
				}
				if repoInfo.Repo != tt.expectedRepo {
					t.Errorf("Expected repo %s, got %s for test: %s", tt.expectedRepo, repoInfo.Repo, tt.description)
				}
				if repoInfo.Ref != tt.expectedRef {
					t.Errorf("Expected ref %s, got %s for test: %s", tt.expectedRef, repoInfo.Ref, tt.description)
				}
			}
		})
	}
}

func TestGitHubImporter_GetRepoTree(t *testing.T) {
	// Create test server that simulates GitHub API
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authentication header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			t.Log("Warning: No authentication header found")
		}

		// Check accept header
		acceptHeader := r.Header.Get("Accept")
		if acceptHeader != "application/vnd.github.v3+json" {
			t.Errorf("Expected Accept header 'application/vnd.github.v3+json', got %s", acceptHeader)
		}

		// Simulate different responses based on URL
		if r.URL.Path == "/repos/owner/repo/git/trees/main" {
			// Valid repository tree
			response := GitHubTreeResponse{
				Tree: []GitHubTreeItem{
					{
						Path: "README.md",
						Mode: "100644",
						Type: "blob",
						SHA:  "abc123",
						Size: 1024,
						URL:  "https://api.github.com/repos/owner/repo/git/blobs/abc123",
					},
					{
						Path: "src/main.go",
						Mode: "100644",
						Type: "blob",
						SHA:  "def456",
						Size: 2048,
						URL:  "https://api.github.com/repos/owner/repo/git/blobs/def456",
					},
					{
						Path: "node_modules/package",
						Mode: "100644",
						Type: "blob",
						SHA:  "xyz789",
						Size: 512,
						URL:  "https://api.github.com/repos/owner/repo/git/blobs/xyz789",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/repos/notfound/repo/git/trees/main" {
			// Repository not found
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message":"Bad Request"}`))
		}
	}))
	defer testServer.Close()

	// Create importer with custom HTTP client and API base URL pointing to test server
	importer := NewGitHubImporterWithClient(&http.Client{Timeout: 5 * time.Second}, testServer.URL)
	importer.SetToken("test-token")

	tests := []struct {
		name          string
		repoInfo      *GitHubRepoInfo
		expectError   bool
		expectedFiles int
		description   string
	}{
		{
			name: "successful tree fetch",
			repoInfo: &GitHubRepoInfo{
				Owner: "owner",
				Repo:  "repo",
				Ref:   "main",
			},
			expectError:   false,
			expectedFiles: 3,
			description:   "should fetch repository tree successfully",
		},
		{
			name: "repository not found",
			repoInfo: &GitHubRepoInfo{
				Owner: "notfound",
				Repo:  "repo",
				Ref:   "main",
			},
			expectError:   true,
			expectedFiles: 0,
			description:   "should handle repository not found error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			tree, err := importer.getRepoTree(ctx, tt.repoInfo)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for test: %s", tt.description)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for test %s: %v", tt.description, err)
				return
			}

			if !tt.expectError {
				if len(tree.Tree) != tt.expectedFiles {
					t.Errorf("Expected %d files, got %d for test: %s", tt.expectedFiles, len(tree.Tree), tt.description)
				}
			}
		})
	}
}

func TestGitHubImporter_FilterFiles(t *testing.T) {
	importer := NewGitHubImporter()

	// Create test tree items
	testItems := []GitHubTreeItem{
		{
			Path: "README.md",
			Type: "blob",
			Size: 1024,
		},
		{
			Path: "src/main.go",
			Type: "blob",
			Size: 2048,
		},
		{
			Path: "src/main.py",
			Type: "blob",
			Size: 512,
		},
		{
			Path: "node_modules/package.json",
			Type: "blob",
			Size: 256,
		},
		{
			Path: ".git/config",
			Type: "blob",
			Size: 128,
		},
		{
			Path: "large_file.txt",
			Type: "blob",
			Size: 2 * 1024 * 1024, // 2MB - larger than default max
		},
		{
			Path: "image.png",
			Type: "blob",
			Size: 1024,
		},
		{
			Path: "docs/",
			Type: "tree",
			Size: 0,
		},
	}

	tests := []struct {
		name          string
		items         []GitHubTreeItem
		expectedPaths []string
		description   string
	}{
		{
			name:          "filter with default settings",
			items:         testItems,
			expectedPaths: []string{"README.md", "src/main.go", "src/main.py"},
			description:   "should filter files based on default exclusions and extensions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := importer.filterFiles(tt.items)

			if len(filtered) != len(tt.expectedPaths) {
				t.Errorf("Expected %d files, got %d for test: %s", len(tt.expectedPaths), len(filtered), tt.description)
				return
			}

			// Check that expected paths are present
			for i, expectedPath := range tt.expectedPaths {
				if filtered[i].Path != expectedPath {
					t.Errorf(
						"Expected path %s at index %d, got %s for test: %s",
						expectedPath,
						i,
						filtered[i].Path,
						tt.description,
					)
				}
			}
		})
	}
}

func TestGitHubImporter_FileFiltering(t *testing.T) {
	importer := NewGitHubImporter()
	
	t.Run("exclusion rules", func(t *testing.T) {
		tests := []struct {
			name        string
			path        string
			expected    bool
			description string
		}{
			{
				name:        "excluded .git path",
				path:        ".git/config",
				expected:    true,
				description: "should exclude .git paths",
			},
			{
				name:        "excluded node_modules path",
				path:        "node_modules/package.json",
				expected:    true,
				description: "should exclude node_modules paths",
			},
			{
				name:        "excluded __pycache__ path",
				path:        "src/__pycache__/module.pyc",
				expected:    true,
				description: "should exclude __pycache__ paths",
			},
			{
				name:        "non-excluded path",
				path:        "src/main.go",
				expected:    false,
				description: "should not exclude valid source paths",
			},
			{
				name:        "root file",
				path:        "README.md",
				expected:    false,
				description: "should not exclude root files",
			},
			{
				name:        "excluded .DS_Store",
				path:        ".DS_Store",
				expected:    true,
				description: "should exclude .DS_Store files",
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := importer.isExcluded(tt.path)
				if result != tt.expected {
					t.Errorf("Expected %v, got %v for test: %s", tt.expected, result, tt.description)
				}
			})
		}
	})
	
	t.Run("supported file extensions", func(t *testing.T) {
		tests := []struct {
			name        string
			path        string
			expected    bool
			description string
		}{
			{
				name:        "supported markdown file",
				path:        "README.md",
				expected:    true,
				description: "should support .md files",
			},
			{
				name:        "supported go file",
				path:        "main.go",
				expected:    true,
				description: "should support .go files",
			},
			{
				name:        "supported python file",
				path:        "script.py",
				expected:    true,
				description: "should support .py files",
			},
			{
				name:        "supported javascript file",
				path:        "app.js",
				expected:    true,
				description: "should support .js files",
			},
			{
				name:        "supported JSON file",
				path:        "package.json",
				expected:    true,
				description: "should support .json files",
			},
			{
				name:        "supported YAML file",
				path:        "config.yaml",
				expected:    true,
				description: "should support .yaml files",
			},
			{
				name:        "supported YML file",
				path:        "config.yml",
				expected:    true,
				description: "should support .yml files",
			},
			{
				name:        "unsupported image file",
				path:        "image.png",
				expected:    false,
				description: "should not support .png files",
			},
			{
				name:        "unsupported binary file",
				path:        "program.exe",
				expected:    false,
				description: "should not support .exe files",
			},
			{
				name:        "file without extension",
				path:        "Dockerfile",
				expected:    false,
				description: "should not support files without extensions",
			},
			{
				name:        "case insensitive extension",
				path:        "README.MD",
				expected:    true,
				description: "should support case insensitive extensions",
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := importer.isSupportedFile(tt.path)
				if result != tt.expected {
					t.Errorf("Expected %v, got %v for test: %s", tt.expected, result, tt.description)
				}
			})
		}
	})
}

func TestGitHubImporter_Configuration(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		importer := NewGitHubImporter()
		
		// Check default values
		if importer.maxFileSize != defaultMaxFileSize {
			t.Errorf("Expected maxFileSize %d, got %d", defaultMaxFileSize, importer.maxFileSize)
		}
		
		if len(importer.supportedExts) == 0 {
			t.Error("Expected non-empty supportedExts")
		}
		
		if len(importer.exclusions) == 0 {
			t.Error("Expected non-empty exclusions")
		}
		
		// Check that some expected extensions are present
		expectedExts := []string{".md", ".go", ".py", ".js"}
		for _, ext := range expectedExts {
			found := false
			for _, supportedExt := range importer.supportedExts {
				if supportedExt == ext {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected extension %s to be supported", ext)
			}
		}
		
		// Check that some expected exclusions are present
		expectedExclusions := []string{".git", "node_modules", "__pycache__"}
		for _, exclusion := range expectedExclusions {
			found := false
			for _, excl := range importer.exclusions {
				if excl == exclusion {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected exclusion %s to be present", exclusion)
			}
		}
	})
	
	t.Run("setter methods", func(t *testing.T) {
		importer := NewGitHubImporter()
		
		// Test SetExclusions
		newExclusions := []string{"custom_exclude", "another_exclude"}
		importer.SetExclusions(newExclusions)
		if len(importer.exclusions) != len(newExclusions) {
			t.Errorf("Expected %d exclusions, got %d", len(newExclusions), len(importer.exclusions))
		}
		for i, exclusion := range newExclusions {
			if importer.exclusions[i] != exclusion {
				t.Errorf("Expected exclusion %s at index %d, got %s", exclusion, i, importer.exclusions[i])
			}
		}
		
		// Test SetSupportedExtensions
		newExts := []string{".custom", ".another"}
		importer.SetSupportedExtensions(newExts)
		if len(importer.supportedExts) != len(newExts) {
			t.Errorf("Expected %d supported extensions, got %d", len(newExts), len(importer.supportedExts))
		}
		for i, ext := range newExts {
			if importer.supportedExts[i] != ext {
				t.Errorf("Expected extension %s at index %d, got %s", ext, i, importer.supportedExts[i])
			}
		}
		
		// Test SetMaxFileSize
		newMaxSize := int64(2048)
		importer.SetMaxFileSize(newMaxSize)
		if importer.maxFileSize != newMaxSize {
			t.Errorf("Expected max file size %d, got %d", newMaxSize, importer.maxFileSize)
		}
		
		// Test SetToken
		newToken := "test-token-123"
		importer.SetToken(newToken)
		if importer.token != newToken {
			t.Errorf("Expected token %s, got %s", newToken, importer.token)
		}
	})
}

func TestGitHubImporter_GitHubTokenEnvironment(t *testing.T) {
	// Load environment variables from .env file
	err := testutil.LoadEnvFromFile("../../../.env")
	if err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Save original environment variable
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer os.Setenv("GITHUB_TOKEN", originalToken)

	// Test with token set
	testToken := "test-github-token"
	os.Setenv("GITHUB_TOKEN", testToken)

	importer := NewGitHubImporter()
	if importer.token != testToken {
		t.Errorf("Expected token from environment %s, got %s", testToken, importer.token)
	}

	// Test with no token
	os.Unsetenv("GITHUB_TOKEN")
	importer2 := NewGitHubImporter()
	if importer2.token != "" {
		t.Errorf("Expected empty token when not set in environment, got %s", importer2.token)
	}
}


// Benchmark tests
func BenchmarkGitHubImporter_ValidateSource(b *testing.B) {
	importer := NewGitHubImporter()
	sourceURL := "https://github.com/code-sleuth/outh"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		importer.ValidateSource(sourceURL)
	}
}

func BenchmarkGitHubImporter_ParseGitHubURL(b *testing.B) {
	importer := NewGitHubImporter()
	sourceURL := "https://github.com/code-sleuth/outh/tree/main"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		importer.parseGitHubURL(sourceURL)
	}
}

func BenchmarkGitHubImporter_FilterFiles(b *testing.B) {
	importer := NewGitHubImporter()

	// Create a large slice of test items
	items := make([]GitHubTreeItem, 1000)
	for i := range items {
		items[i] = GitHubTreeItem{
			Path: "src/file.go",
			Type: "blob",
			Size: 1024,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		importer.filterFiles(items)
	}
}

func BenchmarkNewGitHubImporter(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewGitHubImporter()
	}
}
