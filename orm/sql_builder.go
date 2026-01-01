package orm

import (
	"fmt"
	"strings"
)

func buildSQLTemplates(schema, tableName string, insertColumns, columnNames []string, pkColumn string) SQLTemplates {
	fullTableName := tableName
	if schema != "" && schema != "public" {
		fullTableName = schema + "." + tableName
	}

	placeholders := make([]string, len(insertColumns))
	for i := range insertColumns {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	updatePairs := make([]string, 0)
	updateIdx := 2
	for _, col := range columnNames {
		if col != pkColumn && col != "created_at" {
			updatePairs = append(updatePairs, fmt.Sprintf("%s=$%d", col, updateIdx))
			updateIdx++
		}
	}

	return SQLTemplates{
		Insert:      buildInsertSQL(fullTableName, insertColumns, placeholders),
		Update:      buildUpdateSQL(fullTableName, updatePairs, pkColumn),
		Delete:      buildDeleteSQL(fullTableName, pkColumn),
		SelectByPK:  buildSelectByPKSQL(fullTableName, columnNames, pkColumn),
		SelectAll:   buildSelectAllSQL(fullTableName, columnNames),
		TableName:   fullTableName,
		BatchInsert: buildBatchInsertFunc(fullTableName, insertColumns),
	}
}

func buildInsertSQL(tableName string, columns, placeholders []string) string {
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ","),
		strings.Join(placeholders, ","),
	)
}

func buildUpdateSQL(tableName string, updatePairs []string, pkColumn string) string {
	return fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s=$1",
		tableName,
		strings.Join(updatePairs, ","),
		pkColumn,
	)
}

func buildDeleteSQL(tableName string, pkColumn string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE %s=$1", tableName, pkColumn)
}

func buildSelectByPKSQL(tableName string, columns []string, pkColumn string) string {
	return fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s=$1",
		strings.Join(columns, ","),
		tableName,
		pkColumn,
	)
}

func buildSelectAllSQL(tableName string, columns []string) string {
	return fmt.Sprintf(
		"SELECT %s FROM %s",
		strings.Join(columns, ","),
		tableName,
	)
}

func buildBatchInsertFunc(tableName string, columns []string) func(int) string {
	return func(count int) string {
		var valueSets []string
		for i := 0; i < count; i++ {
			var vals []string
			for j := range columns {
				vals = append(vals, fmt.Sprintf("$%d", i*len(columns)+j+1))
			}
			valueSets = append(valueSets, fmt.Sprintf("(%s)", strings.Join(vals, ",")))
		}
		return fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES %s",
			tableName,
			strings.Join(columns, ","),
			strings.Join(valueSets, ","),
		)
	}
}
