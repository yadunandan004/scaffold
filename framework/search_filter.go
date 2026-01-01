package framework

import (
	"fmt"
	"strings"
)

type filterOperator string

const FilterOperator filterOperator = ""

type FilterPayload struct {
	Field    string
	Values   []interface{}
	Operator string
}

func (f FilterPayload) ToSQL(argCount *int) (string, []interface{}) {
	var clause string
	var args []interface{}

	switch f.Operator {
	case FilterOperator.Eq():
		clause = fmt.Sprintf("%s = $%d", f.Field, *argCount)
		args = append(args, f.Values[0])
		*argCount++
	case FilterOperator.Ne():
		clause = fmt.Sprintf("%s != $%d", f.Field, *argCount)
		args = append(args, f.Values[0])
		*argCount++
	case FilterOperator.Gt():
		clause = fmt.Sprintf("%s > $%d", f.Field, *argCount)
		args = append(args, f.Values[0])
		*argCount++
	case FilterOperator.Gte():
		clause = fmt.Sprintf("%s >= $%d", f.Field, *argCount)
		args = append(args, f.Values[0])
		*argCount++
	case FilterOperator.Lt():
		clause = fmt.Sprintf("%s < $%d", f.Field, *argCount)
		args = append(args, f.Values[0])
		*argCount++
	case FilterOperator.Lte():
		clause = fmt.Sprintf("%s <= $%d", f.Field, *argCount)
		args = append(args, f.Values[0])
		*argCount++
	case FilterOperator.In():
		if len(f.Values) == 0 {
			return "", nil
		}
		placeholders := make([]string, len(f.Values))
		for i, v := range f.Values {
			placeholders[i] = fmt.Sprintf("$%d", *argCount)
			args = append(args, v)
			*argCount++
		}
		clause = fmt.Sprintf("%s IN (%s)", f.Field, strings.Join(placeholders, ","))
	case FilterOperator.NotIn():
		if len(f.Values) == 0 {
			return "", nil
		}
		placeholders := make([]string, len(f.Values))
		for i, v := range f.Values {
			placeholders[i] = fmt.Sprintf("$%d", *argCount)
			args = append(args, v)
			*argCount++
		}
		clause = fmt.Sprintf("%s NOT IN (%s)", f.Field, strings.Join(placeholders, ","))
	case FilterOperator.IsNull():
		clause = f.Field + " IS NULL"
	case FilterOperator.IsNotNull():
		clause = f.Field + " IS NOT NULL"
	case FilterOperator.Like():
		clause = fmt.Sprintf("%s LIKE $%d", f.Field, *argCount)
		args = append(args, f.Values[0])
		*argCount++
	case FilterOperator.NotLike():
		clause = fmt.Sprintf("%s NOT LIKE $%d", f.Field, *argCount)
		args = append(args, f.Values[0])
		*argCount++
	}

	return clause, args
}

func BuildWhereClause(filters []FilterPayload) (string, []interface{}) {
	if len(filters) == 0 {
		return "", nil
	}

	var whereClauses []string
	var allArgs []interface{}
	argCount := 1

	for _, filter := range filters {
		clause, args := filter.ToSQL(&argCount)
		if clause != "" {
			whereClauses = append(whereClauses, clause)
			allArgs = append(allArgs, args...)
		}
	}

	if len(whereClauses) == 0 {
		return "", nil
	}

	return "WHERE " + strings.Join(whereClauses, " AND "), allArgs
}

type SortPayload struct {
	Fields    []string
	Direction string
}

func BuildOrderByClause(sort *SortPayload) string {
	if sort == nil || len(sort.Fields) == 0 {
		return ""
	}
	direction := "ASC"
	if sort.Direction != "" {
		direction = strings.ToUpper(sort.Direction)
	}
	return " ORDER BY " + strings.Join(sort.Fields, ", ") + " " + direction
}

func BuildPaginationClause(page, take int) string {
	if take <= 0 {
		return ""
	}

	offset := 0
	if page > 1 {
		offset = (page - 1) * take
	}

	return fmt.Sprintf(" LIMIT %d OFFSET %d", take, offset)
}

type SearchRequest struct {
	Filters []FilterPayload
	Sort    *SortPayload
	Page    int      // 1-based page number (default: 0 = no pagination)
	Take    int      // Page size / limit (default: 0 = no limit)
	Columns []string // Columns to select (default: empty = SELECT *)
}

func NewSearchRequest() *SearchRequest {
	return &SearchRequest{
		Filters: []FilterPayload{},
	}
}

func (r *SearchRequest) AddFilter(filter FilterPayload) *SearchRequest {
	r.Filters = append(r.Filters, filter)
	return r
}

func (r *SearchRequest) AddEqual(field string, value interface{}) *SearchRequest {
	return r.AddFilter(*EqualFilter(field, value))
}

func (r *SearchRequest) AddNotEqual(field string, value interface{}) *SearchRequest {
	return r.AddFilter(*NotEqualFilter(field, value))
}

func (r *SearchRequest) AddIn(field string, values ...interface{}) *SearchRequest {
	return r.AddFilter(*InFilter(field, values...))
}

func (r *SearchRequest) AddNotIn(field string, values ...interface{}) *SearchRequest {
	return r.AddFilter(*NotInFilter(field, values...))
}

func (r *SearchRequest) AddGreaterThan(field string, value interface{}) *SearchRequest {
	return r.AddFilter(*GreaterThanFilter(field, value))
}

func (r *SearchRequest) AddGreaterThanOrEqual(field string, value interface{}) *SearchRequest {
	return r.AddFilter(*GreaterThanOrEqualFilter(field, value))
}

func (r *SearchRequest) AddLessThan(field string, value interface{}) *SearchRequest {
	return r.AddFilter(*LessThanFilter(field, value))
}

func (r *SearchRequest) AddLessThanOrEqual(field string, value interface{}) *SearchRequest {
	return r.AddFilter(*LessThanOrEqualFilter(field, value))
}

func (r *SearchRequest) SortBy(fields []string, direction string) *SearchRequest {
	r.Sort = &SortPayload{
		Fields:    fields,
		Direction: direction,
	}
	return r
}

func (r *SearchRequest) SortAsc(fields ...string) *SearchRequest {
	r.Sort = &SortPayload{
		Fields:    fields,
		Direction: "ASC",
	}
	return r
}

func (r *SearchRequest) SortDesc(fields ...string) *SearchRequest {
	r.Sort = &SortPayload{
		Fields:    fields,
		Direction: "DESC",
	}
	return r
}

func (r *SearchRequest) WithPage(pageNum int) *SearchRequest {
	if pageNum < 1 {
		pageNum = 1
	}
	r.Page = pageNum
	return r
}

func (r *SearchRequest) WithTake(pageSize int) *SearchRequest {
	r.Take = pageSize
	return r
}

func (r *SearchRequest) AddColumn(column string) *SearchRequest {
	r.Columns = append(r.Columns, column)
	return r
}

func (r *SearchRequest) AddColumns(columns ...string) *SearchRequest {
	r.Columns = append(r.Columns, columns...)
	return r
}

func (r *SearchRequest) HasColumns() bool {
	return len(r.Columns) > 0
}

func (r *SearchRequest) GetColumns() []string {
	if len(r.Columns) == 0 {
		return nil
	}
	result := make([]string, len(r.Columns))
	copy(result, r.Columns)
	return result
}

func (r *SearchRequest) ToQuery(baseQuery string) (string, []interface{}) {
	whereClause, args := BuildWhereClause(r.Filters)
	orderByClause := BuildOrderByClause(r.Sort)
	paginationClause := BuildPaginationClause(r.Page, r.Take)

	fullQuery := baseQuery
	if whereClause != "" {
		fullQuery += " " + whereClause
	}
	if orderByClause != "" {
		fullQuery += orderByClause
	}
	if paginationClause != "" {
		fullQuery += paginationClause
	}

	return fullQuery, args
}

type FilterGroup struct {
	filters []FilterPayload
}

func NewFilterGroup() *FilterGroup {
	return &FilterGroup{}
}

func (f *FilterGroup) Add(filter *FilterPayload) []FilterPayload {
	f.filters = append(f.filters, *filter)
	return f.filters
}

func baseFilter(field string, operator string, values ...interface{}) *FilterPayload {
	return &FilterPayload{
		Field:    field,
		Values:   values,
		Operator: operator,
	}
}
func EqualFilter(field string, value interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.Eq(), value)
}

func NotEqualFilter(field string, value interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.Ne(), value)
}

func GreaterThanFilter(field string, value interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.Gt(), value)
}

func GreaterThanOrEqualFilter(field string, value interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.Gte(), value)
}
func LessThanFilter(field string, value interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.Lt(), value)
}
func LessThanOrEqualFilter(field string, value interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.Lte(), value)
}

func InFilter(field string, values ...interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.In(), values...)
}

func NotInFilter(field string, values ...interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.NotIn(), values...)
}

func LikeFilter(field string, pattern interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.Like(), pattern)
}

func NotLikeFilter(field string, pattern interface{}) *FilterPayload {
	return baseFilter(field, FilterOperator.NotLike(), pattern)
}

func (f filterOperator) In() string {
	return "in"
}

func (f filterOperator) NotIn() string {
	return "not_in"
}

func (f filterOperator) Eq() string {
	return "eq"
}

func (f filterOperator) Ne() string {
	return "ne"
}

func (f filterOperator) Gt() string {
	return "gt"
}

func (f filterOperator) Gte() string {
	return "gte"
}

func (f filterOperator) Lt() string {
	return "lt"
}

func (f filterOperator) Lte() string {
	return "lte"
}

func (f filterOperator) Xor() string {
	return "xor"
}

func (f filterOperator) IsNull() string {
	return "isnull"
}

func (f filterOperator) IsNotNull() string {
	return "isnotnull"
}

func (f filterOperator) Like() string {
	return "like"
}

func (f filterOperator) NotLike() string {
	return "notlike"
}
