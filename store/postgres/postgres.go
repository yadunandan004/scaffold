package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/yadunandan004/scaffold/config"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/ini.v1"
)

var (
	globalDB         *DB
	globalReadOnlyDB *DB
	dbMutex          sync.RWMutex
)

type DB struct {
	*sql.DB
	config *DatabaseConfig
}

type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	DBName       string `yaml:"dbname"`
	SSLMode      string `yaml:"sslmode"`
	SearchPath   string `yaml:"search_path"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

func BuildDSN(cfg *DatabaseConfig) string {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.SSLMode,
	)
	if cfg.SearchPath != "" {
		dsn += fmt.Sprintf(" search_path=%s", cfg.SearchPath)
	}
	return dsn
}

func configureConnectionPool(db *sql.DB, cfg *DatabaseConfig) error {
	// Set maximum number of open connections
	db.SetMaxOpenConns(cfg.MaxOpenConns)

	// Set maximum number of idle connections
	db.SetMaxIdleConns(cfg.MaxIdleConns)

	// Set maximum lifetime of a connection
	db.SetConnMaxLifetime(time.Hour)

	// Set maximum idle time of a connection
	db.SetConnMaxIdleTime(time.Minute * 5)

	return nil
}

func SetupConnectionFromDSN(dsn string, maxOpenConns, maxIdleConns int) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(time.Minute * 5)

	return db, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.DB.Close()
}

// Transaction executes a function within a database transaction
func (d *DB) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// HealthCheck verifies the database connection is alive
func (d *DB) HealthCheck() error {
	return d.DB.Ping()
}

// GetDB returns the global database instance
func GetDB() *DB {
	dbMutex.RLock()
	defer dbMutex.RUnlock()
	return globalDB
}

// SetGlobalDB sets the global database instance
func SetGlobalDB(sqlDB *sql.DB, cfg *DatabaseConfig) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if sqlDB == nil {
		return fmt.Errorf("cannot set nil database")
	}

	globalDB = &DB{
		DB:     sqlDB,
		config: cfg,
	}

	return nil
}

// GetDSN returns the database connection string (sentinel DSN)
func GetDSN() string {
	return GetSentinelDSN()
}

// Ping checks if the database connection is alive
func Ping(ctx context.Context) error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Use request for timeout control
	return db.DB.PingContext(ctx)
}

// configureReadOnlyConnectionPool configures connection pool optimized for read-only operations
func configureReadOnlyConnectionPool(db *sql.DB, cfg *DatabaseConfig) error {
	// Read-only connections can have more open connections
	db.SetMaxOpenConns(cfg.MaxOpenConns * 2)

	// And more idle connections since they're cheaper
	db.SetMaxIdleConns(cfg.MaxIdleConns * 2)

	// Set maximum lifetime of a connection
	db.SetConnMaxLifetime(time.Hour * 2)

	// Set maximum idle time of a connection
	db.SetConnMaxIdleTime(time.Minute * 10)

	return nil
}

// GetReadOnlyDB returns the global read-only database instance
func GetReadOnlyDB() *DB {
	dbMutex.RLock()
	defer dbMutex.RUnlock()
	return globalReadOnlyDB
}

func GetDBConfig(resolver *config.ConfigResolver) *DatabaseConfig {
	return &DatabaseConfig{
		Host:         resolver.GetString("database.host", "DB_HOST", "localhost"),
		Port:         resolver.GetInt("database.port", "DB_PORT", 5432),
		User:         resolver.GetString("database.user", "DB_USER", ""),
		Password:     resolver.GetString("database.password", "DB_PASSWORD", ""),
		DBName:       resolver.GetString("database.name", "DB_NAME", ""),
		SSLMode:      resolver.GetString("database.ssl_mode", "DB_SSL_MODE", "disable"),
		SearchPath:   resolver.GetString("database.search_path", "DB_SEARCH_PATH", ""),
		MaxOpenConns: resolver.GetInt("database.max_open_conns", "DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns: resolver.GetInt("database.max_idle_conns", "DB_MAX_IDLE_CONNS", 10),
	}
}

func GetDBConfigFromEnv() *DatabaseConfig {
	resolver := config.NewConfigResolver("")
	return GetDBConfig(resolver)
}

// NewMockConnection creates a test database using testcontainers (single node)
func NewMockConnection() (testcontainers.Container, error) {
	cluster, err := NewMockCluster(1)
	if err != nil {
		return nil, err
	}
	return cluster.Containers[0], nil
}

// MockCluster holds multi-node test cluster state
type MockCluster struct {
	Containers  []testcontainers.Container
	Network     testcontainers.Network
	NetworkName string
	Nodes       []MockNodeConfig
	SentinelDB  *sql.DB
}

// MockClusterOptions configures MockCluster behavior
type MockClusterOptions struct {
	UseTmpfs       bool // Use tmpfs for PG data (RAM disk - removes disk bottleneck)
	TmpfsSizeMB    int  // Size of tmpfs in MB (default 512)
	PartitionCount int  // Number of partitions (default 8)
}

// MockNodeConfig contains connection info for a test PG node
type MockNodeConfig struct {
	ID            int
	Host          string // Container hostname (for inter-container communication)
	ExternalHost  string // Host machine accessible host
	Port          int    // Always 5432 for inter-container
	ExternalPort  int    // Mapped port for host machine access
	Database      string
	User          string
	Password      string
	IsSentinel    bool
	Partitions    []int
	ContainerName string
}

// GetDSN returns DSN for connecting from host machine
func (n *MockNodeConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		n.ExternalHost, n.ExternalPort, n.User, n.Password, n.Database,
	)
}

// GetInternalDSN returns DSN for inter-container communication
func (n *MockNodeConfig) GetInternalDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		n.Host, n.Port, n.User, n.Password, n.Database,
	)
}

// NewMockCluster creates a multi-node PostgreSQL test cluster
// nodeCount=1: single PG (no foreign tables)
// nodeCount>1: sentinel + (nodeCount-1) remote nodes with foreign tables
func NewMockCluster(nodeCount int) (*MockCluster, error) {
	return NewMockClusterWithOptions(nodeCount, MockClusterOptions{})
}

// NewMockClusterWithOptions creates a multi-node PostgreSQL test cluster with options
// Use UseTmpfs=true to mount PG data on RAM disk (removes disk bottleneck for scaling tests)
func NewMockClusterWithOptions(nodeCount int, opts MockClusterOptions) (*MockCluster, error) {
	if nodeCount < 1 {
		return nil, fmt.Errorf("nodeCount must be at least 1")
	}

	ctx := context.Background()

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		log.Fatalf("Docker daemon is not running. Please start Docker and try again: %v", err)
	}
	defer provider.Close()

	cluster := &MockCluster{
		Containers: make([]testcontainers.Container, 0, nodeCount),
		Nodes:      make([]MockNodeConfig, 0, nodeCount),
	}

	// Create shared network for multi-node
	if nodeCount > 1 {
		networkName := fmt.Sprintf("furio-test-net-%d", time.Now().UnixNano())
		network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
			NetworkRequest: testcontainers.NetworkRequest{
				Name:   networkName,
				Driver: "bridge",
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create test network: %w", err)
		}
		cluster.Network = network
		cluster.NetworkName = networkName
	}

	// Calculate partition distribution
	partitionCount := opts.PartitionCount
	if partitionCount == 0 {
		partitionCount = 8 // Use smaller count for tests
	}
	partitionsPerNode := partitionCount / nodeCount

	// Tmpfs size
	tmpfsSizeMB := opts.TmpfsSizeMB
	if tmpfsSizeMB == 0 {
		tmpfsSizeMB = 512
	}

	// Start all PG containers
	for i := 0; i < nodeCount; i++ {
		nodeID := i + 1
		isSentinel := (i == 0)
		containerName := fmt.Sprintf("furio-pg-test-%d-%d", nodeID, time.Now().UnixNano())

		// Calculate partitions for this node
		var partitions []int
		startPart := i * partitionsPerNode
		endPart := startPart + partitionsPerNode
		if i == nodeCount-1 {
			endPart = partitionCount // Last node gets remainder
		}
		for p := startPart; p < endPart; p++ {
			partitions = append(partitions, p)
		}

		cfg := MockNodeConfig{
			ID:            nodeID,
			Host:          containerName, // Container name = hostname in Docker network
			Port:          5432,
			Database:      "testdb",
			User:          "testuser",
			Password:      "testpass",
			IsSentinel:    isSentinel,
			Partitions:    partitions,
			ContainerName: containerName,
		}

		req := testcontainers.ContainerRequest{
			Image:        "postgres:15-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Name:         containerName,
			Env: map[string]string{
				"POSTGRES_USER":     cfg.User,
				"POSTGRES_PASSWORD": cfg.Password,
				"POSTGRES_DB":       cfg.Database,
			},
			WaitingFor: wait.ForAll(
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
				wait.ForListeningPort("5432/tcp"),
			),
		}

		// Mount tmpfs for PG data if requested (RAM disk - removes disk bottleneck)
		if opts.UseTmpfs {
			req.Tmpfs = map[string]string{
				"/var/lib/postgresql/data": fmt.Sprintf("rw,noexec,nosuid,size=%dm", tmpfsSizeMB),
			}
			log.Printf("[TEST] Node %d using tmpfs (%dMB) for PG data", nodeID, tmpfsSizeMB)
		}

		// Add to shared network for multi-node
		if cluster.NetworkName != "" {
			req.Networks = []string{cluster.NetworkName}
			req.NetworkAliases = map[string][]string{
				cluster.NetworkName: {containerName},
			}
		}

		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			cluster.Cleanup()
			return nil, fmt.Errorf("failed to start postgres container %d: %w", nodeID, err)
		}

		// Get external host/port for host machine access
		host, err := container.Host(ctx)
		if err != nil {
			cluster.Cleanup()
			return nil, fmt.Errorf("failed to get container host: %w", err)
		}
		mappedPort, err := container.MappedPort(ctx, "5432")
		if err != nil {
			cluster.Cleanup()
			return nil, fmt.Errorf("failed to get mapped port: %w", err)
		}

		cfg.ExternalHost = host
		cfg.ExternalPort = mappedPort.Int()

		cluster.Containers = append(cluster.Containers, container)
		cluster.Nodes = append(cluster.Nodes, cfg)

		log.Printf("[TEST] Started PG node %d: %s (external: %s:%d, partitions: %v, sentinel: %v, tmpfs: %v)",
			nodeID, containerName, host, mappedPort.Int(), partitions, isSentinel, opts.UseTmpfs)
	}

	// Connect to sentinel and initialize global pool
	sentinel := cluster.Nodes[0]
	dsn := sentinel.GetDSN()
	db, err := SetupConnectionFromDSN(dsn, 100, 50) // Higher connection pool for benchmarks
	if err != nil {
		cluster.Cleanup()
		return nil, fmt.Errorf("failed to connect to sentinel: %w", err)
	}
	cluster.SentinelDB = db

	connections := []*PgNodeConnections{
		{
			ID:         1,
			PrimaryDB:  db,
			PrimaryDSN: dsn,
			IsSentinel: true,
		},
	}

	if err := InitMultiPgPool(connections); err != nil {
		cluster.Cleanup()
		return nil, fmt.Errorf("failed to initialize multi-PG pool: %w", err)
	}

	return cluster, nil
}

// Cleanup terminates all containers and network
func (c *MockCluster) Cleanup() {
	ctx := context.Background()

	if c.SentinelDB != nil {
		c.SentinelDB.Close()
	}

	for _, container := range c.Containers {
		if container != nil {
			container.Terminate(ctx)
		}
	}

	if c.Network != nil {
		c.Network.Remove(ctx)
	}
}

// GenerateINIContent creates postgres.ini content for schema manager
func (c *MockCluster) GenerateINIContent() string {
	var sb strings.Builder
	for i, node := range c.Nodes {
		sb.WriteString(fmt.Sprintf("[node%d]\n", i+1))
		sb.WriteString(fmt.Sprintf("id = %d\n", node.ID))
		sb.WriteString(fmt.Sprintf("sentinel = %v\n", node.IsSentinel))
		// Use internal hostname for inter-container communication (foreign tables)
		sb.WriteString(fmt.Sprintf("host = %s\n", node.Host))
		sb.WriteString(fmt.Sprintf("port = %d\n", node.Port))
		sb.WriteString(fmt.Sprintf("user = %s\n", node.User))
		sb.WriteString(fmt.Sprintf("password = %s\n", node.Password))
		sb.WriteString(fmt.Sprintf("dbname = %s\n", node.Database))
		sb.WriteString(fmt.Sprintf("partition_start = %d\n", node.Partitions[0]))
		sb.WriteString(fmt.Sprintf("partition_end = %d\n", node.Partitions[len(node.Partitions)-1]))
		sb.WriteString("\n")
	}
	return sb.String()
}

// GetPgNodeConfigs returns configs suitable for schema manager
func (c *MockCluster) GetPgNodeConfigs() []struct {
	ID         int
	Host       string
	Port       int
	Database   string
	User       string
	Password   string
	IsSentinel bool
	Partitions []int
} {
	configs := make([]struct {
		ID         int
		Host       string
		Port       int
		Database   string
		User       string
		Password   string
		IsSentinel bool
		Partitions []int
	}, len(c.Nodes))

	for i, node := range c.Nodes {
		configs[i] = struct {
			ID         int
			Host       string
			Port       int
			Database   string
			User       string
			Password   string
			IsSentinel bool
			Partitions []int
		}{
			ID:         node.ID,
			Host:       node.Host, // Internal hostname for foreign tables
			Port:       node.Port,
			Database:   node.Database,
			User:       node.User,
			Password:   node.Password,
			IsSentinel: node.IsSentinel,
			Partitions: node.Partitions,
		}
	}
	return configs
}

// ConnectToNode connects to a specific node (for schema manager connectFn)
func (c *MockCluster) ConnectToNode(nodeID int) (*sql.DB, error) {
	for _, node := range c.Nodes {
		if node.ID == nodeID {
			return SetupConnectionFromDSN(node.GetDSN(), 10, 5)
		}
	}
	return nil, fmt.Errorf("node %d not found", nodeID)
}

// MigrationPgNodeConfig mirrors migration.PgNodeConfig for test use
type MigrationPgNodeConfig struct {
	ID         int
	Host       string
	Port       int
	Database   string
	User       string
	Password   string
	IsSentinel bool
	Partitions []int
}

// ToMigrationConfigs returns configs suitable for schema manager
func (c *MockCluster) ToMigrationConfigs() []MigrationPgNodeConfig {
	configs := make([]MigrationPgNodeConfig, len(c.Nodes))
	for i, node := range c.Nodes {
		configs[i] = MigrationPgNodeConfig{
			ID:         node.ID,
			Host:       node.Host, // Internal hostname for foreign tables
			Port:       node.Port,
			Database:   node.Database,
			User:       node.User,
			Password:   node.Password,
			IsSentinel: node.IsSentinel,
			Partitions: node.Partitions,
		}
	}
	return configs
}

// BuildNodeDSN builds DSN for connecting to a node from host machine
func (c *MockCluster) BuildNodeDSN(nodeID int) (string, error) {
	for _, node := range c.Nodes {
		if node.ID == nodeID {
			return node.GetDSN(), nil
		}
	}
	return "", fmt.Errorf("node %d not found", nodeID)
}

// GetPartitionCount returns the number of partitions configured
func (c *MockCluster) GetPartitionCount() int {
	total := 0
	for _, node := range c.Nodes {
		total += len(node.Partitions)
	}
	return total
}

type PgNodeConnections struct {
	ID         int
	PrimaryDB  *sql.DB
	StandbyDB  *sql.DB
	IsSentinel bool
	PrimaryDSN string // stored for LISTEN connections
	Partitions []int  // workflow partitions owned by this node
}

func LoadPgNodesFromINI(iniPath string) ([]*PgNodeConnections, error) {
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("postgres.ini not found at %s", iniPath)
	}

	cfg, err := ini.Load(iniPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load postgres.ini: %w", err)
	}

	var connections []*PgNodeConnections
	nodeIndex := 1

	for {
		sectionName := fmt.Sprintf("node%d", nodeIndex)
		section, err := cfg.GetSection(sectionName)
		if err != nil {
			break
		}

		idKey, err := section.GetKey("id")
		if err != nil {
			return nil, fmt.Errorf("missing 'id' in [%s]", sectionName)
		}
		id, err := strconv.Atoi(idKey.String())
		if err != nil {
			return nil, fmt.Errorf("invalid 'id' in [%s]: %w", sectionName, err)
		}

		primaryKey, err := section.GetKey("primary")
		if err != nil {
			return nil, fmt.Errorf("missing 'primary' in [%s]", sectionName)
		}
		primaryDSN := primaryKey.String()
		if primaryDSN == "" {
			return nil, fmt.Errorf("empty 'primary' DSN in [%s]", sectionName)
		}

		primaryDB, err := SetupConnectionFromDSN(primaryDSN, 100, 10)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to primary node %d: %w", id, err)
		}
		log.Printf("Connected to PostgreSQL node %d primary", id)

		// Parse sentinel flag
		isSentinel := false
		if sentinelKey, err := section.GetKey("sentinel"); err == nil {
			isSentinel = sentinelKey.String() == "true"
		}

		conn := &PgNodeConnections{
			ID:         id,
			PrimaryDB:  primaryDB,
			PrimaryDSN: primaryDSN,
			IsSentinel: isSentinel,
		}

		standbyKey, err := section.GetKey("standby")
		if err == nil && standbyKey.String() != "" {
			standbyDSN := standbyKey.String()
			standbyDB, err := SetupConnectionFromDSN(standbyDSN, 100, 10)
			if err != nil {
				return nil, fmt.Errorf("failed to connect to standby node %d: %w", id, err)
			}
			conn.StandbyDB = standbyDB
			log.Printf("Connected to PostgreSQL node %d standby", id)
		}

		connections = append(connections, conn)
		nodeIndex++
	}

	if len(connections) == 0 {
		return nil, fmt.Errorf("no PostgreSQL nodes found in %s - minimum 1 node required", iniPath)
	}

	// Validate sentinel configuration
	if err := validateSentinelConfig(connections); err != nil {
		return nil, err
	}

	log.Printf("Initialized %d PostgreSQL nodes from %s", len(connections), iniPath)
	return connections, nil
}

func validateSentinelConfig(connections []*PgNodeConnections) error {
	sentinelCount := 0
	for _, conn := range connections {
		if conn.IsSentinel {
			sentinelCount++
		}
	}

	// Single node: auto-assume sentinel (no marking needed)
	if len(connections) == 1 {
		connections[0].IsSentinel = true
		log.Printf("Single node configuration: node %d auto-assigned as sentinel", connections[0].ID)
		return nil
	}

	// Multi-node: MUST have exactly one sentinel marked
	if sentinelCount == 0 {
		return fmt.Errorf("multi-node setup requires exactly one node marked as sentinel=true in postgres.ini")
	}

	if sentinelCount > 1 {
		return fmt.Errorf("only one node can be marked as sentinel, found %d", sentinelCount)
	}

	// Log which node is sentinel
	for _, conn := range connections {
		if conn.IsSentinel {
			log.Printf("Node %d designated as sentinel", conn.ID)
			break
		}
	}

	return nil
}

func InitSentinelFromEnv() error {
	cfg := GetDBConfigFromEnv()
	if cfg.Host == "" {
		return fmt.Errorf("DB_HOST environment variable is required")
	}
	if cfg.User == "" {
		return fmt.Errorf("DB_USER environment variable is required")
	}
	if cfg.DBName == "" {
		return fmt.Errorf("DB_NAME environment variable is required")
	}

	dsn := BuildDSN(cfg)
	db, err := SetupConnectionFromDSN(dsn, cfg.MaxOpenConns, cfg.MaxIdleConns)
	if err != nil {
		return fmt.Errorf("failed to connect to sentinel: %w", err)
	}

	connections := []*PgNodeConnections{
		{
			ID:         1,
			PrimaryDB:  db,
			PrimaryDSN: dsn,
			IsSentinel: true,
		},
	}

	if err := InitMultiPgPool(connections); err != nil {
		db.Close()
		return fmt.Errorf("failed to initialize pool: %w", err)
	}

	// Dynamically assign partitions to the single sentinel node
	partitionCount := getPartitionCountFromEnv()
	if err := AssignPartitions(partitionCount); err != nil {
		db.Close()
		return fmt.Errorf("failed to assign partitions: %w", err)
	}

	log.Printf("Connected to sentinel PostgreSQL at %s:%d with %d partitions (foreign tables route writes to remote nodes)", cfg.Host, cfg.Port, partitionCount)
	return nil
}

// getPartitionCountFromEnv reads partition count from environment or returns default
func getPartitionCountFromEnv() int {
	if countStr := os.Getenv("POSTGRES_PARTITION_COUNT"); countStr != "" {
		if count, err := strconv.Atoi(countStr); err == nil && count > 0 {
			return count
		}
	}
	return 8 // default partition count
}
