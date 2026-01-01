package postgres

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
)

type MultiPgPool struct {
	nodes           map[int]*PgNodeConnections
	nodeCount       int
	sentinelID      int
	partitionToNode map[int]int
	mu              sync.RWMutex
}

var (
	globalMultiPgPool *MultiPgPool
	poolMutex         sync.RWMutex
)

func InitMultiPgPool(connections []*PgNodeConnections) error {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	globalMultiPgPool = &MultiPgPool{
		nodes:           make(map[int]*PgNodeConnections),
		nodeCount:       len(connections),
		partitionToNode: make(map[int]int),
	}

	for _, conn := range connections {
		globalMultiPgPool.nodes[conn.ID] = conn

		if conn.IsSentinel {
			globalMultiPgPool.sentinelID = conn.ID
			if err := SetGlobalDB(conn.PrimaryDB, nil); err != nil {
				return fmt.Errorf("failed to set sentinel as globalDB: %w", err)
			}
			log.Printf("Sentinel node %d set as globalDB", conn.ID)
		}
	}

	if globalMultiPgPool.sentinelID == 0 {
		return fmt.Errorf("no sentinel node found in connections")
	}

	return nil
}

func CloseMultiPgPool() error {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	if globalMultiPgPool == nil {
		return nil
	}

	globalMultiPgPool.mu.Lock()
	defer globalMultiPgPool.mu.Unlock()

	for _, conn := range globalMultiPgPool.nodes {
		if conn.PrimaryDB != nil {
			conn.PrimaryDB.Close()
		}
		if conn.StandbyDB != nil {
			conn.StandbyDB.Close()
		}
	}

	return nil
}

func GetSentinelDSN() string {
	poolMutex.RLock()
	defer poolMutex.RUnlock()

	if globalMultiPgPool == nil {
		return ""
	}

	globalMultiPgPool.mu.RLock()
	defer globalMultiPgPool.mu.RUnlock()

	conn, exists := globalMultiPgPool.nodes[globalMultiPgPool.sentinelID]
	if !exists {
		return ""
	}

	return conn.PrimaryDSN
}

// ErrPoolNotInitialized is returned when MultiPgPool operations are called before initialization.
var ErrPoolNotInitialized = fmt.Errorf("MultiPgPool not initialized - call InitMultiPgPool first")

// GetDBForPartition returns the database connection for the node that owns the given partition.
// Returns an error if the pool is not initialized or the partition is not assigned.
func GetDBForPartition(partition int) (*sql.DB, error) {
	poolMutex.RLock()
	defer poolMutex.RUnlock()

	if globalMultiPgPool == nil {
		return nil, ErrPoolNotInitialized
	}

	globalMultiPgPool.mu.RLock()
	defer globalMultiPgPool.mu.RUnlock()

	nodeID, exists := globalMultiPgPool.partitionToNode[partition]
	if !exists {
		return nil, fmt.Errorf("partition %d not assigned to any node - call AssignPartitions first or check config", partition)
	}

	conn, exists := globalMultiPgPool.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %d not found in pool - this indicates a configuration error", nodeID)
	}

	return conn.PrimaryDB, nil
}

// GetPartitionCount returns the total number of configured partitions.
func GetPartitionCount() int {
	poolMutex.RLock()
	defer poolMutex.RUnlock()

	if globalMultiPgPool == nil {
		return 0
	}

	globalMultiPgPool.mu.RLock()
	defer globalMultiPgPool.mu.RUnlock()

	return len(globalMultiPgPool.partitionToNode)
}

// AssignPartitions distributes partitions evenly across all nodes.
// Must be called after InitMultiPgPool and before using partition routing.
// partitionCount is the total number of partitions (e.g., 32).
func AssignPartitions(partitionCount int) error {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	if globalMultiPgPool == nil {
		return fmt.Errorf("MultiPgPool not initialized - call InitMultiPgPool first")
	}

	globalMultiPgPool.mu.Lock()
	defer globalMultiPgPool.mu.Unlock()

	nodeCount := len(globalMultiPgPool.nodes)
	if nodeCount == 0 {
		return fmt.Errorf("no nodes in pool")
	}

	// Build ordered list of node IDs (sorted for deterministic assignment)
	nodeIDs := make([]int, 0, nodeCount)
	for id := range globalMultiPgPool.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	// Simple sort - node IDs are typically 1, 2, 3...
	for i := 0; i < len(nodeIDs)-1; i++ {
		for j := i + 1; j < len(nodeIDs); j++ {
			if nodeIDs[i] > nodeIDs[j] {
				nodeIDs[i], nodeIDs[j] = nodeIDs[j], nodeIDs[i]
			}
		}
	}

	// Calculate partitions per node
	partitionsPerNode := partitionCount / nodeCount
	remainder := partitionCount % nodeCount

	// Clear existing partition mappings
	globalMultiPgPool.partitionToNode = make(map[int]int)

	// Assign partitions to nodes
	partitionIdx := 0
	for i, nodeID := range nodeIDs {
		conn := globalMultiPgPool.nodes[nodeID]
		conn.Partitions = nil // Reset

		nodePartitions := partitionsPerNode
		if i < remainder {
			nodePartitions++ // Distribute remainder to first nodes
		}

		for j := 0; j < nodePartitions; j++ {
			conn.Partitions = append(conn.Partitions, partitionIdx)
			globalMultiPgPool.partitionToNode[partitionIdx] = nodeID
			partitionIdx++
		}

		log.Printf("Node %d assigned partitions: %v", nodeID, conn.Partitions)
	}

	log.Printf("Partition routing configured: %d partitions across %d nodes", partitionCount, nodeCount)
	return nil
}
