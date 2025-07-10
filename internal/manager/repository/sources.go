package repository

import (
	"database/sql"
	"errors"

	"github.com/code-sleuth/ike-go/internal/manager/models"
	"github.com/code-sleuth/ike-go/pkg/db"
	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/rs/zerolog"
)

var errSourceNotFound = errors.New("source not found")

type SourceRepository struct {
	db     *db.DB
	logger zerolog.Logger
}

func NewSourceRepository(database *db.DB) *SourceRepository {
	logger := util.NewLogger(zerolog.ErrorLevel)
	return &SourceRepository{
		db:     database,
		logger: logger,
	}
}

func (r *SourceRepository) Create(source *models.Source) error {
	query := `
		INSERT INTO sources (id, author_email, raw_url, scheme, host, path, 
		                     query, active_domain, format, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(query, source.ID, source.AuthorEmail, source.RawURL, source.Scheme,
		source.Host, source.Path, source.Query, source.ActiveDomain, source.Format,
		source.CreatedAt.Format("2006-01-02T15:04:05Z"), source.UpdatedAt.Format("2006-01-02T15:04:05Z"))
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to create source")
	}
	return err
}

func (r *SourceRepository) GetByID(id string) (*models.Source, error) {
	query := `
		SELECT id, author_email, raw_url, scheme, host, path, query, active_domain, format, created_at, updated_at
		FROM sources WHERE id = ?
	`
	row := r.db.QueryRow(query, id)

	var source models.Source
	err := row.Scan(&source.ID, &source.AuthorEmail, &source.RawURL, &source.Scheme,
		&source.Host, &source.Path, &source.Query, &source.ActiveDomain, &source.Format,
		&source.CreatedAt, &source.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		r.logger.Error().Str("source_id", id).Msg("Source not found")
		return nil, errSourceNotFound
	}
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to get source")
		return nil, err
	}

	return &source, nil
}

func (r *SourceRepository) List() ([]models.Source, error) {
	query := `
		SELECT id, author_email, raw_url, scheme, host, path, query, active_domain, format, created_at, updated_at
		FROM sources ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if err = rows.Err(); err != nil {
		return nil, err
	}

	var sources []models.Source
	for rows.Next() {
		var source models.Source
		err := rows.Scan(&source.ID, &source.AuthorEmail, &source.RawURL, &source.Scheme,
			&source.Host, &source.Path, &source.Query, &source.ActiveDomain, &source.Format,
			&source.CreatedAt, &source.UpdatedAt)
		if err != nil {
			r.logger.Error().Err(err).Msg("Failed to scan source")
			return nil, err
		}
		sources = append(sources, source)
	}

	return sources, nil
}

func (r *SourceRepository) Update(source *models.Source) error {
	query := `
		UPDATE sources SET author_email = ?, raw_url = ?, scheme = ?, host = ?, path = ?, 
		query = ?, active_domain = ?, format = ?, updated_at = datetime('now')
		WHERE id = ?
	`
	_, err := r.db.Exec(query, source.AuthorEmail, source.RawURL, source.Scheme,
		source.Host, source.Path, source.Query, source.ActiveDomain, source.Format,
		source.ID)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to update source")
	}
	return err
}

func (r *SourceRepository) Delete(id string) error {
	query := `DELETE FROM sources WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to delete source")
	}
	return err
}
