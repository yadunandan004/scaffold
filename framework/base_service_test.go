package framework

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/yadunandan004/scaffold/orm"
	"github.com/yadunandan004/scaffold/request"
	"github.com/yadunandan004/scaffold/store/postgres"
)

var (
	testContainer testcontainers.Container
	testDB        *postgres.DB
)

func TestMain(m *testing.M) {
	container, err := postgres.NewMockConnection()
	if err != nil {
		panic("Failed to start test container: " + err.Error())
	}
	testContainer = container
	testDB = postgres.GetDB()

	// Create test tables
	if err := createTestTables(); err != nil {
		panic("Failed to create test tables: " + err.Error())
	}

	// Register the test model
	orm.RegisterModel[TestSample]()

	m.Run()
	if testContainer != nil {
		testContainer.Terminate(context.Background())
	}
}

func createTestTables() error {
	// Create test_samples table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS test_samples (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMP,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		status VARCHAR(50) DEFAULT 'active',
		count INTEGER DEFAULT 0,
		amount DECIMAL(10, 2),
		is_active BOOLEAN DEFAULT true,
		metadata JSONB DEFAULT '{}'
	);

	CREATE TABLE IF NOT EXISTS test_samples_tracker (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		name VARCHAR(255),
		entity JSONB
	);

	-- Create indexes
	CREATE INDEX IF NOT EXISTS idx_test_samples_deleted_at ON test_samples(deleted_at);
	CREATE INDEX IF NOT EXISTS idx_test_samples_name ON test_samples(name);
	CREATE INDEX IF NOT EXISTS idx_test_samples_status ON test_samples(status);
	`

	_, err := testDB.DB.Exec(createTableSQL)
	return err
}

func TestBaseService_Create(t *testing.T) {
	// Setup
	repo := NewTestSampleRepository()
	service := NewBaseService[TestSample](repo)
	ctx := request.NewTestContext()

	// Start transaction for test
	_, err := request.BeginTransactionForModel[TestSample](ctx)
	require.NoError(t, err)
	defer ctx.CloseTxn(err)

	// Test data
	description := "Test Create"
	amount := 50.0
	sample := &TestSample{
		BaseModelImpl: BaseModelImpl{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:        "Test Create Sample",
		Description: &description,
		Status:      "active",
		Count:       10,
		Amount:      &amount,
		IsActive:    true,
		Metadata: JSONB{
			"test": "create",
		},
	}

	// ExecuteTemplate
	created, err := service.Create(ctx, sample)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "Test Create Sample", created.Name)
	assert.Equal(t, 10, created.Count)
	assert.NotZero(t, created.CreatedAt)
	assert.NotZero(t, created.UpdatedAt)
}

func TestBaseService_GetByID(t *testing.T) {
	// Setup
	repo := NewTestSampleRepository()
	service := NewBaseService[TestSample](repo)
	ctx := request.NewTestContext()

	// Start transaction for test
	_, err := request.BeginTransactionForModel[TestSample](ctx)
	require.NoError(t, err)
	defer ctx.CloseTxn(err)

	// Create test data
	sample, err := CreateSampleTable(ctx)
	require.NoError(t, err)

	// ExecuteTemplate
	retrieved, err := service.GetByID(ctx, sample.ID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, sample.ID, retrieved.ID)
	assert.Equal(t, sample.Name, retrieved.Name)
	assert.Equal(t, sample.Count, retrieved.Count)
}

func TestBaseService_Update(t *testing.T) {
	// Setup
	repo := NewTestSampleRepository()
	service := NewBaseService[TestSample](repo)
	ctx := request.NewTestContext()

	// Start transaction for test
	_, err := request.BeginTransactionForModel[TestSample](ctx)
	require.NoError(t, err)
	defer ctx.CloseTxn(err)

	// Create test data
	sample, err := CreateSampleTable(ctx)
	require.NoError(t, err)

	// Update data
	newDescription := "Updated description"
	sample.Description = &newDescription
	sample.Count = 20
	sample.Status = "inactive"

	// ExecuteTemplate
	updated, err := service.Update(ctx, sample.ID, sample)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, "Updated description", *updated.Description)
	assert.Equal(t, 20, updated.Count)
	assert.Equal(t, "inactive", updated.Status)
}

func TestBaseService_Delete(t *testing.T) {
	// Setup
	repo := NewTestSampleRepository()
	service := NewBaseService[TestSample](repo)
	ctx := request.NewTestContext()

	// Start transaction for test
	_, err := request.BeginTransactionForModel[TestSample](ctx)
	require.NoError(t, err)
	defer ctx.CloseTxn(err)

	// Create test data
	sample, err := CreateSampleTable(ctx)
	require.NoError(t, err)

	// ExecuteTemplate
	err = service.Delete(ctx, sample.ID)
	require.NoError(t, err)

	// Verify deletion
	retrieved, err := service.GetByID(ctx, sample.ID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestBaseService_CreateMultiple(t *testing.T) {
	// Setup
	repo := NewTestSampleRepository()
	service := NewBaseService[TestSample](repo)
	ctx := request.NewTestContext()

	// Start transaction for test
	_, err := request.BeginTransactionForModel[TestSample](ctx)
	require.NoError(t, err)
	defer ctx.CloseTxn(err)

	// Test data
	desc1 := "Sample 1"
	desc2 := "Sample 2"
	amount1 := 100.0
	amount2 := 200.0

	samples := []*TestSample{
		{
			BaseModelImpl: BaseModelImpl{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Name:        "Multiple Sample 1",
			Description: &desc1,
			Status:      "active",
			Count:       5,
			Amount:      &amount1,
			IsActive:    true,
			Metadata:    JSONB{},
		},
		{
			BaseModelImpl: BaseModelImpl{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Name:        "Multiple Sample 2",
			Description: &desc2,
			Status:      "pending",
			Count:       10,
			Amount:      &amount2,
			IsActive:    false,
			Metadata:    JSONB{},
		},
	}

	// ExecuteTemplate
	created, err := service.CreateMultiple(ctx, samples)

	// Assert
	require.NoError(t, err)
	assert.Len(t, created, 2)
	assert.NotEqual(t, uuid.Nil, created[0].ID)
	assert.NotEqual(t, uuid.Nil, created[1].ID)
	assert.Equal(t, "Multiple Sample 1", created[0].Name)
	assert.Equal(t, "Multiple Sample 2", created[1].Name)
}

func TestBaseService_UpdateMultiple(t *testing.T) {
	// Setup
	repo := NewTestSampleRepository()
	service := NewBaseService[TestSample](repo)
	ctx := request.NewTestContext()

	// Start transaction for test
	_, err := request.BeginTransactionForModel[TestSample](ctx)
	require.NoError(t, err)
	defer ctx.CloseTxn(err)

	// Create test data
	sample1, err := CreateSampleTable(ctx)
	require.NoError(t, err)
	sample2, err := CreateSampleTable(ctx)
	require.NoError(t, err)

	// Update data
	sample1.Status = "updated"
	sample1.Count = 100
	sample2.Status = "updated"
	sample2.Count = 200

	// ExecuteTemplate
	updated, err := service.UpdateMultiple(ctx, []*TestSample{sample1, sample2})

	// Assert
	require.NoError(t, err)
	assert.Len(t, updated, 2)
	assert.Equal(t, "updated", updated[0].Status)
	assert.Equal(t, 100, updated[0].Count)
	assert.Equal(t, "updated", updated[1].Status)
	assert.Equal(t, 200, updated[1].Count)
}

func TestBaseService_DeleteMultiple(t *testing.T) {
	// Setup
	repo := NewTestSampleRepository()
	service := NewBaseService[TestSample](repo)
	ctx := request.NewTestContext()

	// Start transaction for test
	_, err := request.BeginTransactionForModel[TestSample](ctx)
	require.NoError(t, err)
	defer ctx.CloseTxn(err)

	// Create test data
	sample1, err := CreateSampleTable(ctx)
	require.NoError(t, err)
	sample2, err := CreateSampleTable(ctx)
	require.NoError(t, err)

	ids := []uuid.UUID{sample1.ID, sample2.ID}

	// ExecuteTemplate
	err = service.DeleteMultiple(ctx, ids)
	require.NoError(t, err)

	// Verify deletion
	retrieved1, err1 := service.GetByID(ctx, sample1.ID)
	retrieved2, err2 := service.GetByID(ctx, sample2.ID)

	assert.Error(t, err1)
	assert.Error(t, err2)
	assert.Nil(t, retrieved1)
	assert.Nil(t, retrieved2)
}

func TestBaseService_WithCache(t *testing.T) {
	// This test would require setting up a cache service
	// For now, we'll just test that the service works without cache
	repo := NewTestSampleRepository()
	service := NewBaseService[TestSample](repo)
	ctx := request.NewTestContext()

	// Start transaction for test
	_, err := request.BeginTransactionForModel[TestSample](ctx)
	require.NoError(t, err)
	defer ctx.CloseTxn(err)

	// Create test data
	sample, err := CreateSampleTable(ctx)
	require.NoError(t, err)

	// ExecuteTemplate multiple gets - without cache, each will hit the database
	retrieved1, err := service.GetByID(ctx, sample.ID)
	require.NoError(t, err)

	retrieved2, err := service.GetByID(ctx, sample.ID)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, retrieved1.ID, retrieved2.ID)
	assert.Equal(t, retrieved1.Name, retrieved2.Name)
}
