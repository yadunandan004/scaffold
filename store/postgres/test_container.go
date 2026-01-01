package postgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewMockConnectionWithMigrations creates a test database using testcontainers and runs migrations
func NewMockConnectionWithMigrations(migrationPath string) (testcontainers.Container, error) {
	ctx := context.Background()

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		log.Fatalf("Docker daemon is not running. Please start Docker and try again: %v", err)
	}
	defer provider.Close()

	cfg := &DatabaseConfig{
		User:         "testuser",
		Password:     "testpass",
		DBName:       "testdb",
		SSLMode:      "disable",
		MaxOpenConns: 1000,
		MaxIdleConns: 500,
		SearchPath:   "automation,public",
	}

	// Get partition count from environment or use default for tests
	partitionCount := os.Getenv("POSTGRES_PARTITION_COUNT")
	if partitionCount == "" {
		partitionCount = "2" // Default for testcontainers (small for faster tests)
	}

	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     cfg.User,
			"POSTGRES_PASSWORD": cfg.Password,
			"POSTGRES_DB":       cfg.DBName,
		},
		Cmd: []string{
			"postgres",
			"-c", "max_locks_per_transaction=256",
			"-c", "max_pred_locks_per_transaction=256",
			"-c", "max_connections=200",
			"-c", fmt.Sprintf("app.partition_count=%s", partitionCount),
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(120*time.Second),
			wait.ForListeningPort("5432/tcp"),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	cfg.Host = host
	cfg.Port = mappedPort.Int()

	dsn := BuildDSN(cfg)
	db, err := SetupConnectionFromDSN(dsn, cfg.MaxOpenConns, cfg.MaxIdleConns)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	connections := []*PgNodeConnections{
		{
			ID:         1,
			PrimaryDB:  db,
			IsSentinel: true,
			PrimaryDSN: dsn,
		},
	}

	if err := InitMultiPgPool(connections); err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to initialize multi-PG pool: %w", err)
	}

	log.Println("Database connection established successfully")
	return container, nil
}

// migrationFile holds migration file information
type migrationFile struct {
	name    string
	content string
}

// readMigrationFiles reads all SQL migration files from the specified directory
func readMigrationFiles(migrationPath string) ([]string, error) {
	files, err := os.ReadDir(migrationPath)
	if err != nil {
		return nil, err
	}

	var migrationFiles []migrationFile
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") && !strings.Contains(file.Name(), ".down.") {
			content, err := os.ReadFile(filepath.Join(migrationPath, file.Name()))
			if err != nil {
				return nil, fmt.Errorf("failed to read migration file %s: %w", file.Name(), err)
			}
			migrationFiles = append(migrationFiles, migrationFile{
				name:    file.Name(),
				content: string(content),
			})
		}
	}

	// Sort migrations by filename to ensure correct order
	sort.Slice(migrationFiles, func(i, j int) bool {
		return migrationFiles[i].name < migrationFiles[j].name
	})

	// Extract just the content in sorted order
	var migrations []string
	for _, mf := range migrationFiles {
		log.Printf("Including migration: %s", mf.name)
		migrations = append(migrations, mf.content)
	}

	return migrations, nil
}

// createInitScript creates a single SQL script from all migration files
func createInitScript(migrations []string) string {
	var script strings.Builder

	// Add initial setup
	script.WriteString("-- Auto-generated init script for test database\n")
	script.WriteString("SET client_min_messages TO WARNING;\n\n")

	// Add all migrations
	for i, migration := range migrations {
		script.WriteString(fmt.Sprintf("-- Migration %d\n", i+1))
		script.WriteString(migration)
		// Ensure each migration ends with a semicolon
		if !strings.HasSuffix(strings.TrimSpace(migration), ";") {
			script.WriteString(";")
		}
		script.WriteString("\n\n")
	}

	return script.String()
}

// createInitScriptWithConfig creates a single SQL script with partition count configuration
func createInitScriptWithConfig(migrations []string, partitionCount string) string {
	var script strings.Builder

	// Add initial setup
	script.WriteString("-- Auto-generated init script for test database\n")
	script.WriteString("SET client_min_messages TO WARNING;\n\n")

	// Set partition count configuration BEFORE running migrations
	script.WriteString("-- Configure partition count for dynamic partition creation\n")
	script.WriteString(fmt.Sprintf("ALTER DATABASE testdb SET app.partition_count = %s;\n", partitionCount))
	script.WriteString("-- Reload configuration\n")
	script.WriteString("SELECT pg_reload_conf();\n\n")

	// Add all migrations
	for i, migration := range migrations {
		script.WriteString(fmt.Sprintf("-- Migration %d\n", i+1))
		script.WriteString(migration)
		// Ensure each migration ends with a semicolon
		if !strings.HasSuffix(strings.TrimSpace(migration), ";") {
			script.WriteString(";")
		}
		script.WriteString("\n\n")
	}

	return script.String()
}

// createTempFile creates a temporary file with the given content
func createTempFile(content string) string {
	file, err := os.CreateTemp("", "pg-init-*.sql")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		log.Fatal(err)
	}

	return file.Name()
}
