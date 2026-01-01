package orm

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupTestNodesForTxn(t *testing.T) {
	t.Helper()
	_, _ = scannerTestDB.Exec("DELETE FROM test_nodes")
}

func beginTxn(t *testing.T) *Query {
	t.Helper()
	txn, err := scannerTestDB.Begin()
	require.NoError(t, err)
	return &Query{
		Ctx:     context.Background(),
		Txn:     txn,
		Scanner: &RawScanner{},
	}
}

func TestTransaction_Create(t *testing.T) {
	cleanupTestNodesForTxn(t)

	count := 42
	testNode := &TestNode{
		ID:   uuid.New(),
		Name: "txn-create-test",
		Tags: []string{"tag1", "tag2"},
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": float64(123),
		},
		Config: TestConfig{
			Enabled: true,
			Level:   5,
			Mode:    "test",
		},
		Attributes: map[string]float64{
			"x": 1.5,
			"y": 2.5,
		},
		Items: []TestItem{
			{Name: "item1", Value: 10},
			{Name: "item2", Value: 20},
		},
		Settings: &TestSettings{
			Theme: "dark",
			Size:  14,
		},
		Count:     &count,
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	query := beginTxn(t)
	defer query.Rollback()

	txn := NewTransaction[TestNode]()
	err := txn.Create(query, testNode)
	require.NoError(t, err)

	err = query.Commit()
	require.NoError(t, err)

	query2 := beginTxn(t)
	defer query2.Rollback()

	var result TestNode
	err = txn.FindByPK(query2, &result, testNode.ID)
	require.NoError(t, err)

	assert.Equal(t, testNode.ID, result.ID)
	assert.Equal(t, testNode.Name, result.Name)
	assert.Equal(t, testNode.Tags, result.Tags)
	assert.Equal(t, testNode.Config, result.Config)
	assert.Equal(t, testNode.Attributes, result.Attributes)
	assert.Equal(t, testNode.Items, result.Items)
	assert.NotNil(t, result.Settings)
	assert.Equal(t, testNode.Settings.Theme, result.Settings.Theme)
	assert.NotNil(t, result.Count)
	assert.Equal(t, *testNode.Count, *result.Count)
}

func TestTransaction_Update(t *testing.T) {
	cleanupTestNodesForTxn(t)

	count := 10
	testNode := &TestNode{
		ID:         uuid.New(),
		Name:       "txn-update-test",
		Tags:       []string{"old"},
		Metadata:   map[string]interface{}{"version": float64(1)},
		Config:     TestConfig{Enabled: false, Level: 1, Mode: "old"},
		Attributes: map[string]float64{"a": 1.0},
		Items:      []TestItem{{Name: "old", Value: 1}},
		Settings:   &TestSettings{Theme: "light", Size: 12},
		Count:      &count,
		CreatedAt:  time.Now().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().Truncate(time.Microsecond),
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.Create(query, testNode)
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	newCount := 20
	testNode.Tags = []string{"new", "updated"}
	testNode.Metadata = map[string]interface{}{"version": float64(2)}
	testNode.Config = TestConfig{Enabled: true, Level: 5, Mode: "new"}
	testNode.Attributes = map[string]float64{"b": 2.0}
	testNode.Items = []TestItem{{Name: "new", Value: 100}}
	testNode.Settings = &TestSettings{Theme: "dark", Size: 16}
	testNode.Count = &newCount
	testNode.UpdatedAt = time.Now().Truncate(time.Microsecond)

	query2 := beginTxn(t)
	err = txn.Update(query2, testNode)
	require.NoError(t, err)
	err = query2.Commit()
	require.NoError(t, err)

	query3 := beginTxn(t)
	defer query3.Rollback()

	var result TestNode
	err = txn.FindByPK(query3, &result, testNode.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{"new", "updated"}, result.Tags)
	assert.Equal(t, float64(2), result.Metadata["version"])
	assert.Equal(t, "new", result.Config.Mode)
	assert.Equal(t, "dark", result.Settings.Theme)
	assert.NotNil(t, result.Count)
	assert.Equal(t, 20, *result.Count)
}

func TestTransaction_Delete(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "txn-delete-test",
		Config:    TestConfig{Enabled: true, Level: 1, Mode: "test"},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.Create(query, testNode)
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	query2 := beginTxn(t)
	err = txn.Delete(query2, testNode)
	require.NoError(t, err)
	err = query2.Commit()
	require.NoError(t, err)

	var count int
	err = scannerTestDB.QueryRow("SELECT COUNT(*) FROM test_nodes WHERE id = $1", testNode.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestTransaction_CreateMultiple(t *testing.T) {
	cleanupTestNodesForTxn(t)

	nodes := []*TestNode{
		{
			ID:        uuid.New(),
			Name:      "batch-1",
			Config:    TestConfig{Enabled: true, Level: 1, Mode: "m1"},
			Tags:      []string{"a"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "batch-2",
			Config:    TestConfig{Enabled: false, Level: 2, Mode: "m2"},
			Tags:      []string{"b"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "batch-3",
			Config:    TestConfig{Enabled: true, Level: 3, Mode: "m3"},
			Tags:      []string{"c"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.CreateMultiple(query, nodes)
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	var count int
	err = scannerTestDB.QueryRow("SELECT COUNT(*) FROM test_nodes").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestTransaction_UpdateMultiple(t *testing.T) {
	cleanupTestNodesForTxn(t)

	nodes := []*TestNode{
		{
			ID:        uuid.New(),
			Name:      "update-multi-1",
			Config:    TestConfig{Enabled: false, Level: 1, Mode: "old"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "update-multi-2",
			Config:    TestConfig{Enabled: false, Level: 2, Mode: "old"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.CreateMultiple(query, nodes)
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	for _, node := range nodes {
		node.Config.Mode = "updated"
	}

	query2 := beginTxn(t)
	err = txn.UpdateMultiple(query2, nodes)
	require.NoError(t, err)
	err = query2.Commit()
	require.NoError(t, err)

	for _, node := range nodes {
		var result TestNode
		metadata := GetMetadata[TestNode]()
		scanner := NewScanner(metadata)
		rows, err := scannerTestDB.Query("SELECT * FROM test_nodes WHERE id = $1", node.ID)
		require.NoError(t, err)
		require.True(t, rows.Next())
		err = scanner.ScanRow(rows, &result)
		rows.Close()
		require.NoError(t, err)
		assert.Equal(t, "updated", result.Config.Mode)
	}
}

func TestTransaction_DeleteMultiple(t *testing.T) {
	cleanupTestNodesForTxn(t)

	nodes := []*TestNode{
		{
			ID:        uuid.New(),
			Name:      "del-multi-1",
			Config:    TestConfig{Enabled: true, Level: 1, Mode: "test"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "del-multi-2",
			Config:    TestConfig{Enabled: true, Level: 2, Mode: "test"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.CreateMultiple(query, nodes)
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	query2 := beginTxn(t)
	err = txn.DeleteMultiple(query2, nodes)
	require.NoError(t, err)
	err = query2.Commit()
	require.NoError(t, err)

	var count int
	err = scannerTestDB.QueryRow("SELECT COUNT(*) FROM test_nodes").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestTransaction_FindByPK(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "findby-pk-test",
		Tags:      []string{"find", "me"},
		Metadata:  map[string]interface{}{"findable": true},
		Config:    TestConfig{Enabled: true, Level: 99, Mode: "find"},
		Settings:  &TestSettings{Theme: "light", Size: 10},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.Create(query, testNode)
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	query2 := beginTxn(t)
	var result TestNode
	err = txn.FindByPK(query2, &result, testNode.ID)
	require.NoError(t, err)
	query2.Commit()

	assert.Equal(t, testNode.ID, result.ID)
	assert.Equal(t, testNode.Name, result.Name)
	assert.Equal(t, testNode.Tags, result.Tags)
	assert.Equal(t, testNode.Config, result.Config)
	assert.NotNil(t, result.Settings)
	assert.Equal(t, "light", result.Settings.Theme)
}

func TestTransaction_FindAll(t *testing.T) {
	cleanupTestNodesForTxn(t)

	nodes := []*TestNode{
		{
			ID:        uuid.New(),
			Name:      "findall-1",
			Config:    TestConfig{Enabled: true, Level: 1, Mode: "test"},
			Tags:      []string{"all-1"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "findall-2",
			Config:    TestConfig{Enabled: false, Level: 2, Mode: "test"},
			Tags:      []string{"all-2"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "findall-3",
			Config:    TestConfig{Enabled: true, Level: 3, Mode: "test"},
			Tags:      []string{"all-3"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.CreateMultiple(query, nodes)
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	query2 := beginTxn(t)
	results, err := txn.FindAll(query2)
	require.NoError(t, err)
	query2.Commit()

	require.Len(t, results, 3)
	for i, result := range results {
		assert.Equal(t, nodes[i].Name, result.Name)
		assert.Equal(t, nodes[i].Config.Level, result.Config.Level)
	}
}

func TestTransaction_Upsert_Insert(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "upsert-new",
		Config:    TestConfig{Enabled: true, Level: 5, Mode: "insert"},
		Tags:      []string{"upsert"},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.Upsert(query, testNode, []string{"name"})
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	var count int
	err = scannerTestDB.QueryRow("SELECT COUNT(*) FROM test_nodes WHERE name = $1", "upsert-new").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestTransaction_Upsert_Update(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "upsert-existing",
		Config:    TestConfig{Enabled: false, Level: 1, Mode: "old"},
		Tags:      []string{"old"},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.Create(query, testNode)
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	testNode.Config = TestConfig{Enabled: true, Level: 10, Mode: "updated"}
	testNode.Tags = []string{"new"}

	query2 := beginTxn(t)
	err = txn.Upsert(query2, testNode, []string{"name"})
	require.NoError(t, err)
	err = query2.Commit()
	require.NoError(t, err)

	var result TestNode
	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)
	rows, err := scannerTestDB.Query("SELECT * FROM test_nodes WHERE name = $1", "upsert-existing")
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.Equal(t, "updated", result.Config.Mode)
	assert.Equal(t, 10, result.Config.Level)
	assert.Equal(t, []string{"new"}, result.Tags)
}

func TestTransaction_CommitRollback(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "rollback-test",
		Config:    TestConfig{Enabled: true, Level: 1, Mode: "test"},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.Create(query, testNode)
	require.NoError(t, err)
	err = query.Rollback()
	require.NoError(t, err)

	var count int
	err = scannerTestDB.QueryRow("SELECT COUNT(*) FROM test_nodes WHERE id = $1", testNode.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	query2 := beginTxn(t)
	err = txn.Create(query2, testNode)
	require.NoError(t, err)
	err = query2.Commit()
	require.NoError(t, err)

	err = scannerTestDB.QueryRow("SELECT COUNT(*) FROM test_nodes WHERE id = $1", testNode.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestDB_Create(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "db-create-test",
		Tags:      []string{"db", "test"},
		Metadata:  map[string]interface{}{"type": "db"},
		Config:    TestConfig{Enabled: true, Level: 7, Mode: "db"},
		Settings:  &TestSettings{Theme: "blue", Size: 16},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	db := NewDB[TestNode](scannerTestDB)
	err := db.Create(context.Background(), testNode)
	require.NoError(t, err)

	var result TestNode
	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)
	rows, err := scannerTestDB.Query("SELECT * FROM test_nodes WHERE id = $1", testNode.ID)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.Equal(t, testNode.Name, result.Name)
	assert.Equal(t, testNode.Tags, result.Tags)
	assert.NotNil(t, result.Settings)
	assert.Equal(t, "blue", result.Settings.Theme)
}

func TestDB_Update(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "db-update-test",
		Config:    TestConfig{Enabled: false, Level: 1, Mode: "old"},
		Tags:      []string{"old"},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	db := NewDB[TestNode](scannerTestDB)
	err := db.Create(context.Background(), testNode)
	require.NoError(t, err)

	testNode.Config.Mode = "new"
	testNode.Tags = []string{"new"}
	err = db.Update(context.Background(), testNode)
	require.NoError(t, err)

	var result TestNode
	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)
	rows, err := scannerTestDB.Query("SELECT * FROM test_nodes WHERE id = $1", testNode.ID)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.Equal(t, "new", result.Config.Mode)
	assert.Equal(t, []string{"new"}, result.Tags)
}

func TestDB_Delete(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "db-delete-test",
		Config:    TestConfig{Enabled: true, Level: 1, Mode: "test"},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	db := NewDB[TestNode](scannerTestDB)
	err := db.Create(context.Background(), testNode)
	require.NoError(t, err)

	err = db.Delete(context.Background(), testNode)
	require.NoError(t, err)

	var count int
	err = scannerTestDB.QueryRow("SELECT COUNT(*) FROM test_nodes WHERE id = $1", testNode.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestDB_CreateMultiple(t *testing.T) {
	cleanupTestNodesForTxn(t)

	nodes := []*TestNode{
		{
			ID:        uuid.New(),
			Name:      "db-batch-1",
			Config:    TestConfig{Enabled: true, Level: 1, Mode: "db"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "db-batch-2",
			Config:    TestConfig{Enabled: false, Level: 2, Mode: "db"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
	}

	db := NewDB[TestNode](scannerTestDB)
	err := db.CreateMultiple(context.Background(), nodes)
	require.NoError(t, err)

	var count int
	err = scannerTestDB.QueryRow("SELECT COUNT(*) FROM test_nodes").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestDB_FindByPK(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "db-findpk-test",
		Tags:      []string{"find"},
		Config:    TestConfig{Enabled: true, Level: 50, Mode: "find"},
		Settings:  &TestSettings{Theme: "green", Size: 18},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	db := NewDB[TestNode](scannerTestDB)
	err := db.Create(context.Background(), testNode)
	require.NoError(t, err)

	var result TestNode
	err = db.FindByPK(context.Background(), &result, testNode.ID)
	require.NoError(t, err)

	assert.Equal(t, testNode.Name, result.Name)
	assert.Equal(t, testNode.Tags, result.Tags)
	assert.NotNil(t, result.Settings)
	assert.Equal(t, "green", result.Settings.Theme)
}

func TestDB_FindAll(t *testing.T) {
	cleanupTestNodesForTxn(t)

	nodes := []*TestNode{
		{
			ID:        uuid.New(),
			Name:      "db-findall-1",
			Config:    TestConfig{Enabled: true, Level: 1, Mode: "test"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "db-findall-2",
			Config:    TestConfig{Enabled: false, Level: 2, Mode: "test"},
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
	}

	db := NewDB[TestNode](scannerTestDB)
	err := db.CreateMultiple(context.Background(), nodes)
	require.NoError(t, err)

	results, err := db.FindAll(context.Background())
	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestDB_Upsert(t *testing.T) {
	cleanupTestNodesForTxn(t)

	testNode := &TestNode{
		ID:        uuid.New(),
		Name:      "db-upsert-test",
		Config:    TestConfig{Enabled: false, Level: 1, Mode: "old"},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	db := NewDB[TestNode](scannerTestDB)
	err := db.Upsert(context.Background(), testNode, []string{"name"})
	require.NoError(t, err)

	testNode.Config.Mode = "updated"
	err = db.Upsert(context.Background(), testNode, []string{"name"})
	require.NoError(t, err)

	var result TestNode
	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)
	rows, err := scannerTestDB.Query("SELECT * FROM test_nodes WHERE name = $1", "db-upsert-test")
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.Equal(t, "updated", result.Config.Mode)

	var count int
	err = scannerTestDB.QueryRow("SELECT COUNT(*) FROM test_nodes WHERE name = $1", "db-upsert-test").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestTransaction_Create_RawMessage(t *testing.T) {
	cleanupTestNodesForTxn(t)

	rawJSON := json.RawMessage([]byte(`{"workflow":"automation","version":2,"config":{"enabled":true}}`))

	testNode := &TestNode{
		ID:         uuid.New(),
		Name:       "txn-rawmessage-test",
		Tags:       []string{"raw"},
		Metadata:   map[string]interface{}{},
		Config:     TestConfig{Enabled: true, Level: 1, Mode: "test"},
		Attributes: map[string]float64{},
		Items:      []TestItem{},
		RawData:    rawJSON,
		CreatedAt:  time.Now().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().Truncate(time.Microsecond),
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()

	err := txn.Create(query, testNode)
	require.NoError(t, err)

	err = query.Commit()
	require.NoError(t, err)

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query("SELECT * FROM test_nodes WHERE id = $1", testNode.ID)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())

	var result TestNode
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.Equal(t, testNode.ID, result.ID)
	assert.Equal(t, testNode.Name, result.Name)
	assert.NotNil(t, result.RawData)
	assert.JSONEq(t, string(testNode.RawData), string(result.RawData))
}

func TestTransaction_Update_RawMessage(t *testing.T) {
	cleanupTestNodesForTxn(t)

	initialRawJSON := json.RawMessage([]byte(`{"initial":"data"}`))
	updatedRawJSON := json.RawMessage([]byte(`{"updated":"data","extra":"field"}`))

	testNode := &TestNode{
		ID:         uuid.New(),
		Name:       "txn-update-rawmessage-test",
		Tags:       []string{},
		Metadata:   map[string]interface{}{},
		Config:     TestConfig{Enabled: true, Level: 1, Mode: "test"},
		Attributes: map[string]float64{},
		Items:      []TestItem{},
		RawData:    initialRawJSON,
		CreatedAt:  time.Now().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().Truncate(time.Microsecond),
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()

	err := txn.Create(query, testNode)
	require.NoError(t, err)

	err = query.Commit()
	require.NoError(t, err)

	testNode.RawData = updatedRawJSON
	testNode.Config.Level = 2

	query2 := beginTxn(t)
	txn2 := NewTransaction[TestNode]()

	err = txn2.Update(query2, testNode)
	require.NoError(t, err)

	err = query2.Commit()
	require.NoError(t, err)

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query("SELECT * FROM test_nodes WHERE id = $1", testNode.ID)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())

	var result TestNode
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.Equal(t, 2, result.Config.Level)
	assert.NotNil(t, result.RawData)
	assert.JSONEq(t, string(updatedRawJSON), string(result.RawData))
}

func TestTransaction_FindByQuery(t *testing.T) {
	cleanupTestNodesForTxn(t)

	count1 := 100
	count2 := 200
	nodes := []*TestNode{
		{
			ID:        uuid.New(),
			Name:      "findby-query-1",
			Tags:      []string{"query", "test"},
			Config:    TestConfig{Enabled: true, Level: 10, Mode: "findquery"},
			Settings:  &TestSettings{Theme: "red", Size: 12},
			Count:     &count1,
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "findby-query-2",
			Tags:      []string{"query", "test2"},
			Config:    TestConfig{Enabled: false, Level: 20, Mode: "findquery"},
			Settings:  &TestSettings{Theme: "blue", Size: 14},
			Count:     &count2,
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
	}

	query := beginTxn(t)
	txn := NewTransaction[TestNode]()
	err := txn.CreateMultiple(query, nodes)
	require.NoError(t, err)
	err = query.Commit()
	require.NoError(t, err)

	query2 := beginTxn(t)
	results, err := txn.FindByQuery(query2, "SELECT * FROM test_nodes WHERE name LIKE $1 ORDER BY name", "findby-query%")
	require.NoError(t, err)
	query2.Commit()

	require.Len(t, results, 2)
	assert.Equal(t, "findby-query-1", results[0].Name)
	assert.Equal(t, "findby-query-2", results[1].Name)
	assert.NotNil(t, results[0].Count)
	assert.Equal(t, 100, *results[0].Count)
	assert.NotNil(t, results[1].Count)
	assert.Equal(t, 200, *results[1].Count)
	assert.Equal(t, "red", results[0].Settings.Theme)
	assert.Equal(t, "blue", results[1].Settings.Theme)
}

func TestDB_FindByQuery(t *testing.T) {
	cleanupTestNodesForTxn(t)

	count1 := 555
	count2 := 777
	nodes := []*TestNode{
		{
			ID:        uuid.New(),
			Name:      "db-findquery-1",
			Tags:      []string{"db", "query"},
			Config:    TestConfig{Enabled: true, Level: 15, Mode: "dbfind"},
			Settings:  &TestSettings{Theme: "purple", Size: 18},
			Count:     &count1,
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
		{
			ID:        uuid.New(),
			Name:      "db-findquery-2",
			Tags:      []string{"db", "query2"},
			Config:    TestConfig{Enabled: false, Level: 25, Mode: "dbfind"},
			Settings:  &TestSettings{Theme: "orange", Size: 20},
			Count:     &count2,
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		},
	}

	db := NewDB[TestNode](scannerTestDB)
	err := db.CreateMultiple(context.Background(), nodes)
	require.NoError(t, err)

	results, err := db.FindByQuery(context.Background(), "SELECT * FROM test_nodes WHERE name LIKE $1 ORDER BY name", "db-findquery%")
	require.NoError(t, err)

	require.Len(t, results, 2)
	assert.Equal(t, "db-findquery-1", results[0].Name)
	assert.Equal(t, "db-findquery-2", results[1].Name)
	assert.NotNil(t, results[0].Count)
	assert.Equal(t, 555, *results[0].Count)
	assert.NotNil(t, results[1].Count)
	assert.Equal(t, 777, *results[1].Count)
	assert.Equal(t, "purple", results[0].Settings.Theme)
	assert.Equal(t, "orange", results[1].Settings.Theme)
}
