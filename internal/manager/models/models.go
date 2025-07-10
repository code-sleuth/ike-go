package models

import (
	"time"
)

type Source struct {
	ID           string    `json:"id"`
	AuthorEmail  *string   `json:"author_email"`
	RawURL       *string   `json:"raw_url"`
	Scheme       *string   `json:"scheme"`
	Host         *string   `json:"host"`
	Path         *string   `json:"path"`
	Query        *string   `json:"query"`
	ActiveDomain int       `json:"active_domain"`
	Format       *string   `json:"format"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Download struct {
	ID           string     `json:"id"`
	SourceID     string     `json:"source_id"`
	AttemptedAt  *time.Time `json:"attempted_at"`
	DownloadedAt *time.Time `json:"downloaded_at"`
	StatusCode   *int       `json:"status_code"`
	Headers      string     `json:"headers"`
	Body         *string    `json:"body"`
}

type Document struct {
	ID           string     `json:"id"`
	SourceID     string     `json:"source_id"`
	DownloadID   string     `json:"download_id"`
	Format       *string    `json:"format"`
	IndexedAt    *time.Time `json:"indexed_at"`
	MinChunkSize int        `json:"min_chunk_size"`
	MaxChunkSize int        `json:"max_chunk_size"`
	PublishedAt  *time.Time `json:"published_at"`
	ModifiedAt   *time.Time `json:"modified_at"`
	WPVersion    *string    `json:"wp_version"`
}

type Chunk struct {
	ID            string  `json:"id"`
	DocumentID    string  `json:"document_id"`
	ParentChunkID *string `json:"parent_chunk_id"`
	LeftChunkID   *string `json:"left_chunk_id"`
	RightChunkID  *string `json:"right_chunk_id"`
	Body          *string `json:"body"`
	ByteSize      *int    `json:"byte_size"`
	Tokenizer     *string `json:"tokenizer"`
	TokenCount    *int    `json:"token_count"`
	NaturalLang   *string `json:"natural_lang"`
	CodeLang      *string `json:"code_lang"`
}

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type DocumentTag struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	TagID      string    `json:"tag_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type DocumentMeta struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Key        string    `json:"key"`
	Meta       *string   `json:"meta"`
	CreatedAt  time.Time `json:"created_at"`
}

type Embedding struct {
	ID            string    `json:"id"`
	Embedding1536 []float32 `json:"embedding_1536"`
	Embedding3072 []float32 `json:"embedding_3072"`
	Embedding768  []float32 `json:"embedding_768"`
	Model         *string   `json:"model"`
	EmbeddedAt    time.Time `json:"embedded_at"`
	ObjectID      string    `json:"object_id"`
	ObjectType    string    `json:"object_type"`
}

type Request struct {
	ID           string    `json:"id"`
	Message      string    `json:"message"`
	Meta         *string   `json:"meta"`
	RequestedAt  time.Time `json:"requested_at"`
	ResultChunks *string   `json:"result_chunks"`
}
