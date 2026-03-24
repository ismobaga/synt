package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// DB wraps the sql.DB connection.
type DB struct {
	*sql.DB
}

// New creates a new database connection.
func New(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := conn.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{conn}, nil
}
