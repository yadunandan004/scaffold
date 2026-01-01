package orm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// Scanner provides methods to scan SQL results into Go types
type Scanner struct {
	metadata *ModelMetadata
}

// NewScanner creates a Scanner for a specific model type
func NewScanner(metadata *ModelMetadata) *Scanner {
	return &Scanner{metadata: metadata}
}

// ScanRow scans a single row into a struct using metadata
func (s *Scanner) ScanRow(rows *sql.Rows, dest interface{}) error {
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be a pointer")
	}

	destValue = destValue.Elem()
	if !destValue.CanSet() {
		return fmt.Errorf("destination cannot be set")
	}

	// Get column names from result set
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// Create scan destinations using type handlers
	scanDests := make([]interface{}, len(columns))

	for i, col := range columns {
		// Find field for this column
		fieldIdx, exists := s.metadata.FieldMap[col]
		if !exists {
			// Column not in model, scan to discard
			var discard interface{}
			scanDests[i] = &discard
			continue
		}

		field := s.metadata.Fields[fieldIdx]
		fieldValue := destValue.Field(field.Index)

		// Use type handler to create the appropriate scan target
		fieldPtr := fieldValue.Addr().UnsafePointer()
		scanDests[i] = createScanTarget(fieldPtr, field.Type)
	}

	return rows.Scan(scanDests...)
}

// ScanRows scans multiple rows into a slice
func (s *Scanner) ScanRows(rows *sql.Rows, dest interface{}) error {
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be a pointer to slice")
	}

	destValue = destValue.Elem()
	if destValue.Kind() != reflect.Slice {
		return fmt.Errorf("destination must be a pointer to slice")
	}

	elemType := destValue.Type().Elem()
	isPtr := elemType.Kind() == reflect.Ptr
	if isPtr {
		elemType = elemType.Elem()
	}

	if destValue.IsNil() {
		destValue.Set(reflect.MakeSlice(destValue.Type(), 0, 0))
	}

	for rows.Next() {
		elem := reflect.New(elemType)
		if err := s.ScanRow(rows, elem.Interface()); err != nil {
			return err
		}

		if isPtr {
			destValue.Set(reflect.Append(destValue, elem))
		} else {
			destValue.Set(reflect.Append(destValue, elem.Elem()))
		}
	}

	return rows.Err()
}

// RawScanner provides flexible scanning for any type without metadata
type RawScanner struct{}

// ScanRaw scans a sql.Rows result into any destination type
func (r *RawScanner) ScanRaw(rows *sql.Rows, dest interface{}) error {
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be a pointer")
	}

	destValue = destValue.Elem()

	// Handle different destination types
	switch destValue.Kind() {
	case reflect.Struct:
		return r.scanStruct(rows, destValue)
	case reflect.Slice:
		return r.scanSlice(rows, destValue)
	case reflect.Map:
		return r.scanMap(rows, destValue)
	default:
		// Single value scan
		return rows.Scan(dest)
	}
}

// scanStructRow scans the current row into a struct without calling rows.Next()
func (r *RawScanner) scanStructRow(rows *sql.Rows, destValue reflect.Value) error {
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// Build scan destinations based on struct fields
	scanDests := make([]interface{}, len(columns))
	for i, col := range columns {
		// Find struct field by name (case insensitive)
		field := r.findFieldByName(destValue.Type(), col)
		if field == nil {
			var discard interface{}
			scanDests[i] = &discard
			continue
		}

		fieldValue := destValue.FieldByIndex(field.Index)
		if fieldValue.CanAddr() && fieldValue.CanSet() {
			fieldPtr := fieldValue.Addr().UnsafePointer()
			scanDests[i] = createScanTarget(fieldPtr, field.Type)
		} else {
			var discard interface{}
			scanDests[i] = &discard
		}
	}

	return rows.Scan(scanDests...)
}

// scanStruct scans a single row into a struct
func (r *RawScanner) scanStruct(rows *sql.Rows, destValue reflect.Value) error {
	if !rows.Next() {
		return sql.ErrNoRows
	}
	return r.scanStructRow(rows, destValue)
}

// scanSlice scans multiple rows into a slice
func (r *RawScanner) scanSlice(rows *sql.Rows, destValue reflect.Value) error {
	elemType := destValue.Type().Elem()
	isPtr := elemType.Kind() == reflect.Ptr
	if isPtr {
		elemType = elemType.Elem()
	}

	if destValue.IsNil() {
		destValue.Set(reflect.MakeSlice(destValue.Type(), 0, 0))
	}

	for rows.Next() {
		elem := reflect.New(elemType)

		if elemType.Kind() == reflect.Struct {
			if err := r.scanStructRow(rows, elem.Elem()); err != nil {
				return err
			}
		} else {
			// Single column result
			if err := rows.Scan(elem.Interface()); err != nil {
				return err
			}
		}

		if isPtr {
			destValue.Set(reflect.Append(destValue, elem))
		} else {
			destValue.Set(reflect.Append(destValue, elem.Elem()))
		}
	}

	return rows.Err()
}

// scanMap scans a single row into a map
func (r *RawScanner) scanMap(rows *sql.Rows, destValue reflect.Value) error {
	if !rows.Next() {
		return sql.ErrNoRows
	}

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// Initialize map if needed
	if destValue.IsNil() {
		destValue.Set(reflect.MakeMap(destValue.Type()))
	}

	// Scan into interface{} values
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return err
	}

	// Put values into map
	for i, col := range columns {
		key := reflect.ValueOf(col)
		val := reflect.ValueOf(values[i])
		destValue.SetMapIndex(key, val)
	}

	return nil
}

// findFieldByName finds a struct field by column name (case insensitive)
func (r *RawScanner) findFieldByName(t reflect.Type, name string) *reflect.StructField {
	name = strings.ToLower(name)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Check field name
		if strings.ToLower(field.Name) == name {
			return &field
		}

		// Check orm tag
		if tag := field.Tag.Get("orm"); tag != "" {
			parts := strings.Split(tag, ";")
			for _, part := range parts {
				if strings.HasPrefix(part, "column:") {
					col := strings.TrimPrefix(part, "column:")
					if strings.ToLower(col) == name {
						return &field
					}
				}
			}
		}

		// Check json tag as fallback
		if tag := field.Tag.Get("json"); tag != "" {
			jsonName := strings.Split(tag, ",")[0]
			if strings.ToLower(jsonName) == name {
				return &field
			}
		}
	}

	return nil
}
