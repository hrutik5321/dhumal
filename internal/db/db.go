package db

import "context"

// Connection parameters for any SQL DB.
type ConnConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// Options for fetching rows (pagination + filter).
type QueryOptions struct {
	Limit  int
	Offset int
	Filter string // raw WHERE fragment, without "WHERE"
}

// Page of rows.
type RowPage struct {
	Columns   []string
	Rows      [][]string
	TotalRows int
	Offset    int
}

type DB interface {
	Connect(ctx context.Context, cfg ConnConfig) error
	Close() error

	ListTables(ctx context.Context) ([]string, error)
	FetchRows(ctx context.Context, table string, opts QueryOptions) (RowPage, error)
	DeleteRows(ctx context.Context, table string, where string) (int64, error)
}
