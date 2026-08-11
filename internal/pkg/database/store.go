package database

import (
	"context"
	"database/sql"
)

// Store defines the interface for database operations and transactions.
type Store interface {
	DB() *sql.DB
	ExecTx(ctx context.Context, fn func(tx *sql.Tx) error) error
	Begin() (*sql.Tx, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	
	// Add some standard querying shortcuts if needed
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SQLStore is the sql.DB implementation of Store.
type SQLStore struct {
	db *sql.DB
}

// NewStore creates a new Store instance.
func NewStore(db *sql.DB) Store {
	return &SQLStore{db: db}
}

// DB returns the underlying sql.DB instance.
func (s *SQLStore) DB() *sql.DB {
	return s.db
}

func (s *SQLStore) Begin() (*sql.Tx, error) {
	return s.db.Begin()
}

func (s *SQLStore) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, opts)
}

// ExecTx provides a clean way to execute a block of database operations within a transaction.
func (s *SQLStore) ExecTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	err = fn(tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return err // Return original error, or combine them
		}
		return err
	}
	
	return tx.Commit()
}

// Standard query wrappers
func (s *SQLStore) QueryRow(query string, args ...any) *sql.Row {
	return s.db.QueryRow(query, args...)
}

func (s *SQLStore) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *SQLStore) Query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(query, args...)
}

func (s *SQLStore) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s *SQLStore) Exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(query, args...)
}

func (s *SQLStore) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}
