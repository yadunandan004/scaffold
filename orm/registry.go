package orm

import (
	"database/sql"
	"fmt"
	"reflect"
	"sync"
)

type ModelMetadata struct {
	Type          reflect.Type
	TableName     string
	Schema        string
	Fields        []FieldMetadata
	FieldMap      map[string]int
	PKFields      []string
	UniqueFields  [][]string
	SQLTemplates  SQLTemplates
	ExtractValues func(entity interface{}) []interface{}
	ScanRow       func(row *sql.Row, dest interface{}) error
	ScanRows      func(rows *sql.Rows, dest interface{}) error
	ExtractID     func(entity interface{}) interface{}
	IDColumn      string
	IDType        reflect.Type
	SetID         func(entity interface{}, id interface{})
}

type FieldMetadata struct {
	Name            string
	Column          string
	Type            reflect.Type
	SQLType         string
	Offset          uintptr
	IsPK            bool
	IsUnique        bool
	IsNullable      bool
	IsAutoIncrement bool
	Default         string
	Index           int
}

type SQLTemplates struct {
	Insert      string
	Update      string
	Delete      string
	SelectByPK  string
	SelectAll   string
	TableName   string
	BatchInsert func(count int) string
}

var metadataByType = sync.Map{}

type Registry struct {
	mu        sync.RWMutex
	models    map[reflect.Type]*ModelMetadata
	db        *sql.DB
	validated map[reflect.Type]bool
}

var (
	registry *Registry
	once     sync.Once
)

func GetRegistry() *Registry {
	once.Do(func() {
		registry = &Registry{
			models:    make(map[reflect.Type]*ModelMetadata),
			validated: make(map[reflect.Type]bool),
		}
	})
	return registry
}

func (r *Registry) SetDB(db *sql.DB) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.db = db
}

func (r *Registry) GetMetadata(modelType reflect.Type) (*ModelMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	metadata, ok := r.models[modelType]
	return metadata, ok
}

func (r *Registry) IsRegistered(t reflect.Type) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.models[t]
	return exists
}

func (r *Registry) GetAllMetadata() map[reflect.Type]*ModelMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[reflect.Type]*ModelMetadata)
	for k, v := range r.models {
		result[k] = v
	}
	return result
}

func RegisterModel[T any]() {
	var model T
	typeKey := fmt.Sprintf("%T", model)
	typ := reflect.TypeOf(model)

	schema, tableName := discoverTableName(typ, model)
	fields, fieldOffsets, fieldTypes, columnNames, pkIndex := parseFields(typ)
	insertColumns, insertIndices := filterInsertFields(fields, columnNames)

	idColumn := "id"
	var idType reflect.Type
	if pkIndex >= 0 && pkIndex < len(columnNames) {
		idColumn = columnNames[pkIndex]
		idType = fieldTypes[pkIndex]
	}

	sqlTemplates := buildSQLTemplates(schema, tableName, insertColumns, columnNames, idColumn)

	extractValues := makeExtractValues[T](fieldOffsets, fieldTypes, insertIndices)
	extractID := makeExtractID[T](fieldOffsets, fieldTypes, pkIndex)
	setID := makeSetID[T](fieldOffsets, fieldTypes, pkIndex)
	scanRow := makeScanRow[T](fieldOffsets, fieldTypes)
	scanRows := makeScanRows[T](fieldOffsets, fieldTypes, columnNames)

	metadata := &ModelMetadata{
		Type:          typ,
		TableName:     tableName,
		Schema:        schema,
		Fields:        fields,
		FieldMap:      make(map[string]int),
		PKFields:      []string{},
		SQLTemplates:  sqlTemplates,
		ExtractValues: extractValues,
		ExtractID:     extractID,
		SetID:         setID,
		IDColumn:      idColumn,
		IDType:        idType,
		ScanRow:       scanRow,
		ScanRows:      scanRows,
	}

	for i, field := range fields {
		metadata.FieldMap[field.Column] = i
		if field.IsPK {
			metadata.PKFields = append(metadata.PKFields, field.Column)
		}
	}

	metadataByType.Store(typeKey, metadata)

	r := GetRegistry()
	r.mu.Lock()
	r.models[typ] = metadata
	r.validated[typ] = true
	r.mu.Unlock()
}

func GetMetadata[T any]() *ModelMetadata {
	var model T
	typeKey := fmt.Sprintf("%T", model)
	val, _ := metadataByType.Load(typeKey)
	if val == nil {
		return nil
	}
	return val.(*ModelMetadata)
}
