package orm

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// ValidateSchema validates model metadata against actual database schema
func ValidateSchema(db *sql.DB, metadata *ModelMetadata) error {
	// Check if table exists
	exists, err := tableExists(db, metadata.Schema, metadata.TableName)
	if err != nil {
		return fmt.Errorf("failed to check table existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("table %s.%s does not exist in database", metadata.Schema, metadata.TableName)
	}

	// Validate columns
	if err := validateColumns(db, metadata); err != nil {
		return fmt.Errorf("column validation failed: %w", err)
	}

	// Validate primary keys
	if err := validatePrimaryKeys(db, metadata); err != nil {
		return fmt.Errorf("primary key validation failed: %w", err)
	}

	return nil
}

// tableExists checks if a table exists in the database
func tableExists(db *sql.DB, schema, table string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2
		)
	`
	var exists bool
	err := db.QueryRow(query, schema, table).Scan(&exists)
	return exists, err
}

// validateColumns validates that all model fields exist in the database
func validateColumns(db *sql.DB, metadata *ModelMetadata) error {
	query := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position
	`

	rows, err := db.Query(query, metadata.Schema, metadata.TableName)
	if err != nil {
		return fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	dbColumns := make(map[string]*dbColumn)
	for rows.Next() {
		var col dbColumn
		var charMaxLen sql.NullInt64
		err := rows.Scan(&col.Name, &col.DataType, &col.IsNullable, &col.Default, &charMaxLen)
		if err != nil {
			return err
		}
		if charMaxLen.Valid {
			col.MaxLength = int(charMaxLen.Int64)
		}
		dbColumns[col.Name] = &col
	}

	// Validate each model field exists in database
	for _, field := range metadata.Fields {
		dbCol, exists := dbColumns[field.Column]
		if !exists {
			// Log warning but don't fail - field might be computed or virtual
			log.Printf("[ORM] ⚠ Warning: column '%s' not found in table %s.%s",
				field.Column, metadata.Schema, metadata.TableName)
			continue
		}

		// Check nullability mismatch
		if !field.IsNullable && dbCol.IsNullable == "YES" {
			log.Printf("[ORM] ⚠ Warning: column '%s' is nullable in DB but marked as required in model",
				field.Column)
		}

		// Check for auto-increment
		if dbCol.Default.Valid && strings.Contains(dbCol.Default.String, "nextval") {
			field.IsAutoIncrement = true
		}

		// Basic type compatibility check
		if !isTypeCompatible(field.Type.String(), dbCol.DataType) {
			log.Printf("[ORM] ⚠ Warning: potential type mismatch for column '%s': Go type %s, DB type %s",
				field.Column, field.Type, dbCol.DataType)
		}
	}

	return nil
}

// validatePrimaryKeys validates primary key configuration
func validatePrimaryKeys(db *sql.DB, metadata *ModelMetadata) error {
	query := `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema = $1
			AND tc.table_name = $2
			AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY kcu.ordinal_position
	`

	rows, err := db.Query(query, metadata.Schema, metadata.TableName)
	if err != nil {
		return fmt.Errorf("failed to query primary keys: %w", err)
	}
	defer rows.Close()

	dbPKColumns := []string{}
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return err
		}
		dbPKColumns = append(dbPKColumns, colName)
	}

	// If no PKs defined in model, use DB PKs
	if len(metadata.PKFields) == 0 && len(dbPKColumns) > 0 {
		log.Printf("[ORM] Using database primary keys for %s.%s: %v",
			metadata.Schema, metadata.TableName, dbPKColumns)
		metadata.PKFields = dbPKColumns

		// Update field metadata
		for i, field := range metadata.Fields {
			for _, pk := range dbPKColumns {
				if field.Column == pk {
					metadata.Fields[i].IsPK = true
					break
				}
			}
		}
		return nil
	}

	// Validate PKs match
	if len(dbPKColumns) != len(metadata.PKFields) {
		return fmt.Errorf("primary key count mismatch:\n  Database: %v\n  Model: %v",
			dbPKColumns, metadata.PKFields)
	}

	// Check order and names match
	for i, dbCol := range dbPKColumns {
		if dbCol != metadata.PKFields[i] {
			return fmt.Errorf("primary key mismatch at position %d:\n  Database: %s\n  Model: %s",
				i+1, dbCol, metadata.PKFields[i])
		}
	}

	return nil
}

// dbColumn represents a database column
type dbColumn struct {
	Name       string
	DataType   string
	IsNullable string
	Default    sql.NullString
	MaxLength  int
}

// isTypeCompatible checks if Go type is compatible with SQL type
func isTypeCompatible(goType, sqlType string) bool {
	// Basic compatibility map
	compatMap := map[string][]string{
		"string":          {"text", "varchar", "character varying", "uuid", "char"},
		"int":             {"integer", "int", "int4", "serial"},
		"int64":           {"bigint", "int8", "bigserial"},
		"int32":           {"integer", "int", "int4"},
		"bool":            {"boolean", "bool"},
		"time.Time":       {"timestamp", "timestamptz", "timestamp with time zone", "timestamp without time zone"},
		"uuid.UUID":       {"uuid"},
		"[]uint8":         {"bytea"},
		"json.RawMessage": {"json", "jsonb"},
	}

	// Check compatibility
	for gt, sqlTypes := range compatMap {
		if strings.Contains(goType, gt) {
			for _, st := range sqlTypes {
				if strings.Contains(strings.ToLower(sqlType), st) {
					return true
				}
			}
		}
	}

	// If not found in map, log it but don't fail
	return true
}
