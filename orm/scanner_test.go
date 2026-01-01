package orm

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/yadunandan004/scaffold/store/postgres"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

var (
	scannerTestContainer testcontainers.Container
	scannerTestDB        *sql.DB
)

type TestConfig struct {
	Enabled bool   `json:"enabled"`
	Level   int    `json:"level"`
	Mode    string `json:"mode"`
}

type TestItem struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type TestSettings struct {
	Theme string `json:"theme"`
	Size  int    `json:"size"`
}

type TestNode struct {
	ID         uuid.UUID              `json:"id" orm:"column:id;pk"`
	Name       string                 `json:"name" orm:"column:name"`
	Tags       []string               `json:"tags" orm:"column:tags"`
	Metadata   map[string]interface{} `json:"metadata" orm:"column:metadata"`
	Config     TestConfig             `json:"config" orm:"column:config"`
	Attributes map[string]float64     `json:"attributes" orm:"column:attributes"`
	Items      []TestItem             `json:"items" orm:"column:items"`
	Settings   *TestSettings          `json:"settings" orm:"column:settings"`
	Count      *int                   `json:"count" orm:"column:count"`
	RawData    json.RawMessage        `json:"raw_data" orm:"column:raw_data"`
	CreatedAt  time.Time              `json:"created_at" orm:"column:created_at"`
	UpdatedAt  time.Time              `json:"updated_at" orm:"column:updated_at"`
}

func (TestNode) TableName() string {
	return "test_nodes"
}

func TestMain(m *testing.M) {
	container, err := postgres.NewMockConnection()
	if err != nil {
		panic("Failed to start test container for scanner tests: " + err.Error())
	}
	scannerTestContainer = container
	scannerTestDB = postgres.GetDB().DB

	if err := createTestNodesTable(); err != nil {
		panic("Failed to create test_nodes table: " + err.Error())
	}

	RegisterModel[TestNode]()

	m.Run()
	if scannerTestContainer != nil {
		scannerTestContainer.Terminate(context.Background())
	}
}

func createTestNodesTable() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS test_nodes (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL UNIQUE,
		tags JSONB DEFAULT '[]',
		metadata JSONB DEFAULT '{}',
		config JSONB NOT NULL,
		attributes JSONB DEFAULT '{}',
		items JSONB DEFAULT '[]',
		settings JSONB,
		count INTEGER,
		raw_data JSONB,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);
	`

	_, err := scannerTestDB.Exec(createTableSQL)
	return err
}

func insertTestNode(t *testing.T, node *TestNode) {
	t.Helper()

	tagsJSON, _ := json.Marshal(node.Tags)
	metadataJSON, _ := json.Marshal(node.Metadata)
	configJSON, _ := json.Marshal(node.Config)
	attributesJSON, _ := json.Marshal(node.Attributes)
	itemsJSON, _ := json.Marshal(node.Items)

	var settingsJSON interface{}
	if node.Settings != nil {
		bytes, _ := json.Marshal(node.Settings)
		settingsJSON = bytes
	}

	var rawDataJSON interface{}
	if node.RawData != nil {
		rawDataJSON = []byte(node.RawData)
	}

	query := `
		INSERT INTO test_nodes
		(id, name, tags, metadata, config, attributes, items, settings, count, raw_data, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := scannerTestDB.Exec(query,
		node.ID, node.Name, tagsJSON, metadataJSON, configJSON,
		attributesJSON, itemsJSON, settingsJSON, node.Count,
		rawDataJSON, node.CreatedAt, node.UpdatedAt,
	)
	require.NoError(t, err)
}

func cleanupTestNodes(t *testing.T) {
	t.Helper()
	_, _ = scannerTestDB.Exec("DELETE FROM test_nodes")
}

func TestScanner_ScanRow_AllCompoundTypes(t *testing.T) {
	cleanupTestNodes(t)

	testNode := &TestNode{
		ID:   uuid.New(),
		Name: "test-node-all-types",
		Tags: []string{"tag1", "tag2", "tag3"},
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": float64(123),
			"nested": map[string]interface{}{
				"inner": "data",
			},
		},
		Config: TestConfig{
			Enabled: true,
			Level:   5,
			Mode:    "production",
		},
		Attributes: map[string]float64{
			"x": 1.5,
			"y": 2.5,
			"z": 3.5,
		},
		Items: []TestItem{
			{Name: "item1", Value: 10},
			{Name: "item2", Value: 20},
		},
		Settings: &TestSettings{
			Theme: "dark",
			Size:  14,
		},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	insertTestNode(t, testNode)

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query(
		"SELECT * FROM test_nodes WHERE id = $1",
		testNode.ID,
	)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())

	var result TestNode
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.Equal(t, testNode.ID, result.ID)
	assert.Equal(t, testNode.Name, result.Name)
	assert.Equal(t, testNode.Tags, result.Tags)
	assert.Equal(t, testNode.Config, result.Config)
	assert.Equal(t, testNode.Attributes, result.Attributes)
	assert.Equal(t, testNode.Items, result.Items)
	assert.NotNil(t, result.Settings)
	assert.Equal(t, testNode.Settings.Theme, result.Settings.Theme)
	assert.Equal(t, testNode.Settings.Size, result.Settings.Size)

	metadataJSON, _ := json.Marshal(testNode.Metadata)
	resultJSON, _ := json.Marshal(result.Metadata)
	assert.JSONEq(t, string(metadataJSON), string(resultJSON))
}

func TestScanner_ScanRow_EmptyCompoundTypes(t *testing.T) {
	cleanupTestNodes(t)

	testNode := &TestNode{
		ID:         uuid.New(),
		Name:       "test-node-empty",
		Tags:       []string{},
		Metadata:   map[string]interface{}{},
		Config:     TestConfig{Enabled: false, Level: 0, Mode: ""},
		Attributes: map[string]float64{},
		Items:      []TestItem{},
		Settings:   nil,
		Count:      nil,
		CreatedAt:  time.Now().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().Truncate(time.Microsecond),
	}

	insertTestNode(t, testNode)

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query(
		"SELECT * FROM test_nodes WHERE id = $1",
		testNode.ID,
	)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())

	var result TestNode
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Tags)
	assert.Len(t, result.Tags, 0)
	assert.NotNil(t, result.Metadata)
	assert.Len(t, result.Metadata, 0)
	assert.NotNil(t, result.Attributes)
	assert.Len(t, result.Attributes, 0)
	assert.NotNil(t, result.Items)
	assert.Len(t, result.Items, 0)
	assert.Nil(t, result.Settings)
	assert.Nil(t, result.Count)
}

func TestScanner_ScanRow_NullCompoundTypes(t *testing.T) {
	cleanupTestNodes(t)

	query := `
		INSERT INTO test_nodes (id, name, tags, metadata, config, attributes, items, settings, count, created_at, updated_at)
		VALUES ($1, $2, '[]', '{}', '{"enabled":false,"level":0,"mode":""}', '{}', '[]', NULL, NULL, NOW(), NOW())
	`

	testID := uuid.New()
	_, err := scannerTestDB.Exec(query, testID, "test-node-nulls")
	require.NoError(t, err)

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query(
		"SELECT * FROM test_nodes WHERE id = $1",
		testID,
	)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())

	var result TestNode
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.Nil(t, result.Settings)
	assert.Nil(t, result.Count)
}

func TestScanner_ScanRow_SliceOfStructs(t *testing.T) {
	cleanupTestNodes(t)

	testNode := &TestNode{
		ID:         uuid.New(),
		Name:       "test-node-slice-structs",
		Tags:       []string{},
		Metadata:   map[string]interface{}{},
		Config:     TestConfig{Enabled: true, Level: 1, Mode: "test"},
		Attributes: map[string]float64{},
		Items: []TestItem{
			{Name: "first", Value: 100},
			{Name: "second", Value: 200},
			{Name: "third", Value: 300},
		},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	insertTestNode(t, testNode)

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query(
		"SELECT * FROM test_nodes WHERE id = $1",
		testNode.ID,
	)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())

	var result TestNode
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	require.Len(t, result.Items, 3)
	assert.Equal(t, "first", result.Items[0].Name)
	assert.Equal(t, 100, result.Items[0].Value)
	assert.Equal(t, "second", result.Items[1].Name)
	assert.Equal(t, 200, result.Items[1].Value)
	assert.Equal(t, "third", result.Items[2].Name)
	assert.Equal(t, 300, result.Items[2].Value)
}

func TestScanner_ScanRows_MultipleRows(t *testing.T) {
	cleanupTestNodes(t)

	nodes := []*TestNode{
		{
			ID:         uuid.New(),
			Name:       "node-1",
			Tags:       []string{"a", "b"},
			Metadata:   map[string]interface{}{"key": "val1"},
			Config:     TestConfig{Enabled: true, Level: 1, Mode: "m1"},
			Attributes: map[string]float64{"x": 1.0},
			Items:      []TestItem{{Name: "i1", Value: 10}},
			CreatedAt:  time.Now().Truncate(time.Microsecond),
			UpdatedAt:  time.Now().Truncate(time.Microsecond),
		},
		{
			ID:         uuid.New(),
			Name:       "node-2",
			Tags:       []string{"c", "d"},
			Metadata:   map[string]interface{}{"key": "val2"},
			Config:     TestConfig{Enabled: false, Level: 2, Mode: "m2"},
			Attributes: map[string]float64{"y": 2.0},
			Items:      []TestItem{{Name: "i2", Value: 20}},
			CreatedAt:  time.Now().Truncate(time.Microsecond),
			UpdatedAt:  time.Now().Truncate(time.Microsecond),
		},
		{
			ID:         uuid.New(),
			Name:       "node-3",
			Tags:       []string{"e", "f"},
			Metadata:   map[string]interface{}{"key": "val3"},
			Config:     TestConfig{Enabled: true, Level: 3, Mode: "m3"},
			Attributes: map[string]float64{"z": 3.0},
			Items:      []TestItem{{Name: "i3", Value: 30}},
			CreatedAt:  time.Now().Truncate(time.Microsecond),
			UpdatedAt:  time.Now().Truncate(time.Microsecond),
		},
	}

	for _, node := range nodes {
		insertTestNode(t, node)
	}

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query("SELECT * FROM test_nodes ORDER BY name")
	require.NoError(t, err)
	defer rows.Close()

	var results []TestNode
	err = scanner.ScanRows(rows, &results)
	require.NoError(t, err)

	require.Len(t, results, 3)

	for i, result := range results {
		assert.Equal(t, nodes[i].Name, result.Name)
		assert.Equal(t, nodes[i].Tags, result.Tags)
		assert.Equal(t, nodes[i].Config, result.Config)
		assert.Equal(t, nodes[i].Attributes, result.Attributes)
		assert.Equal(t, nodes[i].Items, result.Items)
	}
}

func TestScanner_RoundTrip_ComplexNesting(t *testing.T) {
	cleanupTestNodes(t)

	testNode := &TestNode{
		ID:   uuid.New(),
		Name: "test-node-complex",
		Tags: []string{"complex", "nested", "test"},
		Metadata: map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": []interface{}{
						"value1",
						float64(42),
						map[string]interface{}{
							"deep": "data",
						},
					},
				},
			},
			"array": []interface{}{
				map[string]interface{}{"a": float64(1)},
				map[string]interface{}{"b": float64(2)},
			},
		},
		Config: TestConfig{
			Enabled: true,
			Level:   99,
			Mode:    "ultra",
		},
		Attributes: map[string]float64{
			"pi":  3.14159,
			"e":   2.71828,
			"phi": 1.61803,
		},
		Items: []TestItem{
			{Name: "unicode-🚀", Value: 1},
			{Name: "special-chars-!@#$", Value: 2},
		},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	insertTestNode(t, testNode)

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query(
		"SELECT * FROM test_nodes WHERE id = $1",
		testNode.ID,
	)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())

	var result TestNode
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	originalJSON, _ := json.Marshal(testNode.Metadata)
	resultJSON, _ := json.Marshal(result.Metadata)
	assert.JSONEq(t, string(originalJSON), string(resultJSON))

	assert.Equal(t, testNode.Items, result.Items)
	assert.Equal(t, testNode.Attributes, result.Attributes)
}

func TestRawScanner_ScanStruct(t *testing.T) {
	cleanupTestNodes(t)

	testNode := &TestNode{
		ID:   uuid.New(),
		Name: "test-raw-scanner",
		Tags: []string{"raw", "test"},
		Metadata: map[string]interface{}{
			"scanner": "raw",
		},
		Config: TestConfig{
			Enabled: true,
			Level:   7,
			Mode:    "raw-mode",
		},
		Attributes: map[string]float64{"test": 1.23},
		Items:      []TestItem{{Name: "raw-item", Value: 777}},
		CreatedAt:  time.Now().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().Truncate(time.Microsecond),
	}

	insertTestNode(t, testNode)

	rawScanner := &RawScanner{}

	rows, err := scannerTestDB.Query(
		"SELECT * FROM test_nodes WHERE id = $1",
		testNode.ID,
	)
	require.NoError(t, err)
	defer rows.Close()

	var result TestNode
	err = rawScanner.ScanRaw(rows, &result)
	require.NoError(t, err)

	assert.Equal(t, testNode.ID, result.ID)
	assert.Equal(t, testNode.Name, result.Name)
	assert.Equal(t, testNode.Tags, result.Tags)
	assert.Equal(t, testNode.Config, result.Config)
	assert.Equal(t, testNode.Attributes, result.Attributes)
	assert.Equal(t, testNode.Items, result.Items)
}

func TestScanner_ScanRow_RawMessage(t *testing.T) {
	cleanupTestNodes(t)

	rawJSON := json.RawMessage([]byte(`{"custom":"data","count":42,"nested":{"field":"value"}}`))

	testNode := &TestNode{
		ID:         uuid.New(),
		Name:       "test-node-rawmessage",
		Tags:       []string{"raw"},
		Metadata:   map[string]interface{}{},
		Config:     TestConfig{Enabled: true, Level: 1, Mode: "test"},
		Attributes: map[string]float64{},
		Items:      []TestItem{},
		RawData:    rawJSON,
		CreatedAt:  time.Now().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().Truncate(time.Microsecond),
	}

	insertTestNode(t, testNode)

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query(
		"SELECT * FROM test_nodes WHERE id = $1",
		testNode.ID,
	)
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

func TestScanner_ScanRow_NullRawMessage(t *testing.T) {
	cleanupTestNodes(t)

	testNode := &TestNode{
		ID:         uuid.New(),
		Name:       "test-node-null-rawmessage",
		Tags:       []string{},
		Metadata:   map[string]interface{}{},
		Config:     TestConfig{Enabled: false, Level: 0, Mode: ""},
		Attributes: map[string]float64{},
		Items:      []TestItem{},
		RawData:    nil,
		CreatedAt:  time.Now().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().Truncate(time.Microsecond),
	}

	insertTestNode(t, testNode)

	metadata := GetMetadata[TestNode]()
	scanner := NewScanner(metadata)

	rows, err := scannerTestDB.Query(
		"SELECT * FROM test_nodes WHERE id = $1",
		testNode.ID,
	)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())

	var result TestNode
	err = scanner.ScanRow(rows, &result)
	require.NoError(t, err)

	assert.Equal(t, testNode.ID, result.ID)
	assert.Nil(t, result.RawData)
}
