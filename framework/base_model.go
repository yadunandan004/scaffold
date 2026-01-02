package framework

import (
	"github.com/yadunandan004/scaffold/request"
	"time"

	"github.com/google/uuid"
)

type IDType interface {
	~int | ~int64 | ~string | uuid.UUID
}

type BaseReadModel[ID IDType] interface {
	TableName() string
	IsPostgresEnabled() bool
	IsClickhouseEnabled() bool
	SaveInCache() bool
	GetID() ID
}

type BaseInsertModel[ID IDType] interface {
	BaseReadModel[ID]
	Validate(ctx request.Context) error
	PreInsert(ctx request.Context) error
	PostInsert(ctx request.Context) error
	SaveTracker() bool
	MapToTracker(ctx request.Context) *Tracker
	TrackerTableName() string
	OnConflict() []string
	UpdateColumns() []string
}

type BaseUpdateModel[ID IDType] interface {
	BaseInsertModel[ID]
	PreUpdate(ctx request.Context) error
	PostUpdate(ctx request.Context) error
}

type BaseDeleteModel[ID IDType] interface {
	BaseUpdateModel[ID]
	PreDelete(ctx request.Context) error
	PostDelete(ctx request.Context) error
}

type BaseCompleteModel[ID IDType] interface {
	BaseDeleteModel[ID]
}

type BaseModel[ID IDType] interface {
	BaseInsertModel[ID]
	PreUpdate(ctx request.Context) error
	PostUpdate(ctx request.Context) error
	PreDelete(ctx request.Context) error
	PostDelete(ctx request.Context) error
}

type BaseReadModelImpl[ID IDType] struct {
	ID ID `json:"id" orm:"column:id;pk"`
}

type BaseInsertModelImpl[ID IDType] struct {
	BaseReadModelImpl[ID]
	CreatedAt time.Time `json:"created_at" orm:"column:created_at;default:CURRENT_TIMESTAMP"`
	CreatedBy *string   `json:"created_by,omitempty" orm:"column:created_by;type:varchar(100);nullable"`
}

type BaseModelImpl[ID IDType] struct {
	ID        ID         `json:"id" orm:"column:id;pk"`
	CreatedAt time.Time  `json:"created_at" orm:"column:created_at;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `json:"updated_at" orm:"column:updated_at;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `json:"-" orm:"column:deleted_at;nullable"`
}

func (b *BaseModelImpl[ID]) GetID() ID {
	return b.ID
}

func (b *BaseModelImpl[ID]) SaveTracker() bool {
	return false
}

func (b *BaseModelImpl[ID]) SaveInCache() bool {
	return false
}

func (b *BaseModelImpl[ID]) Validate(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl[ID]) PreInsert(ctx request.Context) error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	if b.UpdatedAt.IsZero() {
		b.UpdatedAt = time.Now()
	}
	return nil
}

func (b *BaseModelImpl[ID]) PostInsert(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl[ID]) PreUpdate(ctx request.Context) error {
	b.UpdatedAt = time.Now()
	return nil
}

func (b *BaseModelImpl[ID]) PostUpdate(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl[ID]) PreDelete(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl[ID]) PostDelete(ctx request.Context) error {
	return nil
}

func (b *BaseModelImpl[ID]) MapToTracker(ctx request.Context) *Tracker {
	return nil
}

func (b *BaseModelImpl[ID]) TrackerTableName() string {
	return ""
}

func (b *BaseModelImpl[ID]) IsPostgresEnabled() bool {
	return true
}

func (b *BaseModelImpl[ID]) IsClickhouseEnabled() bool {
	return false
}

func (b *BaseModelImpl[ID]) OnConflict() []string {
	return []string{}
}

func (b *BaseModelImpl[ID]) UpdateColumns() []string {
	return nil
}

func (b *BaseReadModelImpl[ID]) TableName() string {
	return ""
}

func (b *BaseReadModelImpl[ID]) GetID() ID {
	return b.ID
}

func (b *BaseReadModelImpl[ID]) IsPostgresEnabled() bool {
	return true
}

func (b *BaseReadModelImpl[ID]) IsClickhouseEnabled() bool {
	return false
}

func (b *BaseReadModelImpl[ID]) SaveInCache() bool {
	return false
}

func (b *BaseInsertModelImpl[ID]) Validate(ctx request.Context) error {
	return nil
}

func (b *BaseInsertModelImpl[ID]) PreInsert(ctx request.Context) error {
	return nil
}

func (b *BaseInsertModelImpl[ID]) PostInsert(ctx request.Context) error {
	return nil
}

func (b *BaseInsertModelImpl[ID]) SaveTracker() bool {
	return false
}

func (b *BaseInsertModelImpl[ID]) MapToTracker(ctx request.Context) *Tracker {
	return nil
}

func (b *BaseInsertModelImpl[ID]) TrackerTableName() string {
	return ""
}

func (b *BaseInsertModelImpl[ID]) OnConflict() []string {
	return []string{}
}

func (b *BaseInsertModelImpl[ID]) UpdateColumns() []string {
	return nil
}

type Tracker struct {
	Name   string `json:"name"`
	Entity JSONB  `json:"entity" orm:"column:entity;type:jsonb"`
}
