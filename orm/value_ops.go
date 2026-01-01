package orm

import (
	"database/sql"
	"reflect"
	"unsafe"

	"github.com/google/uuid"
)

func makeExtractValues[T any](fieldOffsets []uintptr, fieldTypes []reflect.Type, insertIndices []int) func(interface{}) []interface{} {
	return func(entity interface{}) []interface{} {
		ptr := uintptr(unsafe.Pointer(entity.(*T)))
		values := make([]interface{}, len(insertIndices))

		for valueIdx, fieldIdx := range insertIndices {
			offset := fieldOffsets[fieldIdx]
			fieldPtr := unsafe.Pointer(ptr + offset)
			fieldType := fieldTypes[fieldIdx]

			val, err := extractFieldValue(fieldPtr, fieldType)
			if err != nil {
				values[valueIdx] = nil
			} else {
				values[valueIdx] = val
			}
		}
		return values
	}
}

func makeScanRow[T any](fieldOffsets []uintptr, fieldTypes []reflect.Type) func(*sql.Row, interface{}) error {
	return func(row *sql.Row, dest interface{}) error {
		ptr := uintptr(unsafe.Pointer(dest.(*T)))
		scanTargets := make([]interface{}, len(fieldOffsets))

		for i := range fieldOffsets {
			fieldPtr := unsafe.Pointer(ptr + fieldOffsets[i])
			scanTargets[i] = createScanTarget(fieldPtr, fieldTypes[i])
		}

		return row.Scan(scanTargets...)
	}
}

func makeScanRows[T any](fieldOffsets []uintptr, fieldTypes []reflect.Type, columnNames []string) func(*sql.Rows, interface{}) error {
	fieldMap := make(map[string]int, len(columnNames))
	for i, colName := range columnNames {
		fieldMap[colName] = i
	}

	return func(rows *sql.Rows, dest interface{}) error {
		columns, err := rows.Columns()
		if err != nil {
			return err
		}

		ptr := uintptr(unsafe.Pointer(dest.(*T)))
		scanTargets := make([]interface{}, len(columns))

		for i, col := range columns {
			fieldIdx, exists := fieldMap[col]
			if !exists {
				var discard interface{}
				scanTargets[i] = &discard
				continue
			}

			fieldPtr := unsafe.Pointer(ptr + fieldOffsets[fieldIdx])
			scanTargets[i] = createScanTarget(fieldPtr, fieldTypes[fieldIdx])
		}

		return rows.Scan(scanTargets...)
	}
}

func makeExtractID[T any](fieldOffsets []uintptr, fieldTypes []reflect.Type, pkIndex int) func(interface{}) interface{} {
	return func(entity interface{}) interface{} {
		if pkIndex < 0 {
			return nil
		}
		ptr := uintptr(unsafe.Pointer(entity.(*T)))
		fieldPtr := unsafe.Pointer(ptr + fieldOffsets[pkIndex])

		if fieldTypes[pkIndex] == uuidType {
			return *(*uuid.UUID)(fieldPtr)
		}
		return *(*int64)(fieldPtr)
	}
}

func makeSetID[T any](fieldOffsets []uintptr, fieldTypes []reflect.Type, pkIndex int) func(interface{}, interface{}) {
	return func(entity interface{}, id interface{}) {
		if pkIndex < 0 {
			return
		}
		ptr := uintptr(unsafe.Pointer(entity.(*T)))
		fieldPtr := unsafe.Pointer(ptr + fieldOffsets[pkIndex])

		if fieldTypes[pkIndex] == uuidType {
			if uuidVal, ok := id.(uuid.UUID); ok {
				*(*uuid.UUID)(fieldPtr) = uuidVal
			}
		} else if fieldTypes[pkIndex].Kind() == reflect.Int64 {
			if intVal, ok := id.(int64); ok {
				*(*int64)(fieldPtr) = intVal
			}
		} else if fieldTypes[pkIndex].Kind() == reflect.Int {
			if intVal, ok := id.(int64); ok {
				*(*int)(fieldPtr) = int(intVal)
			}
		}
	}
}
