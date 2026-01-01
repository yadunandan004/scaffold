package clickhouse

import (
	"context"
	"fmt"
	"sync"
)

var (
	globalConn *Connection
	connMutex  sync.RWMutex
)

// SetGlobalConnection sets the global ClickHouse connection
func SetGlobalConnection(conn *Connection) {
	connMutex.Lock()
	defer connMutex.Unlock()
	globalConn = conn
}

// GetGlobalConnection returns the global ClickHouse connection
func GetGlobalConnection() *Connection {
	connMutex.RLock()
	defer connMutex.RUnlock()
	return globalConn
}

// Ping checks if the ClickHouse connection is alive
func Ping(ctx context.Context) error {
	conn := GetGlobalConnection()
	if conn == nil {
		return fmt.Errorf("clickhouse connection not initialized")
	}

	return conn.conn.Ping(ctx)
}
