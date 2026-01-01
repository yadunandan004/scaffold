package framework

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yadunandan004/scaffold/store/postgres"
	"time"

	"github.com/google/uuid"

	injContext "github.com/yadunandan004/scaffold/request"
)

type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("unsupported type for JSONB")
	}

	return json.Unmarshal(bytes, j)
}

// TestSample is a test model that corresponds to test_samples table
type TestSample struct {
	BaseModelImpl
	Name        string   `json:"name" orm:"column:name;not null"`
	Description *string  `json:"description" orm:"column:description"`
	Status      string   `json:"status" orm:"column:status;default:active"`
	Count       int      `json:"count" orm:"column:count;default:0"`
	Amount      *float64 `json:"amount" orm:"column:amount"`
	IsActive    bool     `json:"is_active" orm:"column:is_active;default:true"`
	Metadata    JSONB    `json:"metadata" orm:"column:metadata;type:jsonb;default:'{}'"`
}

// TestSampleRepository for testing
type TestSampleRepository interface {
	BaseRepository[TestSample]
	GetByName(ctx injContext.Context, name string) (*TestSample, error)
}

type testSampleRepositoryImpl struct {
	*PostgresRepository[TestSample]
}

func NewTestSampleRepository() TestSampleRepository {
	return &testSampleRepositoryImpl{
		PostgresRepository: NewPostgresRepository[TestSample](),
	}
}

func (r *testSampleRepositoryImpl) GetByName(ctx injContext.Context, name string) (*TestSample, error) {
	var sample TestSample
	db := postgres.GetDB()
	if db == nil {
		return nil, fmt.Errorf("no database")
	}
	query := "SELECT * FROM test_samples WHERE name = $1 LIMIT 1"
	err := db.QueryRowContext(context.Background(), query, name).Scan(
		&sample.ID,
		&sample.CreatedAt,
		&sample.UpdatedAt,
		&sample.Name,
		&sample.Description,
		&sample.Status,
		&sample.Count,
		&sample.Amount,
		&sample.IsActive,
		&sample.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("record not found")
	}
	if err != nil {
		return nil, err
	}
	return &sample, nil
}

func (t TestSample) TableName() string {
	return "test_samples"
}

func (t TestSample) GetID() uuid.UUID {
	return t.ID
}

func (t TestSample) TrackerTableName() string {
	return "test_samples_tracker"
}

func (t TestSample) SaveTracker() bool {
	return false // Disable tracker for tests
}

func (t TestSample) SaveInCache() bool {
	return true
}

func (t TestSample) Validate(ctx injContext.Context) error {
	return nil
}

func (t TestSample) PreInsert(ctx injContext.Context) error {
	// Value receiver can't modify - timestamps will be set by test
	return nil
}

func (t TestSample) PostInsert(ctx injContext.Context) error {
	return nil
}

func (t TestSample) PreUpdate(ctx injContext.Context) error {
	return nil
}

func (t TestSample) PostUpdate(ctx injContext.Context) error {
	return nil
}

func (t TestSample) PreDelete(ctx injContext.Context) error {
	return nil
}

func (t TestSample) PostDelete(ctx injContext.Context) error {
	return nil
}

func (t TestSample) MapToTracker(ctx injContext.Context) *Tracker {
	return &Tracker{
		Name: t.Name,
		Entity: JSONB{
			"id":     t.ID,
			"action": "test_sample_update",
			"changes": map[string]interface{}{
				"status": t.Status,
				"count":  t.Count,
			},
		},
	}
}

// CreateSampleTable creates a sample entry for testing
func CreateSampleTable(ctx injContext.Context) (*TestSample, error) {
	description := "Test description"
	amount := 100.50
	sample := &TestSample{
		BaseModelImpl: BaseModelImpl{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:        "Test Sample " + uuid.New().String()[:8],
		Description: &description,
		Status:      "active",
		Count:       5,
		Amount:      &amount,
		IsActive:    true,
		Metadata: JSONB{
			"key1": "value1",
			"key2": 123,
		},
	}

	repo := NewTestSampleRepository()
	if err := repo.Create(ctx, sample); err != nil {
		return nil, err
	}

	return sample, nil
}

// IsPostgresEnabled returns true as TestSample is stored in PostgreSQL
func (t TestSample) IsPostgresEnabled() bool {
	return true
}

// IsClickhouseEnabled returns false as TestSample is not stored in ClickHouse
func (t TestSample) IsClickhouseEnabled() bool {
	return false
}

// OnConflict returns the fields to check for conflicts during upsert
func (t TestSample) OnConflict() []string {
	return []string{"name"}
}

// UpdateColumns returns nil for auto-generate behavior
func (t TestSample) UpdateColumns() []string {
	return nil
}

// Ensure TestSample implements BaseModel
var _ BaseModel = (*TestSample)(nil)
