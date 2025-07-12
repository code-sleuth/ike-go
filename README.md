# IKE-GO

A production-ready Go application for importing, transforming, and processing content from various sources (WordPress JSON API, GitHub repositories) into a structured document database with vector embeddings.

## Overview

IKE-GO is a complete rewrite of the original Python scripts, providing:

- **Extensible Architecture**: Plugin-based system for importers, transformers, chunkers, and embedders
- **High Performance**: Concurrent processing with configurable worker pools
- **Multiple Sources**: Support for WordPress JSON API and GitHub repositories
- **Vector Embeddings**: Integration with OpenAI and Together AI embedding models
- **Turso Database**: Optimized for Turso/SQLite with proper schema management

## Architecture

IKE-GO follows a clean architecture pattern with well-defined interfaces and separation of concerns:

- **Command Layer** (`cmd/`): CLI commands using Cobra framework
- **Service Layer** (`internal/manager/services/`): Business logic orchestration
- **Domain Layer** (`internal/manager/`): Core business logic with interfaces
- **Infrastructure Layer** (`pkg/`): External concerns (database, logging, utilities)

The system uses a plugin-based architecture where components implement interfaces, making it easy to extend with new sources, transformers, chunkers, and embedders.

## Features

### Core Components

1. **Importers**: Fetch content from external sources
   - WordPress JSON API importer
   - GitHub repository importer

2. **Transformers**: Convert raw downloads into structured documents
   - WordPress content transformer (HTML to Markdown)
   - GitHub file transformer (with syntax highlighting)

3. **Chunkers**: Split documents into embeddable chunks
   - Token-based chunking using tiktoken with configurable tokenizer support
   - Environment-configurable chunk sizes and overlap settings
   - Support for multiple tokenizer encodings (cl100k_base, p50k_base, r50k_base)

4. **Embedders**: Generate vector embeddings
   - OpenAI embeddings (text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002)
   - Together AI embeddings
   - HTTP client injection support for testing and custom configurations

### Processing Pipeline

1. **Import**: Fetch content from source URLs
2. **Transform**: Convert downloads to structured documents
3. **Chunk**: Split documents into manageable pieces
4. **Embed**: Generate vector embeddings for each chunk
5. **Store**: Save everything to Turso database

## Database Schema

The application manages the following entities:

- **Sources**: Content sources with URL parsing and metadata
- **Downloads**: Download history and status tracking
- **Documents**: Processed documents with chunking configuration
- **Chunks**: Text chunks with hierarchical relationships
- **Tags**: Document tagging system
- **Embeddings**: Vector embeddings for semantic search
- **Requests**: Query history and result tracking

## Prerequisites

- Go 1.24 or higher
- Turso database account and credentials
- Make (optional, for using Makefile)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/code-sleuth/ike-go.git
cd ike-go
```

2. Install dependencies:
```bash
go mod tidy
```

3. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your Turso credentials
```

4. Build the application:
```bash
make build
# or
go build -o ike-go .
```

## Configuration

Set the following environment variables:

```bash
# Turso Database
export TURSO_DATABASE_URL="your-turso-database-url"
export TURSO_AUTH_TOKEN="your-turso-auth-token"

# OpenAI (optional)
export OPENAI_API_KEY="your-openai-api-key"

# Together AI (optional)
export TOGETHER_API_KEY="your-together-api-key"

# GitHub (optional - for private repos)
export GITHUB_TOKEN="your-github-token"

# Deployment stage eg (local, dev, prod)
export STAGE="your-deployment-stage"

# Chunker Configuration (optional)
export CHUNKER_TOKENIZER="cl100k_base"          # Tokenizer: cl100k_base, p50k_base, r50k_base
export CHUNKER_DEFAULT_MAX_TOKENS="100"         # Default max tokens per chunk
export CHUNKER_DEFAULT_OVERLAP_TOKENS="20"      # Default overlap tokens for overlapping chunks
export CHUNKER_LOG_LEVEL="error"                # Log level: debug, info, warn, error
```

## Usage

### Import from WordPress JSON API

```bash
./bin/ike-go import --url "https://wsform.com/wp-json/wp/v2/knowledgebase"
```

### Import from GitHub Repository

```bash
./bin/ike-go import --url "https://github.com/code-sleuth/outh"
```

### Import with Custom Settings

```bash
./bin/ike-go import \
  --url "https://example.com/wp-json/wp/v2/posts" \
  --model "text-embedding-3-large" \
  --tokens 4096 \
  --concurrency 10
```

### Transform Existing Downloads

```bash
./bin/ike-go transform --download-id "123e4567-e89b-12d3-a456-426614174000"
```

### Manage Sources and Documents

```bash
# List all sources
./bin/ike-go sources list

# Get specific source
./bin/ike-go sources get <source-id>

# List all documents
./bin/ike-go documents list

# Get specific document
./bin/ike-go documents get <document-id>
```

### Database Migration

```bash
./bin/ike-go migrate
```

## Supported Models

### OpenAI Embeddings
- `text-embedding-3-small` (1536 dimensions, 8191 max tokens)
- `text-embedding-3-large` (3072 dimensions, 8191 max tokens)
- `text-embedding-ada-002` (1536 dimensions, 8191 max tokens)

### Together AI Embeddings
- `togethercomputer/m2-bert-80M-8k-retrieval` (768 dimensions, 8192 max tokens)
- `togethercomputer/m2-bert-80M-32k-retrieval` (768 dimensions, 32768 max tokens)

## File Support

### WordPress JSON API
- Automatically converts HTML content to Markdown
- Extracts metadata (title, description, canonical URL, etc.)
- Supports language detection
- Handles pagination automatically

### GitHub Repositories
- Supports multiple file types with syntax highlighting
- Automatic exclusion of common non-content directories
- Configurable file size limits and supported extensions
- Base64 content decoding
- Language detection and metadata extraction

### Supported File Extensions

The GitHub importer supports the following file types:

#### Documentation Files
- `.md` - Markdown
- `.txt` - Plain text
- `.rst` - reStructuredText

#### Programming Languages
- `.py` - Python
- `.js` - JavaScript 
- `.ts` - TypeScript
- `.go` - Go
- `.java` - Java
- `.cpp`, `.c`, `.h`, `.hpp` - C/C++
- `.rb` - Ruby
- `.php` - PHP
- `.swift` - Swift
- `.kt` - Kotlin
- `.scala` - Scala
- `.rs` - Rust
- `.dart` - Dart
- `.lua` - Lua
- `.pl` - Perl
- `.r` - R

#### Web Technologies
- `.css` - Cascading Style Sheets
- `.html`, `.htm` - HTML
- `.xml` - XML

#### Data & Configuration
- `.json` - JSON
- `.yaml`, `.yml` - YAML
- `.toml` - TOML
- `.ini`, `.cfg`, `.conf` - Configuration files

#### Shell Scripts
- `.sh` - Bash shell scripts
- `.bash` - Bash scripts
- `.zsh` - Zsh scripts
- `.fish` - Fish scripts
- `.ps1` - PowerShell scripts

#### Database
- `.sql` - SQL scripts

#### Exclusions
The following directories and files are automatically excluded:
- `.git/` - Git version control
- `node_modules/` - Node.js dependencies
- `.next/`, `.nuxt/` - Framework build directories
- `dist/`, `build/` - Build output directories
- `.vscode/`, `.idea/` - IDE configuration
- `__pycache__/`, `.pytest_cache/` - Python cache
- `.coverage` - Coverage reports
- `.DS_Store` - macOS system files

> **Note**: Both supported file extensions and exclusion patterns are configurable programmatically through the `SetSupportedExtensions()` and `SetExclusions()` methods of the GitHub importer.

## Chunking Configuration

The token-based chunker supports environment-based configuration:

### Tokenizer Options
- **cl100k_base** (default): Used by GPT-3.5-turbo and GPT-4 models
- **p50k_base**: Used by older GPT-3 models
- **r50k_base**: Used by Codex models

### Configuration Variables
- `CHUNKER_TOKENIZER`: Tokenizer encoding to use (default: cl100k_base)
- `CHUNKER_DEFAULT_MAX_TOKENS`: Default maximum tokens per chunk (default: 100)
- `CHUNKER_DEFAULT_OVERLAP_TOKENS`: Default overlap tokens for overlapping chunks (default: 20)
- `CHUNKER_LOG_LEVEL`: Chunker logging level - debug, info, warn, error (default: error)

### Usage in Tests
Tests automatically load configuration from `.env` file and use environment values for chunk sizing, making tests configurable without code changes.

## Available Commands

- `import` - Import content from external sources
- `transform` - Transform existing downloads into documents and chunks
- `migrate` - Run database migrations
- `sources` - Manage content sources
  - `list` - List all sources
  - `get` - Get source by ID
  - `create` - Create new source
  - `delete` - Delete source
- `documents` - Manage documents
  - `list` - List all documents
  - `get` - Get document by ID

## Makefile Targets

- `make build` - Build the ike-go binary for current platform
- `make build-linux` - Build the ike-go binary for Linux
- `make build-mac` - Build the ike-go binary for macOS
- `make build-windows` - Build the ike-go binary for Windows
- `make migrate` - Run database migrations
- `make run` - Build and run the CLI
- `make clean` - Remove built binary
- `make deps` - Install/update dependencies
- `make fmt` - Format Go code using gofmt, goimports, and golines
- `make lint` - Run linting checks using golangci-lint
- `make test` - Run all tests with coverage
- `make version` - Show version and build information
- `make help` - Show available targets

## Development

### Project Structure

```
ike-go/
├── main.go                     # Entry point
├── .env.example               # Environment variables template
├── Makefile                   # Build automation
├── cmd/                       # CLI commands
│   ├── root.go               # Root command setup
│   ├── migrate.go            # Database migration command
│   ├── import.go             # Import command
│   ├── transform.go          # Transform command
│   ├── sources.go            # Source management commands
│   └── documents.go          # Document management commands
├── internal/                  # Internal application code
│   └── manager/
│       ├── chunkers/
│       │   └── token.go      # Token-based chunking
│       ├── embedders/
│       │   ├── openai.go     # OpenAI embedding integration
│       │   └── togetherai.go # Together AI integration
│       ├── importers/
│       │   ├── github.go     # GitHub repository importer
│       │   └── wpjson.go     # WordPress JSON API importer
│       ├── interfaces/
│       │   └── interfaces.go # Core interfaces
│       ├── models/
│       │   └── models.go     # Data models
│       ├── repository/
│       │   └── sources.go    # Repository layer
│       ├── services/
│       │   └── engine.go     # Processing engine
│       └── transformers/
│           ├── github.go     # GitHub content transformer
│           └── wpjson.go     # WordPress transformer
├── pkg/                       # Shared packages
│   ├── db/
│   │   └── connection.go     # Turso DB connection
│   ├── migrations/
│   │   └── init_schema.sql   # Database schema
│   └── util/
│       └── logger.go         # Logging utilities
└── py/                        # Legacy Python scripts
    ├── wp-json-importer.py
    └── wp-json-transform-sync.py
```

### Adding New Commands

1. Create a new command file in `cmd/`
2. Implement the command using Cobra framework
3. Add the command to `root.go`
4. Update this README with new command documentation

### Testing

The project includes comprehensive test coverage with both unit and integration tests:

- **Unit Tests**: Fast tests that don't require external dependencies
- **Integration Tests**: Real API tests that validate actual service integration
- **Database Tests**: Integration tests with real Turso database connections

#### Running Tests

```bash
make test              # Run all tests with coverage
go test ./...          # Run all tests without coverage
go test -v ./...       # Run all tests with verbose output
```

#### API Integration Tests

The embedders include real API integration tests that require valid API keys:

```bash
# Set API keys in .env file, then run specific embedder tests
export OPENAI_API_KEY="your-key"
export TOGETHER_API_KEY="your-key"

# Run embedder tests with real API calls
go test ./internal/manager/embedders -v
```

**Note**: Tests gracefully skip when API keys are not available, ensuring the build never fails due to missing credentials.

### Code Quality

The project includes comprehensive linting and formatting tools:

- **Linting**: Uses `golangci-lint` with configuration in `.golangci.yaml`
- **Formatting**: Automatically formats code with `gofmt`, `goimports`, and `golines` (120 char limit)
- **CI Ready**: All checks can be run via `make lint` and `make fmt`

Run before committing:
```bash
make fmt   # Format all Go files
make lint  # Run all linting checks
make test  # Run all tests
```

## Dependencies

### Core Dependencies
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [libsql-client-go](https://github.com/tursodatabase/libsql-client-go) - Turso database client
- [godotenv](https://github.com/joho/godotenv) - Environment variable loading
- [zerolog](https://github.com/rs/zerolog) - Structured logging
- [uuid](https://github.com/google/uuid) - UUID generation

### Processing Dependencies
- [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown) - HTML to Markdown conversion
- [tiktoken-go](https://github.com/tiktoken-go/tokenizer) - Token counting for chunking

### Development Dependencies
- [golangci-lint](https://golangci-lint.run/) - Linting (configured in `.golangci.yaml`)
- [gofmt](https://golang.org/cmd/gofmt/) - Code formatting
- [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) - Import management
- [golines](https://github.com/segmentio/golines) - Line length formatting

## License

[MIT](LICENSE)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Troubleshooting

### Common Issues

1. **Database Connection Issues**
   - Verify `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN` are set correctly
   - Check your Turso database is accessible and active

2. **Embedding API Errors**
   - Ensure API keys are set: `OPENAI_API_KEY` or `TOGETHER_API_KEY`
   - Check your API quota and rate limits
   - Verify the model name is exactly as listed in supported models

3. **GitHub Import Issues**
   - Set `GITHUB_TOKEN` for private repositories or to increase rate limits
   - Check the repository URL format: `https://github.com/owner/repo`

4. **Build Issues**
   - Run `go mod tidy` to ensure dependencies are up to date
   - Verify Go version 1.24+ is installed

5. **Test Issues**
   - Embedder tests require API keys: set `OPENAI_API_KEY` and/or `TOGETHER_API_KEY`
   - Integration tests may be rate-limited: consider running fewer concurrent tests
   - Database tests require valid Turso credentials in `.env` file
   - Chunker tests use environment configuration: customize `CHUNKER_*` variables for different test scenarios

## Support

For issues and questions, please create an issue in the repository.