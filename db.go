package main

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ----- DB helper: build DSN -----

func buildDSN(host, port, user, pass, db string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, db)
}

// ----- Commands (run in background) -----

func connectToDB(host, port, user, pass, db string) tea.Cmd {
	dsn := buildDSN(host, port, user, pass, db)
	return func() tea.Msg {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			return dbResultMsg{err: err}
		}
		defer conn.Close(ctx)

		if err := conn.Ping(ctx); err != nil {
			return dbResultMsg{err: err}
		}

		return dbResultMsg{err: nil}
	}
}

func fetchTables(host, port, user, pass, db string) tea.Cmd {
	dsn := buildDSN(host, port, user, pass, db)
	return func() tea.Msg {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			return tablesResultMsg{err: err}
		}
		defer conn.Close(ctx)

		rows, err := conn.Query(ctx, `
			SELECT table_name 
			FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
			ORDER BY table_name;
		`)
		if err != nil {
			return tablesResultMsg{err: err}
		}
		defer rows.Close()

		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return tablesResultMsg{err: err}
			}
			tables = append(tables, name)
		}
		if rows.Err() != nil {
			return tablesResultMsg{err: rows.Err()}
		}

		return tablesResultMsg{tables: tables, err: nil}
	}
}

func fetchRows(host, port, user, pass, db, table string, offset, limit int) tea.Cmd {
	dsn := buildDSN(host, port, user, pass, db)
	return func() tea.Msg {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			return rowsResultMsg{err: err}
		}
		defer conn.Close(ctx)

		// 1) Get total row count for pagination
		var total int
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)
		if err := conn.QueryRow(ctx, countQuery).Scan(&total); err != nil {
			return rowsResultMsg{err: err}
		}

		// 2) Fetch current page
		// WARNING: in a real app, safely quote the table name
		query := fmt.Sprintf(`SELECT * FROM %s LIMIT $1 OFFSET $2`, table)

		rows, err := conn.Query(ctx, query, limit, offset)
		if err != nil {
			return rowsResultMsg{err: err}
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
				return rowsResultMsg{err: err}
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
			return rowsResultMsg{err: rows.Err()}
		}

		return rowsResultMsg{
			columns:   cols,
			rows:      data,
			totalRows: total,
			offset:    offset,
			err:       nil,
		}
	}
}
