package db

import (
	"database/sql"
	"errors"
	"os"
	"strings"

	"github.com/code-sleuth/ike-go/pkg/util"

	"github.com/rs/zerolog"
	"github.com/tursodatabase/libsql-client-go/libsql"
)

var (
	ErrDatabaseURLRequired = errors.New("TURSO_DATABASE_URL environment variable is required")
	ErrAuthTokenRequired   = errors.New("TURSO_AUTH_TOKEN environment variable is required")
)

type DB struct {
	*sql.DB
}

func NewConnection() (*DB, error) {
	dbURL := os.Getenv("TURSO_DATABASE_URL")
	logger := util.NewLogger(zerolog.ErrorLevel)
	if strings.EqualFold(dbURL, "") {
		logger.Error().Msg("TURSO_DATABASE_URL env variable not set")
		return nil, ErrDatabaseURLRequired
	}

	authToken := os.Getenv("TURSO_AUTH_TOKEN")
	if strings.EqualFold(authToken, "") {
		logger.Error().Msg("TURSO_AUTH_TOKEN env variable not set")
		return nil, ErrAuthTokenRequired
	}

	connector, err := libsql.NewConnector(dbURL, libsql.WithAuthToken(authToken))
	if err != nil {
		logger.Err(err).Msg("failed to create connector")
		return nil, err
	}

	db := sql.OpenDB(connector)
	if err := db.Ping(); err != nil {
		logger.Err(err).Msg("failed to ping database")
		return nil, err
	}

	return &DB{DB: db}, nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}

// Connect is an alias for NewConnection for compatibility.
func Connect() (*sql.DB, error) {
	dbWrapper, err := NewConnection()
	if err != nil {
		return nil, err
	}
	return dbWrapper.DB, nil
}
