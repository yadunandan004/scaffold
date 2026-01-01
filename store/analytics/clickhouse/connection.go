package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/yadunandan004/scaffold/config"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Config represents ClickHouse connection configuration
type Config struct {
	Host         string
	Port         int
	Database     string
	Username     string
	Password     string
	MaxOpenConns int
	MaxIdleConns int
	Debug        bool
}

// Connection represents a ClickHouse database connection
type Connection struct {
	conn driver.Conn
	cfg  *Config
}

// NewConnection creates a new ClickHouse connection
func NewConnection(cfg *Config) (*Connection, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:     time.Duration(10) * time.Second,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: time.Duration(10) * time.Minute,
		Debug:           cfg.Debug,
	}

	if config.IsLocalEnv() {
		options.TLS = nil
	} else {
		options.TLS = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse connection: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}

	return &Connection{
		conn: conn,
		cfg:  cfg,
	}, nil
}

// GetConn returns the underlying ClickHouse connection
func (c *Connection) GetConn() driver.Conn {
	return c.conn
}

// Close closes the ClickHouse connection
func (c *Connection) Close() error {
	return c.conn.Close()
}

// Exec executes a query without returning rows
func (c *Connection) Exec(ctx context.Context, query string, args ...interface{}) error {
	return c.conn.Exec(ctx, query, args...)
}

// Query executes a query and returns rows
func (c *Connection) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	return c.conn.Query(ctx, query, args...)
}

// QueryRow executes a query and returns a single row
func (c *Connection) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	return c.conn.QueryRow(ctx, query, args...)
}

// PrepareBatch prepares a batch insert
func (c *Connection) PrepareBatch(ctx context.Context, query string) (driver.Batch, error) {
	return c.conn.PrepareBatch(ctx, query)
}
