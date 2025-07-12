package transformers

import (
	"path/filepath"
	"testing"

	"github.com/code-sleuth/ike-go/internal/manager/models"
)

func TestNewGitHubTransformer(t *testing.T) {
	transformer := NewGitHubTransformer()

	if transformer == nil {
		t.Fatal("Expected non-nil transformer")
	}

	if transformer.GetSourceType() != "github" {
		t.Errorf("Expected source type 'github', got %s", transformer.GetSourceType())
	}
}

func TestGitHubTransformer_CanTransform(t *testing.T) {
	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		download    *models.Download
		expected    bool
		description string
	}{
		{
			name: "valid github download with SHA header",
			download: &models.Download{
				Headers: `{"X-GitHub-SHA": ["abc123def456"], "Content-Type": ["application/json"]}`,
				Body:    stringPtr("file content"),
			},
			expected:    true,
			description: "should return true for download with GitHub SHA header",
		},
		{
			name: "download without GitHub headers",
			download: &models.Download{
				Headers: `{"Content-Type": ["application/json"], "Server": ["nginx"]}`,
				Body:    stringPtr("file content"),
			},
			expected:    false,
			description: "should return false for download without GitHub headers",
		},
		{
			name: "nil body",
			download: &models.Download{
				Headers: `{"X-GitHub-SHA": ["abc123def456"]}`,
				Body:    nil,
			},
			expected:    false,
			description: "should return false for nil body",
		},
		{
			name: "invalid headers JSON",
			download: &models.Download{
				Headers: `invalid json`,
				Body:    stringPtr("file content"),
			},
			expected:    false,
			description: "should return false for invalid headers JSON",
		},
		{
			name: "empty headers",
			download: &models.Download{
				Headers: `{}`,
				Body:    stringPtr("file content"),
			},
			expected:    false,
			description: "should return false for empty headers",
		},
		{
			name: "headers with different GitHub header format",
			download: &models.Download{
				Headers: `{"x-github-sha": ["abc123def456"]}`,
				Body:    stringPtr("file content"),
			},
			expected:    false,
			description: "should return false for case-sensitive header mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.CanTransform(tt.download)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for test: %s", tt.expected, result, tt.description)
			}
		})
	}
}

func TestGitHubTransformer_ExtractFilePath(t *testing.T) {
	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		rawURL      *string
		expected    string
		description string
	}{
		{
			name:        "valid github blob URL",
			rawURL:      stringPtr("https://github.com/code-sleuth/outh/blob/main/src/main.go"),
			expected:    "src/main.go",
			description: "should extract file path from blob URL",
		},
		{
			name:        "github blob URL with nested path",
			rawURL:      stringPtr("https://github.com/code-sleuth/outh/blob/main/docs/api/readme.md"),
			expected:    "docs/api/readme.md",
			description: "should extract nested file path",
		},
		{
			name:        "github blob URL with branch name containing slashes",
			rawURL:      stringPtr("https://github.com/code-sleuth/outh/blob/feature/new-feature/file.txt"),
			expected:    "new-feature/file.txt",
			description: "should handle branch names with slashes",
		},
		{
			name:        "invalid URL - too few parts",
			rawURL:      stringPtr("https://github.com/owner"),
			expected:    "",
			description: "should return empty string for invalid URL",
		},
		{
			name:        "URL without blob",
			rawURL:      stringPtr("https://github.com/code-sleuth/outh/tree/main/src"),
			expected:    "",
			description: "should return empty string for non-blob URL",
		},
		{
			name:        "nil URL",
			rawURL:      nil,
			expected:    "",
			description: "should return empty string for nil URL",
		},
		{
			name:        "empty URL",
			rawURL:      stringPtr(""),
			expected:    "",
			description: "should return empty string for empty URL",
		},
		{
			name:        "root file",
			rawURL:      stringPtr("https://github.com/code-sleuth/outh/blob/main/README.md"),
			expected:    "README.md",
			description: "should extract root file path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.extractFilePath(tt.rawURL)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s' for test: %s", tt.expected, result, tt.description)
			}
		})
	}
}

func TestGitHubTransformer_ProcessContent(t *testing.T) {
	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		body        string
		filePath    string
		expected    string
		description string
	}{
		{
			name:        "markdown file",
			body:        "# Hello World\nThis is a **markdown** file.",
			filePath:    "README.md",
			expected:    "# Hello World\nThis is a **markdown** file.",
			description: "should return markdown as-is",
		},
		{
			name:        "go source file",
			body:        "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			filePath:    "main.go",
			expected:    "```go\npackage main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```",
			description: "should wrap Go code in code blocks",
		},
		{
			name:        "python file",
			body:        "def hello():\n    print(\"Hello, World!\")",
			filePath:    "script.py",
			expected:    "```python\ndef hello():\n    print(\"Hello, World!\")\n```",
			description: "should wrap Python code in code blocks",
		},
		{
			name:        "javascript file",
			body:        "function hello() {\n  console.log('Hello');\n}",
			filePath:    "script.js",
			expected:    "```javascript\nfunction hello() {\n  console.log('Hello');\n}\n```",
			description: "should wrap JavaScript code in code blocks",
		},
		{
			name:        "json file",
			body:        "{\n  \"name\": \"test\",\n  \"version\": \"1.0.0\"\n}",
			filePath:    "package.json",
			expected:    "```json\n{\n  \"name\": \"test\",\n  \"version\": \"1.0.0\"\n}\n```",
			description: "should wrap JSON in code blocks",
		},
		{
			name:        "html file",
			body:        "<h1>Title</h1>\n<p>This is <strong>bold</strong> text.</p>",
			filePath:    "index.html",
			expected:    "# Title\n\nThis is **bold** text.",
			description: "should convert HTML to markdown",
		},
		{
			name:        "text file",
			body:        "This is plain text content.",
			filePath:    "notes.txt",
			expected:    "```\nThis is plain text content.\n```",
			description: "should wrap text files in code blocks",
		},
		{
			name:        "unknown extension",
			body:        "Some content with unknown extension.",
			filePath:    "file.xyz",
			expected:    "Some content with unknown extension.",
			description: "should return content as-is for unknown extensions",
		},
		{
			name:        "base64 encoded content",
			body:        "SGVsbG8gV29ybGQ=", // "Hello World" in base64 - but doesn't contain "base64" keyword
			filePath:    "test.txt",
			expected:    "```\nSGVsbG8gV29ybGQ=\n```",
			description: "should handle base64-like content without decoding",
		},
		{
			name:        "cpp file",
			body:        "#include <iostream>\nint main() {\n    std::cout << \"Hello\" << std::endl;\n}",
			filePath:    "main.cpp",
			expected:    "```cpp\n#include <iostream>\nint main() {\n    std::cout << \"Hello\" << std::endl;\n}\n```",
			description: "should wrap C++ code in code blocks",
		},
		{
			name:        "yaml file",
			body:        "name: test\nversion: 1.0\ndependencies:\n  - package1\n  - package2",
			filePath:    "config.yaml",
			expected:    "```yaml\nname: test\nversion: 1.0\ndependencies:\n  - package1\n  - package2\n```",
			description: "should wrap YAML in code blocks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.processContent(tt.body, tt.filePath)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s\nFor test: %s", tt.expected, result, tt.description)
			}
		})
	}
}

func TestGitHubTransformer_GetLanguageFromExtension(t *testing.T) {
	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		extension   string
		expected    string
		description string
	}{
		{
			name:        "go extension",
			extension:   ".go",
			expected:    "go",
			description: "should return 'go' for .go files",
		},
		{
			name:        "python extension",
			extension:   ".py",
			expected:    "python",
			description: "should return 'python' for .py files",
		},
		{
			name:        "javascript extension",
			extension:   ".js",
			expected:    "javascript",
			description: "should return 'javascript' for .js files",
		},
		{
			name:        "typescript extension",
			extension:   ".ts",
			expected:    "typescript",
			description: "should return 'typescript' for .ts files",
		},
		{
			name:        "cpp extension",
			extension:   ".cpp",
			expected:    "cpp",
			description: "should return 'cpp' for .cpp files",
		},
		{
			name:        "c extension",
			extension:   ".c",
			expected:    "c",
			description: "should return 'c' for .c files",
		},
		{
			name:        "header extension",
			extension:   ".h",
			expected:    "c",
			description: "should return 'c' for .h files",
		},
		{
			name:        "hpp extension",
			extension:   ".hpp",
			expected:    "cpp",
			description: "should return 'cpp' for .hpp files",
		},
		{
			name:        "java extension",
			extension:   ".java",
			expected:    "java",
			description: "should return 'java' for .java files",
		},
		{
			name:        "json extension",
			extension:   ".json",
			expected:    "json",
			description: "should return 'json' for .json files",
		},
		{
			name:        "yaml extension",
			extension:   ".yaml",
			expected:    "yaml",
			description: "should return 'yaml' for .yaml files",
		},
		{
			name:        "yml extension",
			extension:   ".yml",
			expected:    "yaml",
			description: "should return 'yaml' for .yml files",
		},
		{
			name:        "unknown extension",
			extension:   ".xyz",
			expected:    "",
			description: "should return empty string for unknown extensions",
		},
		{
			name:        "no extension",
			extension:   "",
			expected:    "",
			description: "should return empty string for no extension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.getLanguageFromExtension(tt.extension)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s' for test: %s", tt.expected, result, tt.description)
			}
		})
	}
}

func TestGitHubTransformer_CreateDocument(t *testing.T) {
	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		download    *models.Download
		filePath    string
		description string
	}{
		{
			name: "markdown file document",
			download: &models.Download{
				ID:       "download-123",
				SourceID: "source-123",
			},
			filePath:    "README.md",
			description: "should create document for markdown file",
		},
		{
			name: "json file document",
			download: &models.Download{
				ID:       "download-456",
				SourceID: "source-456",
			},
			filePath:    "package.json",
			description: "should create document for JSON file",
		},
		{
			name: "yaml file document",
			download: &models.Download{
				ID:       "download-789",
				SourceID: "source-789",
			},
			filePath:    "config.yaml",
			description: "should create document for YAML file",
		},
		{
			name: "code file document",
			download: &models.Download{
				ID:       "download-999",
				SourceID: "source-999",
			},
			filePath:    "main.go",
			description: "should create document for code file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := transformer.createDocument(tt.download, tt.filePath)

			if document == nil {
				t.Errorf("Expected non-nil document for test: %s", tt.description)
				return
			}

			if document.SourceID != tt.download.SourceID {
				t.Errorf("Expected source ID %s, got %s for test: %s",
					tt.download.SourceID, document.SourceID, tt.description)
			}

			if document.DownloadID != tt.download.ID {
				t.Errorf("Expected download ID %s, got %s for test: %s",
					tt.download.ID, document.DownloadID, tt.description)
			}

			if document.ID == "" {
				t.Errorf("Expected non-empty document ID for test: %s", tt.description)
			}

			if document.MinChunkSize != 212 {
				t.Errorf("Expected min chunk size 212, got %d for test: %s",
					document.MinChunkSize, tt.description)
			}

			if document.MaxChunkSize != 8191 {
				t.Errorf("Expected max chunk size 8191, got %d for test: %s",
					document.MaxChunkSize, tt.description)
			}

			if document.IndexedAt == nil {
				t.Errorf("Expected non-nil IndexedAt for test: %s", tt.description)
			}

			// Check format based on file extension
			extension := filepath.Ext(tt.filePath)
			expectedFormat := "json"
			switch extension {
			case ".json":
				expectedFormat = "json"
			case ".yaml", ".yml":
				expectedFormat = "yaml"
			}

			if document.Format == nil {
				t.Errorf("Expected non-nil format for test: %s", tt.description)
			} else if *document.Format != expectedFormat {
				t.Errorf("Expected format %s, got %s for test: %s",
					expectedFormat, *document.Format, tt.description)
			}
		})
	}
}

func TestGitHubTransformer_DetectLanguage(t *testing.T) {
	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		content     string
		filePath    string
		expected    string
		description string
	}{
		{
			name:        "go code file",
			content:     "package main\nfunc main() {}",
			filePath:    "main.go",
			expected:    "",
			description: "should return empty string for Go code files (not in allowed list)",
		},
		{
			name:        "python code file",
			content:     "def hello():\n    print('world')",
			filePath:    "script.py",
			expected:    "python",
			description: "should return 'python' for Python code files",
		},
		{
			name:        "javascript code file",
			content:     "function test() { return true; }",
			filePath:    "app.js",
			expected:    "javascript",
			description: "should return 'javascript' for JavaScript code files",
		},
		{
			name:        "markdown file with english",
			content:     "This is an English document with common words.",
			filePath:    "README.md",
			expected:    "en",
			description: "should detect English in markdown files",
		},
		{
			name:        "markdown file with french",
			content:     "Ceci est un document français avec les mots français comme le, la, les, et, dans, avec.",
			filePath:    "README.md",
			expected:    "fr",
			description: "should detect French in markdown files",
		},
		{
			name:        "text file with english",
			content:     "This is a regular English text file.",
			filePath:    "notes.txt",
			expected:    "en",
			description: "should detect English in text files",
		},
		{
			name:        "unknown extension",
			content:     "Some content here.",
			filePath:    "file.xyz",
			expected:    "en",
			description: "should default to English for unknown file types",
		},
		{
			name:        "empty content",
			content:     "",
			filePath:    "empty.txt",
			expected:    "en",
			description: "should default to English for empty content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.detectLanguage(tt.content, tt.filePath)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s' for test: %s", tt.expected, result, tt.description)
			}
		})
	}
}

func TestGitHubTransformer_IsCodeFile(t *testing.T) {
	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		extension   string
		expected    bool
		description string
	}{
		{
			name:        "go file",
			extension:   ".go",
			expected:    true,
			description: "should return true for .go files",
		},
		{
			name:        "python file",
			extension:   ".py",
			expected:    true,
			description: "should return true for .py files",
		},
		{
			name:        "javascript file",
			extension:   ".js",
			expected:    true,
			description: "should return true for .js files",
		},
		{
			name:        "markdown file",
			extension:   ".md",
			expected:    false,
			description: "should return false for .md files",
		},
		{
			name:        "text file",
			extension:   ".txt",
			expected:    false,
			description: "should return false for .txt files",
		},
		{
			name:        "json file",
			extension:   ".json",
			expected:    true,
			description: "should return true for .json files",
		},
		{
			name:        "yaml file",
			extension:   ".yaml",
			expected:    true,
			description: "should return true for .yaml files",
		},
		{
			name:        "unknown extension",
			extension:   ".xyz",
			expected:    false,
			description: "should return false for unknown extensions",
		},
		{
			name:        "no extension",
			extension:   "",
			expected:    false,
			description: "should return false for no extension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.isCodeFile(tt.extension)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for test: %s", tt.expected, result, tt.description)
			}
		})
	}
}

func TestGitHubTransformer_ExtractMetadata(t *testing.T) {
	transformer := NewGitHubTransformer()

	// Create a mock source
	source := &models.Source{
		RawURL: stringPtr("https://github.com/code-sleuth/outh/blob/main/src/main.go"),
	}

	tests := []struct {
		name        string
		filePath    string
		content     string
		description string
	}{
		{
			name:        "go source file",
			filePath:    "src/main.go",
			content:     "package main\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			description: "should extract metadata for Go source file",
		},
		{
			name:        "markdown documentation",
			filePath:    "docs/README.md",
			content:     "# Project Title\n\nThis is documentation.",
			description: "should extract metadata for markdown file",
		},
		{
			name:        "nested file",
			filePath:    "pkg/utils/helper.go",
			content:     "package utils\n\nfunc Helper() {}",
			description: "should extract metadata for nested file",
		},
		{
			name:        "root file",
			filePath:    "LICENSE",
			content:     "MIT License\n\nCopyright (c) 2023",
			description: "should extract metadata for root file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := transformer.extractMetadata(source, tt.filePath, tt.content)

			if metadata == nil {
				t.Errorf("Expected non-nil metadata for test: %s", tt.description)
				return
			}

			// Check required fields
			if filePath, exists := metadata["file_path"]; !exists || filePath != tt.filePath {
				t.Errorf("Expected file_path %s, got %v for test: %s", tt.filePath, filePath, tt.description)
			}

			if fileName, exists := metadata["file_name"]; !exists || fileName == "" {
				t.Errorf("Expected non-empty file_name for test: %s", tt.description)
			}

			if _, exists := metadata["file_extension"]; !exists {
				t.Errorf("Expected file_extension to exist for test: %s", tt.description)
			}

			if fileSize, exists := metadata["file_size"]; !exists || fileSize != len(tt.content) {
				t.Errorf("Expected file_size %d, got %v for test: %s", len(tt.content), fileSize, tt.description)
			}

			if lineCount, exists := metadata["line_count"]; !exists || lineCount.(int) < 1 {
				t.Errorf("Expected line_count >= 1, got %v for test: %s", lineCount, tt.description)
			}

			if charCount, exists := metadata["character_count"]; !exists || charCount != len(tt.content) {
				t.Errorf("Expected character_count %d, got %v for test: %s", len(tt.content), charCount, tt.description)
			}

			// Check repository info
			if repository, exists := metadata["repository"]; exists {
				if repoMap, ok := repository.(map[string]string); ok {
					if owner, exists := repoMap["owner"]; !exists || owner != "code-sleuth" {
						t.Errorf("Expected owner 'code-sleuth', got %s for test: %s", owner, tt.description)
					}
					if repo, exists := repoMap["repo"]; !exists || repo != "outh" {
						t.Errorf("Expected repo 'outh', got %s for test: %s", repo, tt.description)
					}
				}
			}

			// Check content type
			if contentType, exists := metadata["content_type"]; !exists {
				t.Errorf("Expected content_type to exist for test: %s", tt.description)
			} else if contentType != "code" && contentType != "text" {
				t.Errorf("Expected content_type to be 'code' or 'text', got %s for test: %s", contentType, tt.description)
			}
		})
	}
}

func TestGitHubTransformer_ExtractRepoInfo(t *testing.T) {
	transformer := NewGitHubTransformer()

	tests := []struct {
		name        string
		rawURL      string
		expected    map[string]string
		description string
	}{
		{
			name:   "standard github URL",
			rawURL: "https://github.com/code-sleuth/outh/blob/main/file.go",
			expected: map[string]string{
				"owner":  "code-sleuth",
				"repo":   "outh",
				"branch": "main",
			},
			description: "should extract repo info from standard URL",
		},
		{
			name:   "github URL with branch containing slashes",
			rawURL: "https://github.com/code-sleuth/outh/blob/feature/new-feature/file.go",
			expected: map[string]string{
				"owner":  "code-sleuth",
				"repo":   "outh",
				"branch": "feature/new-feature",
			},
			description: "should extract repo info with complex branch name",
		},
		{
			name:        "invalid URL",
			rawURL:      "https://github.com/owner",
			expected:    nil,
			description: "should return nil for invalid URL",
		},
		{
			name:   "non-blob URL",
			rawURL: "https://github.com/code-sleuth/outh/tree/main",
			expected: map[string]string{
				"owner": "code-sleuth",
				"repo":  "outh",
			},
			description: "should extract basic info from non-blob URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.extractRepoInfo(tt.rawURL)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil result, got %v for test: %s", result, tt.description)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected non-nil result for test: %s", tt.description)
				return
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists || actualValue != expectedValue {
					t.Errorf("Expected %s=%s, got %s=%s for test: %s",
						key, expectedValue, key, actualValue, tt.description)
				}
			}
		})
	}
}

// Test the integration is skipped due to database complexity
func TestGitHubTransformer_Transform_Integration(t *testing.T) {
	t.Skip("Integration test requires database mocking - skipping for now")

	// This test would verify the complete Transform method workflow
	// but requires proper database mocking which is complex to set up
}

// Benchmark tests
func BenchmarkGitHubTransformer_ProcessContent(b *testing.B) {
	transformer := NewGitHubTransformer()
	content := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}"
	filePath := "main.go"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transformer.processContent(content, filePath)
	}
}

func BenchmarkGitHubTransformer_DetectLanguage(b *testing.B) {
	transformer := NewGitHubTransformer()
	content := "This is a sample English document for language detection testing."
	filePath := "README.md"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transformer.detectLanguage(content, filePath)
	}
}

func BenchmarkGitHubTransformer_ExtractMetadata(b *testing.B) {
	transformer := NewGitHubTransformer()
	source := &models.Source{
		RawURL: stringPtr("https://github.com/code-sleuth/outh/blob/main/src/main.go"),
	}
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}"
	filePath := "src/main.go"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transformer.extractMetadata(source, filePath, content)
	}
}
