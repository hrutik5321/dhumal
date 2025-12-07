package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hrutik5321/dhumal/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

func New() *PostgresDB {
	return &PostgresDB{}
}

func (p *PostgresDB) buildDSN(cfg db.ConnConfig) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)
}

func (p *PostgresDB) DeleteRows(ctx context.Context, table string, where string) (int64, error) {
	if p.pool == nil {
		return 0, fmt.Errorf("database not connected")
	}

	if strings.TrimSpace(where) == "" {
		return 0, fmt.Errorf("empty WHERE clause is not allowed for DELETE")
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE %s`, table, where)

	cmdTag, err := p.pool.Exec(ctx, query)
	if err != nil {
		return 0, err
	}

	return cmdTag.RowsAffected(), nil
}

// Connect implements db.DB.
func (p *PostgresDB) Connect(ctx context.Context, cfg db.ConnConfig) error {
	dsn := p.buildDSN(cfg)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return err
	}

	p.pool = pool
	return nil
}

// Close db
func (p *PostgresDB) Close() error {
	if p.pool != nil {
		p.pool.Close()
	}
	return nil
}

// ListTables
func (p *PostgresDB) ListTables(ctx context.Context) ([]string, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return tables, nil
}

// FetchRows
func (p *PostgresDB) FetchRows(
	ctx context.Context,
	table string,
	opts db.QueryOptions,
) (db.RowPage, error) {
	if p.pool == nil {
		return db.RowPage{}, fmt.Errorf("database not connected")
	}

	// Build optional WHERE clause from filter
	whereClause := ""
	if opts.Filter != "" {
		whereClause = " WHERE " + opts.Filter
	}

	// 1) Get total row count for pagination
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s%s`, table, whereClause)
	if err := p.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return db.RowPage{}, err
	}

	// 2) Fetch current page
	query := fmt.Sprintf(`SELECT * FROM %s%s LIMIT $1 OFFSET $2`, table, whereClause)

	rows, err := p.pool.Query(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return db.RowPage{}, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = string(fd.Name)
	}

	var data [][]string
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return db.RowPage{}, err
		}
		r := make([]string, len(values))

		for i, v := range values {
			switch val := v.(type) {

			// UUID as [16]byte
			case [16]byte:
				if uid, err := uuid.FromBytes(val[:]); err == nil {
					r[i] = uid.String()
				} else {
					r[i] = fmt.Sprint(val)
				}

			// UUID / binary as []byte
			case []byte:
				if uid, err := uuid.FromBytes(val); err == nil {
					r[i] = uid.String()
				} else {
					r[i] = string(val)
				}

			// pgx UUID type
			case pgtype.UUID:
				if val.Valid {
					r[i] = val.String()
				} else {
					r[i] = "NULL"
				}

			case nil:
				r[i] = "NULL"

			case fmt.Stringer:
				r[i] = val.String()

			default:
				r[i] = fmt.Sprint(v)
			}
		}
		data = append(data, r)
	}
	if rows.Err() != nil {
		return db.RowPage{}, rows.Err()
	}

	return db.RowPage{
		Columns:   cols,
		Rows:      data,
		TotalRows: total,
		Offset:    opts.Offset,
	}, nil
}
