package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/eval_llm/backend/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Database represents a PostgreSQL connection pool
type Database struct {
	pool *pgxpool.Pool
}

// NewDatabase creates a new database connection pool
func NewDatabase(cfg *config.DatabaseConfig) (*Database, error) {
	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s pool_max_conns=%d",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.MaxConnections,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{pool: pool}, nil
}

// Close closes the connection pool
func (db *Database) Close() {
	db.pool.Close()
}

// Pool returns the underlying connection pool
func (db *Database) Pool() *pgxpool.Pool {
	return db.pool
}

// Ping verifies the database connection
func (db *Database) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// BeginTx starts a new transaction
func (db *Database) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return db.pool.Begin(ctx)
}
