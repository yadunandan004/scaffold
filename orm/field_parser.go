package orm

import (
	"reflect"
	"strings"
)

type FieldOptions struct {
	Column          string
	IsPK            bool
	IsAutoIncrement bool
}

func parseFields(typ reflect.Type) ([]FieldMetadata, []uintptr, []reflect.Type, []string, int) {
	var fields []FieldMetadata
	var fieldOffsets []uintptr
	var fieldTypes []reflect.Type
	var columnNames []string
	pkIndex := -1

	parseFieldsRecursive(typ, 0, &fields, &fieldOffsets, &fieldTypes, &columnNames, &pkIndex)

	return fields, fieldOffsets, fieldTypes, columnNames, pkIndex
}

func parseFieldsRecursive(
	typ reflect.Type,
	baseOffset uintptr,
	fields *[]FieldMetadata,
	fieldOffsets *[]uintptr,
	fieldTypes *[]reflect.Type,
	columnNames *[]string,
	pkIndex *int,
) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			parseFieldsRecursive(
				field.Type,
				baseOffset+field.Offset,
				fields,
				fieldOffsets,
				fieldTypes,
				columnNames,
				pkIndex,
			)
			continue
		}

		tag := field.Tag.Get("orm")
		if tag == "-" {
			continue
		}

		opts := parseORMTag(field.Name, tag)

		if opts.IsPK {
			*pkIndex = len(*fieldOffsets)
		}

		*fields = append(*fields, FieldMetadata{
			Name:            field.Name,
			Column:          opts.Column,
			Type:            field.Type,
			Offset:          baseOffset + field.Offset,
			IsPK:            opts.IsPK,
			IsAutoIncrement: opts.IsAutoIncrement,
			Index:           i,
		})

		*fieldOffsets = append(*fieldOffsets, baseOffset+field.Offset)
		*fieldTypes = append(*fieldTypes, field.Type)
		*columnNames = append(*columnNames, opts.Column)
	}
}

func parseORMTag(fieldName string, tag string) FieldOptions {
	opts := FieldOptions{
		Column: fieldName,
	}

	if tag == "" {
		return opts
	}

	parts := strings.Split(tag, ";")
	for _, part := range parts {
		if strings.HasPrefix(part, "column:") {
			opts.Column = strings.TrimPrefix(part, "column:")
		} else if part == "pk" {
			opts.IsPK = true
		} else if part == "auto" {
			opts.IsAutoIncrement = true
		}
	}

	return opts
}

func discoverTableName(typ reflect.Type, model interface{}) (schema string, tableName string) {
	if tn, ok := model.(interface{ TableName() string }); ok {
		fullName := tn.TableName()
		parts := strings.Split(fullName, ".")
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "public", fullName
	}
	return "public", strings.ToLower(typ.Name()) + "s"
}

func filterInsertFields(fields []FieldMetadata, columnNames []string) ([]string, []int) {
	insertColumns := make([]string, 0, len(columnNames))
	insertIndices := make([]int, 0, len(columnNames))

	for i, field := range fields {
		if field.IsAutoIncrement {
			continue
		}
		insertColumns = append(insertColumns, columnNames[i])
		insertIndices = append(insertIndices, i)
	}

	return insertColumns, insertIndices
}
