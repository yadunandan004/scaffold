package orm

import (
	"context"
	"database/sql"
	"fmt"
)

// Query provides simple helpers for raw SQL queries
// Leverages RawScanner for flexible type handling
type Query struct {
	Ctx     context.Context
	Txn     *sql.Tx
	Scanner *RawScanner
}

// Count executes a COUNT query and returns the integer result
// Example: count, err := q.Count("SELECT COUNT(*) FROM users WHERE active = $1", true)
func (q *Query) Count(query string, args ...interface{}) (int, error) {
	var count int
	err := q.Txn.QueryRowContext(q.Ctx, query, args...).Scan(&count)
	return count, err
}

// Exists checks if a query returns any rows
// Wraps the query in SELECT EXISTS(...) for efficiency
func (q *Query) Exists(query string, args ...interface{}) (bool, error) {
	var exists bool
	checkQuery := fmt.Sprintf("SELECT EXISTS(%s)", query)
	err := q.Txn.QueryRowContext(q.Ctx, checkQuery, args...).Scan(&exists)
	return exists, err
}

// QueryRow executes a query expecting a single row and scans into dest
// Uses RawScanner for flexible destination types (struct, slice, map, primitive)
// Returns sql.ErrNoRows if no rows found
func (q *Query) QueryRow(query string, dest interface{}, args ...interface{}) error {
	rows, err := q.Txn.QueryContext(q.Ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return q.Scanner.ScanRow(rows, dest)
}

// QueryRows executes a query expecting multiple rows and scans into dest slice
// Uses RawScanner for flexible destination types
// dest must be a pointer to a slice
func (q *Query) QueryRows(query string, dest interface{}, args ...interface{}) error {
	rows, err := q.Txn.QueryContext(q.Ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return q.Scanner.ScanRaw(rows, dest)
}

// Exec executes a command (INSERT/UPDATE/DELETE) and returns the result
func (q *Query) Exec(query string, args ...interface{}) (sql.Result, error) {
	return q.Txn.ExecContext(q.Ctx, query, args...)
}

// Tx returns the underlying sql.Tx for advanced use cases
// Use sparingly - prefer Query methods when possible
func (q *Query) Tx() *sql.Tx {
	return q.Txn
}

// Commit commits the transaction
func (q *Query) Commit() error {
	return q.Txn.Commit()
}

// Rollback rolls back the transaction
func (q *Query) Rollback() error {
	return q.Txn.Rollback()
}

// Query executes a query that returns rows (for manual iteration)
// Returns *sql.Rows for custom scanning logic
func (q *Query) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return q.Txn.QueryContext(q.Ctx, query, args...)
}

// QueryRowRaw executes a query expecting a single row
// Returns *sql.Row for manual Scan() - use for simple cases
func (q *Query) QueryRowRaw(query string, args ...interface{}) *sql.Row {
	return q.Txn.QueryRowContext(q.Ctx, query, args...)
}
