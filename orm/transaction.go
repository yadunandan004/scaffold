package orm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Transaction[T] is now stateless - it's just a helper with metadata
type Transaction[T any] struct {
	metadata *ModelMetadata
}

func NewTransaction[T any]() *Transaction[T] {
	return &Transaction[T]{
		metadata: GetMetadata[T](),
	}
}

func (t *Transaction[T]) Create(query *Query, entity *T) error {
	if query == nil {
		return fmt.Errorf("no transaction in request")
	}
	values := t.metadata.ExtractValues(entity)
	_, err := query.Exec(t.metadata.SQLTemplates.Insert, values...)
	return err
}

func (t *Transaction[T]) Update(query *Query, entity *T) error {
	if query == nil {
		return fmt.Errorf("no transaction in request")
	}
	values := t.metadata.ExtractValues(entity)
	id := t.metadata.ExtractID(entity)

	pkColumn := t.metadata.IDColumn
	updateValues := []interface{}{id}
	for i, field := range t.metadata.Fields {
		if field.Column != pkColumn && field.Column != "created_at" {
			updateValues = append(updateValues, values[i])
		}
	}

	_, err := query.Exec(t.metadata.SQLTemplates.Update, updateValues...)
	return err
}

func (t *Transaction[T]) Delete(query *Query, entity *T) error {
	if query == nil {
		return fmt.Errorf("no transaction in request")
	}
	id := t.metadata.ExtractID(entity)
	_, err := query.Exec(t.metadata.SQLTemplates.Delete, id)
	return err
}

func (t *Transaction[T]) CreateMultiple(query *Query, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}

	if query == nil {
		return fmt.Errorf("no transaction in request")
	}

	var allValues []interface{}
	for _, entity := range entities {
		values := t.metadata.ExtractValues(entity)
		allValues = append(allValues, values...)
	}

	batchSQL := t.metadata.SQLTemplates.BatchInsert(len(entities))

	if t.metadata.IDColumn != "" {
		batchSQL += fmt.Sprintf(" RETURNING %s", t.metadata.IDColumn)
		rows, err := query.Query(batchSQL, allValues...)
		if err != nil {
			return err
		}
		defer rows.Close()

		i := 0
		for rows.Next() {
			if i >= len(entities) {
				break
			}
			var id interface{}
			if err := rows.Scan(&id); err != nil {
				return err
			}
			// Set the ID back on the entity
			if t.metadata.SetID != nil {
				t.metadata.SetID(entities[i], id)
			}
			i++
		}
		return rows.Err()
	}

	_, err := query.Exec(batchSQL, allValues...)
	return err
}

func (t *Transaction[T]) UpdateMultiple(query *Query, entities []*T) error {
	for _, entity := range entities {
		if err := t.Update(query, entity); err != nil {
			return err
		}
	}
	return nil
}

func (t *Transaction[T]) DeleteMultiple(query *Query, entities []*T) error {
	if query == nil {
		return fmt.Errorf("no transaction in request")
	}

	var ids []interface{}
	for _, entity := range entities {
		ids = append(ids, t.metadata.ExtractID(entity))
	}

	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)",
		t.metadata.TableName,
		t.metadata.IDColumn,
		strings.Join(placeholders, ","))

	_, err := query.Exec(deleteSQL, ids...)
	return err
}

func (t *Transaction[T]) FindByPK(query *Query, dest *T, pk interface{}) error {
	if query == nil {
		return fmt.Errorf("no transaction in request")
	}
	row := query.QueryRowRaw(t.metadata.SQLTemplates.SelectByPK, pk)
	return t.metadata.ScanRow(row, dest)
}

func (t *Transaction[T]) FindByQuery(query *Query, querySQL string, args ...interface{}) ([]*T, error) {
	if query == nil {
		return nil, fmt.Errorf("no transaction in request")
	}

	rows, err := query.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*T, 0)
	for rows.Next() {
		var entity T
		if err := t.metadata.ScanRows(rows, &entity); err != nil {
			return nil, err
		}
		results = append(results, &entity)
	}
	return results, rows.Err()
}

func (t *Transaction[T]) FindAll(query *Query) ([]*T, error) {
	if query == nil {
		return nil, fmt.Errorf("no transaction in request")
	}
	rows, err := query.Query(t.metadata.SQLTemplates.SelectAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*T, 0)
	for rows.Next() {
		var entity T
		if err := t.metadata.ScanRows(rows, &entity); err != nil {
			return nil, err
		}
		results = append(results, &entity)
	}
	return results, rows.Err()
}

func (t *Transaction[T]) Upsert(query *Query, entity *T, conflictColumns []string) error {
	if len(conflictColumns) == 0 {
		return t.Create(query, entity)
	}

	if query == nil {
		return fmt.Errorf("no transaction in request")
	}

	values := t.metadata.ExtractValues(entity)

	var returnCols []string
	for _, field := range t.metadata.Fields {
		returnCols = append(returnCols, field.Column)
	}

	var explicitUpdateCols []string
	if provider, ok := any(*entity).(interface{ UpdateColumns() []string }); ok {
		explicitUpdateCols = provider.UpdateColumns()
	}

	if explicitUpdateCols != nil && len(explicitUpdateCols) == 0 {
		upsertSQL := fmt.Sprintf("%s ON CONFLICT (%s) DO NOTHING RETURNING %s",
			t.metadata.SQLTemplates.Insert,
			strings.Join(conflictColumns, ","),
			strings.Join(returnCols, ","))

		row := query.QueryRowRaw(upsertSQL, values...)
		err := t.metadata.ScanRow(row, entity)
		if err == nil {
			return nil
		}
		if err == sql.ErrNoRows {
			whereConditions := make([]string, len(conflictColumns))
			for i, col := range conflictColumns {
				whereConditions[i] = fmt.Sprintf("%s = $%d", col, i+1)
			}
			selectSQL := fmt.Sprintf("SELECT %s FROM %s WHERE %s",
				strings.Join(returnCols, ","),
				t.metadata.TableName,
				strings.Join(whereConditions, " AND "))

			conflictValues := make([]interface{}, len(conflictColumns))
			for i, col := range conflictColumns {
				for j, field := range t.metadata.Fields {
					if field.Column == col {
						conflictValues[i] = values[j]
						break
					}
				}
			}

			row = query.QueryRowRaw(selectSQL, conflictValues...)
			return t.metadata.ScanRow(row, entity)
		}
		return err
	}

	var updateCols []string
	if len(explicitUpdateCols) > 0 {
		for _, col := range explicitUpdateCols {
			updateCols = append(updateCols, fmt.Sprintf("%s=EXCLUDED.%s", col, col))
		}
	} else {
		insertSQL := t.metadata.SQLTemplates.Insert
		startIdx := strings.Index(insertSQL, "(")
		endIdx := strings.Index(insertSQL, ")")
		cols := strings.Split(insertSQL[startIdx+1:endIdx], ",")

		for _, col := range cols {
			colName := strings.TrimSpace(col)
			isConflict := false
			for _, cc := range conflictColumns {
				if colName == cc {
					isConflict = true
					break
				}
			}
			if !isConflict && colName != "created_at" {
				updateCols = append(updateCols, fmt.Sprintf("%s=EXCLUDED.%s", colName, colName))
			}
		}
	}

	upsertSQL := fmt.Sprintf("%s ON CONFLICT (%s) DO UPDATE SET %s RETURNING %s",
		t.metadata.SQLTemplates.Insert,
		strings.Join(conflictColumns, ","),
		strings.Join(updateCols, ","),
		strings.Join(returnCols, ","))

	row := query.QueryRowRaw(upsertSQL, values...)
	return t.metadata.ScanRow(row, entity)
}

// Commit commits the transaction
func (t *Transaction[T]) Commit(query *Query) error {
	if query == nil {
		return fmt.Errorf("no transaction in request")
	}
	return query.Commit()
}

// Rollback rolls back the transaction
func (t *Transaction[T]) Rollback(query *Query) error {
	if query == nil {
		return fmt.Errorf("no transaction in request")
	}
	return query.Rollback()
}

type DB[T any] struct {
	db       *sql.DB
	metadata *ModelMetadata
}

func NewDB[T any](db *sql.DB) *DB[T] {
	if db == nil {
		return nil
	}
	return &DB[T]{
		db:       db,
		metadata: GetMetadata[T](),
	}
}

func (d *DB[T]) Create(ctx context.Context, entity *T) error {
	values := d.metadata.ExtractValues(entity)
	_, err := d.db.ExecContext(ctx, d.metadata.SQLTemplates.Insert, values...)
	return err
}

func (d *DB[T]) Update(ctx context.Context, entity *T) error {
	values := d.metadata.ExtractValues(entity)
	id := d.metadata.ExtractID(entity)

	pkColumn := d.metadata.IDColumn
	updateValues := []interface{}{id}
	for i, field := range d.metadata.Fields {
		if field.Column != pkColumn && field.Column != "created_at" {
			updateValues = append(updateValues, values[i])
		}
	}

	_, err := d.db.ExecContext(ctx, d.metadata.SQLTemplates.Update, updateValues...)
	return err
}

func (d *DB[T]) Delete(ctx context.Context, entity *T) error {
	id := d.metadata.ExtractID(entity)
	_, err := d.db.ExecContext(ctx, d.metadata.SQLTemplates.Delete, id)
	return err
}

func (d *DB[T]) CreateMultiple(ctx context.Context, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}

	var allValues []interface{}
	for _, entity := range entities {
		values := d.metadata.ExtractValues(entity)
		allValues = append(allValues, values...)
	}

	batchSQL := d.metadata.SQLTemplates.BatchInsert(len(entities))

	// For tables with auto-generated IDs, we need to get them back
	if d.metadata.IDColumn != "" {
		batchSQL += fmt.Sprintf(" RETURNING %s", d.metadata.IDColumn)
		rows, err := d.db.QueryContext(ctx, batchSQL, allValues...)
		if err != nil {
			return err
		}
		defer rows.Close()

		i := 0
		for rows.Next() {
			if i >= len(entities) {
				break
			}
			var id interface{}
			if err := rows.Scan(&id); err != nil {
				return err
			}
			// Set the ID back on the entity
			if d.metadata.SetID != nil {
				d.metadata.SetID(entities[i], id)
			}
			i++
		}
		return rows.Err()
	}

	_, err := d.db.ExecContext(ctx, batchSQL, allValues...)
	return err
}

func (d *DB[T]) UpdateMultiple(ctx context.Context, entities []*T) error {
	for _, entity := range entities {
		if err := d.Update(ctx, entity); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB[T]) DeleteMultiple(ctx context.Context, entities []*T) error {
	var ids []interface{}
	for _, entity := range entities {
		ids = append(ids, d.metadata.ExtractID(entity))
	}

	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)",
		d.metadata.TableName,
		d.metadata.IDColumn,
		strings.Join(placeholders, ","))

	_, err := d.db.ExecContext(ctx, query, ids...)
	return err
}

func (d *DB[T]) FindByPK(ctx context.Context, dest *T, pk interface{}) error {
	row := d.db.QueryRowContext(ctx, d.metadata.SQLTemplates.SelectByPK, pk)
	return d.metadata.ScanRow(row, dest)
}

func (d *DB[T]) FindByQuery(ctx context.Context, querySQL string, args ...interface{}) ([]*T, error) {
	rows, err := d.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*T, 0)
	for rows.Next() {
		var entity T
		if err := d.metadata.ScanRows(rows, &entity); err != nil {
			return nil, err
		}
		results = append(results, &entity)
	}
	return results, rows.Err()
}

func (d *DB[T]) FindAll(ctx context.Context) ([]*T, error) {
	rows, err := d.db.QueryContext(ctx, d.metadata.SQLTemplates.SelectAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*T, 0)
	for rows.Next() {
		var entity T
		if err := d.metadata.ScanRows(rows, &entity); err != nil {
			return nil, err
		}
		results = append(results, &entity)
	}
	return results, rows.Err()
}

func (d *DB[T]) Upsert(ctx context.Context, entity *T, conflictColumns []string) error {
	if len(conflictColumns) == 0 {
		return d.Create(ctx, entity)
	}

	values := d.metadata.ExtractValues(entity)

	var updateCols []string
	insertSQL := d.metadata.SQLTemplates.Insert
	startIdx := strings.Index(insertSQL, "(")
	endIdx := strings.Index(insertSQL, ")")
	cols := strings.Split(insertSQL[startIdx+1:endIdx], ",")

	for _, col := range cols {
		colName := strings.TrimSpace(col)
		isConflict := false
		for _, cc := range conflictColumns {
			if colName == cc {
				isConflict = true
				break
			}
		}
		if !isConflict && colName != "created_at" {
			updateCols = append(updateCols, fmt.Sprintf("%s=EXCLUDED.%s", colName, colName))
		}
	}

	var returnCols []string
	for _, field := range d.metadata.Fields {
		returnCols = append(returnCols, field.Column)
	}

	upsertSQL := fmt.Sprintf("%s ON CONFLICT (%s) DO UPDATE SET %s RETURNING %s",
		d.metadata.SQLTemplates.Insert,
		strings.Join(conflictColumns, ","),
		strings.Join(updateCols, ","),
		strings.Join(returnCols, ","))

	row := d.db.QueryRowContext(ctx, upsertSQL, values...)
	return d.metadata.ScanRow(row, entity)
}
