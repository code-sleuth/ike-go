-- migrate:up

-- sources table
CREATE TABLE IF NOT EXISTS sources (
    id TEXT NOT NULL PRIMARY KEY,
    author_email TEXT,
    raw_url TEXT,
    scheme TEXT,
    host TEXT,
    path TEXT,
    query TEXT,
    active_domain INTEGER NOT NULL CHECK (active_domain IN (0, 1)),
    format TEXT CHECK (format IN ('json', 'yml', 'yaml')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- downloads table
CREATE TABLE IF NOT EXISTS downloads (
    id TEXT NOT NULL PRIMARY KEY,
    source_id TEXT NOT NULL,
    attempted_at TEXT,
    downloaded_at TEXT,
    status_code INTEGER,
    headers TEXT NOT NULL,
    body TEXT,
    FOREIGN KEY (source_id) REFERENCES sources(id)
);

-- documents table
CREATE TABLE IF NOT EXISTS documents (
    id TEXT NOT NULL PRIMARY KEY,
    source_id TEXT NOT NULL,
    download_id TEXT NOT NULL,
    format TEXT CHECK (format IN ('json', 'yml', 'yaml')),
    indexed_at TEXT,
    min_chunk_size INTEGER NOT NULL,
    max_chunk_size INTEGER NOT NULL,
    published_at TEXT,
    modified_at TEXT,
    wp_version TEXT,
    FOREIGN KEY (source_id) REFERENCES sources(id),
    FOREIGN KEY (download_id) REFERENCES downloads(id)
);

-- chunks table
CREATE TABLE IF NOT EXISTS chunks (
    id TEXT NOT NULL PRIMARY KEY,
    document_id TEXT NOT NULL,
    parent_chunk_id TEXT,
    left_chunk_id TEXT,
    right_chunk_id TEXT,
    body TEXT,
    byte_size INTEGER,
    tokenizer TEXT,
    token_count INTEGER,
    natural_lang TEXT CHECK (natural_lang IN ('en', 'fr')),
    code_lang TEXT CHECK (code_lang IN ('python', 'sql', 'javascript')),
    FOREIGN KEY (document_id) REFERENCES documents(id),
    FOREIGN KEY (parent_chunk_id) REFERENCES chunks(id),
    FOREIGN KEY (left_chunk_id) REFERENCES chunks(id),
    FOREIGN KEY (right_chunk_id) REFERENCES chunks(id)
);

-- tags table
CREATE TABLE IF NOT EXISTS tags (
    id TEXT NOT NULL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- document_tags table
CREATE TABLE IF NOT EXISTS document_tags (
    id TEXT NOT NULL PRIMARY KEY,
    document_id TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (document_id, tag_id),
    FOREIGN KEY (document_id) REFERENCES documents(id),
    FOREIGN KEY (tag_id) REFERENCES tags(id)
);

-- document_meta table
CREATE TABLE IF NOT EXISTS document_meta (
    id TEXT NOT NULL PRIMARY KEY,
    document_id TEXT NOT NULL,
    "key" TEXT NOT NULL,
    meta TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (document_id, key),
    FOREIGN KEY (document_id) REFERENCES documents(id)
);

-- embeddings table
CREATE TABLE IF NOT EXISTS embeddings (
    id TEXT NOT NULL PRIMARY KEY,
    embedding_1536 TEXT,
    embedding_3072 TEXT,
    embedding_768 TEXT,
    model TEXT,
    embedded_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    object_id TEXT NOT NULL,
    object_type TEXT NOT NULL DEFAULT 'chunk',
    FOREIGN KEY (object_id) REFERENCES chunks(id)
);

-- requests table
CREATE TABLE IF NOT EXISTS requests (
    id TEXT NOT NULL PRIMARY KEY,
    message TEXT NOT NULL,
    meta TEXT,
    requested_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    result_chunks TEXT -- Store as comma-separated UUIDs or JSON array
);

-- schema_migrations
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT
);

-- indexes for search
CREATE INDEX IF NOT EXISTS idx_documents_source_id ON documents(source_id);
CREATE INDEX IF NOT EXISTS idx_downloads_source_id ON downloads(source_id);
CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_document_tags_document_id ON document_tags(document_id);
CREATE INDEX IF NOT EXISTS idx_document_tags_tag_id ON document_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_document_meta_document_id ON document_meta(document_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_object_id ON embeddings(object_id);

-- trigger function to maintain last 3 downloads
CREATE TRIGGER IF NOT EXISTS maintain_last_3_downloads
AFTER INSERT ON downloads
BEGIN
    DELETE FROM downloads
    WHERE source_id = NEW.source_id
      AND id NOT IN (
        SELECT id
        FROM downloads
        WHERE source_id = NEW.source_id
        ORDER BY downloaded_at DESC NULLS LAST
        LIMIT 3
      );
END;