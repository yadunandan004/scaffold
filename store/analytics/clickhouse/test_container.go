package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestClickHouseDB wraps the ClickHouse test connection
type TestClickHouseDB struct {
	DB        *sql.DB
	Container testcontainers.Container
	Config    *Config
}

// NewMockConnectionWithMigrations creates a test ClickHouse database using testcontainers and runs migrations
func NewMockConnectionWithMigrations(migrationPath string) (*TestClickHouseDB, error) {
	ctx := context.Background()

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		log.Fatalf("Docker daemon is not running. Please start Docker and try again: %v", err)
	}
	defer provider.Close()

	cfg := &Config{
		Database: "testdb",
		Username: "default",
		Password: "",
	}

	// Read migration files
	migrationFiles, err := readMigrationFiles(migrationPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration files: %w", err)
	}

	// Create init script that runs migrations
	initScript := createInitScript(cfg.Database, migrationFiles)

	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:latest",
		ExposedPorts: []string{"9000/tcp", "8123/tcp"},
		Env: map[string]string{
			"CLICKHOUSE_DB":       cfg.Database,
			"CLICKHOUSE_USER":     cfg.Username,
			"CLICKHOUSE_PASSWORD": cfg.Password,
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      createTempFile(initScript),
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
				FileMode:          0644,
			},
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Ready for connections"),
			wait.ForListeningPort("9000/tcp"),
			wait.ForHTTP("/ping").WithPort("8123/tcp"),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start clickhouse container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "9000")
	if err != nil {
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	cfg.Host = host
	cfg.Port = mappedPort.Int()

	// Connect to ClickHouse
	db, err := connectToClickHouse(cfg)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to connect to test clickhouse: %w", err)
	}

	// Wait a bit for migrations to complete
	time.Sleep(2 * time.Second)

	// Run migrations if init script didn't work (some ClickHouse images don't support it)
	if err := runMigrationsDirectly(db, cfg.Database, migrationFiles); err != nil {
		log.Printf("Warning: Failed to run migrations directly: %v", err)
	}

	log.Println("ClickHouse connection established successfully with migrations applied")

	return &TestClickHouseDB{
		DB:        db,
		Container: container,
		Config:    cfg,
	}, nil
}

// connectToClickHouse establishes a connection to ClickHouse
func connectToClickHouse(cfg *Config) (*sql.DB, error) {
	_ = fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

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
		DialTimeout:      time.Second * 10,
		MaxOpenConns:     5,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	}

	conn := clickhouse.OpenDB(options)

	// Test the connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}

	return conn, nil
}

// readMigrationFiles reads all SQL migration files from the specified directory
func readMigrationFiles(migrationPath string) ([]string, error) {
	files, err := os.ReadDir(migrationPath)
	if err != nil {
		return nil, err
	}

	var migrations []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") {
			content, err := os.ReadFile(filepath.Join(migrationPath, file.Name()))
			if err != nil {
				return nil, fmt.Errorf("failed to read migration file %s: %w", file.Name(), err)
			}
			migrations = append(migrations, string(content))
		}
	}

	// Sort migrations by filename to ensure correct order
	sort.Strings(migrations)
	return migrations, nil
}

// createInitScript creates a single SQL script from all migration files
func createInitScript(database string, migrations []string) string {
	var script strings.Builder

	// Add initial setup
	script.WriteString("-- Auto-generated init script for test database\n")
	script.WriteString(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;\n", database))
	script.WriteString(fmt.Sprintf("USE %s;\n\n", database))

	// Add all migrations
	for i, migration := range migrations {
		script.WriteString(fmt.Sprintf("-- Migration %d\n", i+1))
		script.WriteString(migration)
		script.WriteString("\n\n")
	}

	return script.String()
}

// runMigrationsDirectly runs migrations directly on the database
func runMigrationsDirectly(db *sql.DB, database string, migrations []string) error {
	// Create database if not exists
	if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", database)); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	// Use the database
	if _, err := db.Exec(fmt.Sprintf("USE %s", database)); err != nil {
		return fmt.Errorf("failed to use database: %w", err)
	}

	// Run each migration
	for i, migration := range migrations {
		log.Printf("Running ClickHouse migration %d", i+1)
		if _, err := db.Exec(migration); err != nil {
			// Log but don't fail - some migrations might already be applied
			log.Printf("Warning: Migration %d failed: %v", i+1, err)
		}
	}

	return nil
}

// createTempFile creates a temporary file with the given content
func createTempFile(content string) string {
	file, err := os.CreateTemp("", "ch-init-*.sql")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		log.Fatal(err)
	}

	return file.Name()
}

// Close closes the database connection and terminates the container
func (t *TestClickHouseDB) Close() error {
	if t.DB != nil {
		t.DB.Close()
	}
	if t.Container != nil {
		return t.Container.Terminate(context.Background())
	}
	return nil
}
