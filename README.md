# IKE-GO

CLI tool for importing, chunking, and embedding content from WordPress and GitHub into a vector database.

## What It Does

IKE-GO processes content through a 5-step pipeline:

1. **Import** - Fetch content from WordPress JSON API or GitHub repositories
2. **Transform** - Convert raw content to structured documents with metadata
3. **Chunk** - Split documents into token-sized pieces for embedding
4. **Embed** - Generate vector embeddings using OpenAI or Together AI
5. **Store** - Save everything to Turso/SQLite database

## Quick Start

### Prerequisites
- Go 1.24+
- [Turso](https://turso.tech) database account

### Setup

```bash
git clone https://github.com/code-sleuth/ike-go.git
cd ike-go
cp .env.example .env    # Add your credentials
make build
./bin/ike-go migrate
```

## Configuration

Set these environment variables in `.env`:

```bash
# Required - Database
TURSO_DATABASE_URL="libsql://your-db.turso.io"
TURSO_AUTH_TOKEN="your-token"

# Required - At least one embedding provider
OPENAI_API_KEY="sk-..."
TOGETHER_API_KEY="..."

# Optional
GITHUB_TOKEN="ghp_..."              # For private repos
STAGE="local"                       # local, dev, prod
```

## Workflow Example

A typical workflow importing content from multiple sources:

```bash
# 1. Run database migrations
./bin/ike-go migrate

# 2. Import WordPress content (knowledgebase articles)
./bin/ike-go import --url "https://wsform.com/wp-json/wp/v2/knowledgebase"

# 3. Import a GitHub repository
./bin/ike-go import --url "https://github.com/code-sleuth/outh"

# 4. View imported sources
./bin/ike-go sources list

# 5. View processed documents
./bin/ike-go documents list

# 6. Re-embed existing content with a different model
./bin/ike-go transform --download-id "<uuid>" --model "text-embedding-3-large"
```

## Command Reference

| Command | Description |
|---------|-------------|
| `migrate` | Run database migrations |
| `import --url <url>` | Import and embed content from URL |
| `transform --download-id <uuid>` | Re-process existing downloads |
| `sources list` | List all content sources |
| `sources get <id>` | Get source details |
| `documents list` | List all documents |
| `documents get <id>` | Get document details |

### Import Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--model` | `text-embedding-3-small` | Embedding model |
| `--tokens` | `100` | Max tokens per chunk |
| `--concurrency` | `5` | Worker pool size |

## Supported Models

**OpenAI**
- `text-embedding-3-small` (1536 dims)
- `text-embedding-3-large` (3072 dims)
- `text-embedding-ada-002` (1536 dims)

**Together AI**
- `togethercomputer/m2-bert-80M-8k-retrieval` (768 dims)
- `togethercomputer/m2-bert-80M-32k-retrieval` (768 dims)

## Development

```bash
make build    # Build binary (includes fmt + lint)
make test     # Run tests with coverage
make lint     # Run golangci-lint
make fmt      # Format code
```

## License

[MIT](LICENSE)
