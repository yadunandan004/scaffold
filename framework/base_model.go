package framework

import (
	"github.com/yadunandan004/scaffold/request"
	"time"

	"github.com/google/uuid"
)

// BaseReadModel - Minimal interface for read-only operations
type BaseReadModel interface {
	TableName() string
	IsPostgresEnabled() bool
	IsClickhouseEnabled() bool
	SaveInCache() bool
}

// BaseInsertModel - Interface for read + insert operations
type BaseInsertModel interface {
	BaseReadModel
	GetID() uuid.UUID
	Validate(ctx request.Context) error
	PreInsert(ctx request.Context) error
	PostInsert(ctx request.Context) error
	SaveTracker() bool
	MapToTracker(ctx request.Context) *Tracker
	TrackerTableName() string
	OnConflict() []string
	UpdateColumns() []string
}

// BaseUpdateModel - Interface for read + insert + update operations
type BaseUpdateModel interface {
	BaseInsertModel
	PreUpdate(ctx request.Context) error
	PostUpdate(ctx request.Context) error
}

// BaseDeleteModel - Interface for read + insert + update + delete operations
type BaseDeleteModel interface {
	BaseUpdateModel
	PreDelete(ctx request.Context) error
	PostDelete(ctx request.Context) error
}

// BaseCompleteModel - Full CRUD interface (alias for BaseDeleteModel)
type BaseCompleteModel interface {
	BaseDeleteModel
}

// BaseModel
//
//	@Description	Some Generic Body
type BaseModel interface {
	BaseInsertModel
	PreUpdate(ctx request.Context) error
	PostUpdate(ctx request.Context) error
	PreDelete(ctx request.Context) error
	PostDelete(ctx request.Context) error
}

// BaseReadModelImpl - Minimal struct with only ID
type BaseReadModelImpl struct {
	ID uuid.UUID `json:"id" orm:"column:id;type:uuid;pk"`
}

// BaseInsertModelImpl - Struct with ID and creation fields
type BaseInsertModelImpl struct {
	BaseReadModelImpl
	CreatedAt time.Time `json:"created_at" orm:"column:created_at;default:CURRENT_TIMESTAMP"`
	CreatedBy *string   `json:"created_by,omitempty" orm:"column:created_by;type:varchar(100);nullable"`
}

// BaseModelImpl provides a default implementation of BaseModel
type BaseModelImpl struct {
	ID        uuid.UUID  `json:"id" orm:"column:id;type:uuid;default:uuid_generate_v4();pk"`
	CreatedAt time.Time  `json:"created_at" orm:"column:created_at;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `json:"updated_at" orm:"column:updated_at;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `json:"-" orm:"column:deleted_at;nullable"`
}

// GetID returns the entity ID
func (b *BaseModelImpl) GetID() uuid.UUID {
	return b.ID
}

// SaveTracker returns false by default - override in specific models if needed
func (b *BaseModelImpl) SaveTracker() bool {
	return false
}

// SaveInCache returns false by default - override in specific models if needed
func (b *BaseModelImpl) SaveInCache() bool {
	return false
}

func (b *BaseModelImpl) Validate(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl) PreInsert(ctx request.Context) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	if b.UpdatedAt.IsZero() {
		b.UpdatedAt = time.Now()
	}
	return nil
}

func (b *BaseModelImpl) PostInsert(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl) PreUpdate(ctx request.Context) error {
	b.UpdatedAt = time.Now()
	return nil
}

func (b *BaseModelImpl) PostUpdate(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl) PreDelete(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl) PostDelete(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl) MapToTracker(ctx request.Context) *Tracker {
	return nil
}

func (b *BaseModelImpl) TrackerTableName() string {
	return ""
}

// IsPostgresEnabled returns true by default - override in specific models if needed
func (b *BaseModelImpl) IsPostgresEnabled() bool {
	return true
}

// IsClickhouseEnabled returns false by default - override in specific models if needed
func (b *BaseModelImpl) IsClickhouseEnabled() bool {
	return false
}

// OnConflict returns empty by default - override in specific models to specify conflict fields
func (b *BaseModelImpl) OnConflict() []string {
	return []string{}
}

// UpdateColumns returns nil by default - override to control upsert behavior
func (b *BaseModelImpl) UpdateColumns() []string {
	return nil
}

// BaseReadModelImpl methods
func (b *BaseReadModelImpl) TableName() string {
	return ""
}

func (b *BaseReadModelImpl) GetID() uuid.UUID {
	return b.ID
}

func (b *BaseReadModelImpl) IsPostgresEnabled() bool {
	return true
}

func (b *BaseReadModelImpl) IsClickhouseEnabled() bool {
	return false
}

func (b *BaseReadModelImpl) SaveInCache() bool {
	return false
}

// BaseInsertModelImpl methods
func (b *BaseInsertModelImpl) Validate(ctx request.Context) error {
	return nil
}

func (b *BaseInsertModelImpl) PreInsert(ctx request.Context) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

func (b *BaseInsertModelImpl) PostInsert(ctx request.Context) error {
	return nil
}

func (b *BaseInsertModelImpl) SaveTracker() bool {
	return false
}

func (b *BaseInsertModelImpl) MapToTracker(ctx request.Context) *Tracker {
	return nil
}

func (b *BaseInsertModelImpl) TrackerTableName() string {
	return ""
}

func (b *BaseInsertModelImpl) OnConflict() []string {
	return []string{}
}

func (b *BaseInsertModelImpl) UpdateColumns() []string {
	return nil
}

type Tracker struct {
	Name   string `json:"name"`
	Entity JSONB  `json:"entity" orm:"column:entity;type:jsonb"`
}
