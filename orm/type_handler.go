package orm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"time"
	"unsafe"

	"github.com/google/uuid"
)

var (
	valuerType     = reflect.TypeOf((*driver.Valuer)(nil)).Elem()
	uuidType       = reflect.TypeOf(uuid.UUID{})
	timeType       = reflect.TypeOf(time.Time{})
	rawMessageType = reflect.TypeOf(json.RawMessage{})
)

type TypeHandler interface {
	CanHandle(typ reflect.Type) bool
	ExtractValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error)
	ScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{}
}

var typeHandlers = []TypeHandler{
	&PrimitiveHandler{},
	&PointerHandler{},
	&ValuerHandler{},
	&SpecialTypeHandler{},
	&RawMessageHandler{},
	&SliceHandler{},
	&JSONHandler{},
	&FallbackHandler{},
}

func extractFieldValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error) {
	for _, handler := range typeHandlers {
		if handler.CanHandle(typ) {
			return handler.ExtractValue(fieldPtr, typ)
		}
	}
	return nil, fmt.Errorf("unsupported type: %v", typ)
}

func createScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{} {
	for _, handler := range typeHandlers {
		if handler.CanHandle(typ) {
			return handler.ScanTarget(fieldPtr, typ)
		}
	}
	return reflect.NewAt(typ, fieldPtr).Interface()
}

type ValuerHandler struct{}

func (h *ValuerHandler) CanHandle(typ reflect.Type) bool {
	if typ.Implements(valuerType) {
		return true
	}
	if typ.Kind() != reflect.Ptr && reflect.PtrTo(typ).Implements(valuerType) {
		return true
	}
	return false
}

func (h *ValuerHandler) ExtractValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error) {
	if typ.Implements(valuerType) {
		valuer := reflect.NewAt(typ, fieldPtr).Interface().(driver.Valuer)
		return valuer.Value()
	}
	valuer := reflect.NewAt(typ, fieldPtr).Elem().Interface().(driver.Valuer)
	return valuer.Value()
}

func (h *ValuerHandler) ScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{} {
	return reflect.NewAt(typ, fieldPtr).Interface()
}

type PrimitiveHandler struct{}

func (h *PrimitiveHandler) CanHandle(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.String, reflect.Int, reflect.Int64, reflect.Bool, reflect.Float64:
		return true
	}
	return false
}

func (h *PrimitiveHandler) ExtractValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error) {
	switch typ.Kind() {
	case reflect.String:
		return *(*string)(fieldPtr), nil
	case reflect.Int:
		return int64(*(*int)(fieldPtr)), nil
	case reflect.Int64:
		return *(*int64)(fieldPtr), nil
	case reflect.Bool:
		return *(*bool)(fieldPtr), nil
	case reflect.Float64:
		return *(*float64)(fieldPtr), nil
	}
	return nil, fmt.Errorf("unsupported primitive type: %v", typ)
}

func (h *PrimitiveHandler) ScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{} {
	return &primitiveScanner{
		fieldPtr: fieldPtr,
		typ:      typ,
	}
}

type primitiveScanner struct {
	fieldPtr unsafe.Pointer
	typ      reflect.Type
}

func (s *primitiveScanner) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	switch s.typ.Kind() {
	case reflect.String:
		strPtr := (*string)(s.fieldPtr)
		switch v := src.(type) {
		case string:
			*strPtr = v
		case []byte:
			*strPtr = string(v)
		default:
			return fmt.Errorf("cannot scan %T into string", src)
		}
	case reflect.Int:
		intPtr := (*int)(s.fieldPtr)
		switch v := src.(type) {
		case int64:
			*intPtr = int(v)
		case int:
			*intPtr = v
		case []byte:
			val := reflect.ValueOf(intPtr).Elem()
			if _, err := fmt.Sscan(string(v), val.Addr().Interface()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot scan %T into int", src)
		}
	case reflect.Int64:
		int64Ptr := (*int64)(s.fieldPtr)
		switch v := src.(type) {
		case int64:
			*int64Ptr = v
		case int:
			*int64Ptr = int64(v)
		case []byte:
			val := reflect.ValueOf(int64Ptr).Elem()
			if _, err := fmt.Sscan(string(v), val.Addr().Interface()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot scan %T into int64", src)
		}
	case reflect.Bool:
		boolPtr := (*bool)(s.fieldPtr)
		switch v := src.(type) {
		case bool:
			*boolPtr = v
		case []byte:
			val := reflect.ValueOf(boolPtr).Elem()
			if _, err := fmt.Sscan(string(v), val.Addr().Interface()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot scan %T into bool", src)
		}
	case reflect.Float64:
		float64Ptr := (*float64)(s.fieldPtr)
		switch v := src.(type) {
		case float64:
			*float64Ptr = v
		case []byte:
			val := reflect.ValueOf(float64Ptr).Elem()
			if _, err := fmt.Sscan(string(v), val.Addr().Interface()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot scan %T into float64", src)
		}
	default:
		return fmt.Errorf("unsupported primitive type: %v", s.typ)
	}

	return nil
}

type PointerHandler struct{}

func (h *PointerHandler) CanHandle(typ reflect.Type) bool {
	return typ.Kind() == reflect.Ptr
}

func (h *PointerHandler) ExtractValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error) {
	ptrToPtr := (*unsafe.Pointer)(fieldPtr)
	if *ptrToPtr == nil {
		return nil, nil
	}

	elemType := typ.Elem()
	actualValuePtr := *ptrToPtr

	return extractFieldValue(actualValuePtr, elemType)
}

func (h *PointerHandler) ScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{} {
	return &pointerScanner{
		fieldPtr: fieldPtr,
		typ:      typ,
	}
}

type pointerScanner struct {
	fieldPtr unsafe.Pointer
	typ      reflect.Type
}

func (s *pointerScanner) Scan(src interface{}) error {
	if src == nil {
		ptrToPtr := (*unsafe.Pointer)(s.fieldPtr)
		*ptrToPtr = nil
		return nil
	}

	elemType := s.typ.Elem()
	newElem := reflect.New(elemType)
	elemPtr := newElem.UnsafePointer()

	scanner := createScanTarget(elemPtr, elemType)

	if sqlScanner, ok := scanner.(interface{ Scan(interface{}) error }); ok {
		if err := sqlScanner.Scan(src); err != nil {
			return err
		}
	} else {
		scannerValue := reflect.NewAt(elemType, elemPtr)
		srcValue := reflect.ValueOf(src)
		if scannerValue.Elem().CanSet() && srcValue.Type().AssignableTo(scannerValue.Elem().Type()) {
			scannerValue.Elem().Set(srcValue)
		} else {
			return fmt.Errorf("cannot scan %T into %v", src, elemType)
		}
	}

	ptrToPtr := (*unsafe.Pointer)(s.fieldPtr)
	*ptrToPtr = elemPtr

	return nil
}

func convertAndScan(scanner interface{}, src interface{}) error {
	if sqlScanner, ok := scanner.(interface{ Scan(interface{}) error }); ok {
		return sqlScanner.Scan(src)
	}

	srcValue := reflect.ValueOf(src)
	scannerValue := reflect.ValueOf(scanner)
	if scannerValue.Kind() == reflect.Ptr && scannerValue.Elem().CanSet() {
		scannerValue.Elem().Set(srcValue)
		return nil
	}

	return fmt.Errorf("cannot scan into %T", scanner)
}

type SliceHandler struct{}

func (h *SliceHandler) CanHandle(typ reflect.Type) bool {
	if typ.Kind() != reflect.Slice {
		return false
	}
	if typ.Implements(valuerType) {
		return true
	}
	if reflect.PtrTo(typ).Implements(valuerType) {
		return true
	}
	return false
}

func (h *SliceHandler) ExtractValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error) {
	if typ.Implements(valuerType) {
		valuer := reflect.NewAt(typ, fieldPtr).Interface().(driver.Valuer)
		return valuer.Value()
	}
	valuer := reflect.NewAt(typ, fieldPtr).Elem().Interface().(driver.Valuer)
	return valuer.Value()
}

func (h *SliceHandler) ScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{} {
	return reflect.NewAt(typ, fieldPtr).Interface()
}

type SpecialTypeHandler struct{}

func (h *SpecialTypeHandler) CanHandle(typ reflect.Type) bool {
	return typ == uuidType || typ == timeType
}

func (h *SpecialTypeHandler) ExtractValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error) {
	if typ == uuidType {
		return *(*uuid.UUID)(fieldPtr), nil
	}
	if typ == timeType {
		return *(*time.Time)(fieldPtr), nil
	}
	return nil, fmt.Errorf("unsupported special type: %v", typ)
}

func (h *SpecialTypeHandler) ScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{} {
	if typ == uuidType {
		return (*uuid.UUID)(fieldPtr)
	}
	if typ == timeType {
		return (*time.Time)(fieldPtr)
	}
	return nil
}

type RawMessageHandler struct{}

func (h *RawMessageHandler) CanHandle(typ reflect.Type) bool {
	return typ == rawMessageType
}

func (h *RawMessageHandler) ExtractValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error) {
	rawMsg := *(*json.RawMessage)(fieldPtr)
	if rawMsg == nil {
		return []byte("null"), nil
	}
	return []byte(rawMsg), nil
}

func (h *RawMessageHandler) ScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{} {
	return &rawMessageScanner{
		fieldPtr: fieldPtr,
	}
}

type rawMessageScanner struct {
	fieldPtr unsafe.Pointer
}

func (s *rawMessageScanner) Scan(src interface{}) error {
	if src == nil {
		*(*json.RawMessage)(s.fieldPtr) = nil
		return nil
	}

	var bytes []byte
	switch v := src.(type) {
	case []byte:
		bytes = make([]byte, len(v))
		copy(bytes, v)
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan type %T into json.RawMessage", src)
	}

	*(*json.RawMessage)(s.fieldPtr) = json.RawMessage(bytes)
	return nil
}

type JSONHandler struct{}

func (h *JSONHandler) CanHandle(typ reflect.Type) bool {
	kind := typ.Kind()

	if kind == reflect.Slice {
		if typ.Elem().Kind() == reflect.Uint8 {
			return false
		}
		return true
	}

	if kind == reflect.Map {
		return true
	}

	if kind == reflect.Struct {
		if typ == uuidType || typ == timeType {
			return false
		}
		return true
	}

	return false
}

func (h *JSONHandler) ExtractValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error) {
	value := reflect.NewAt(typ, fieldPtr).Elem()

	if value.IsZero() {
		kind := typ.Kind()
		if kind == reflect.Slice {
			return []byte("[]"), nil
		}
		if kind == reflect.Map {
			return []byte("{}"), nil
		}
	}

	bytes, err := json.Marshal(value.Interface())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %v: %w", typ, err)
	}
	return bytes, nil
}

func (h *JSONHandler) ScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{} {
	return &jsonScanner{
		fieldPtr: fieldPtr,
		typ:      typ,
	}
}

type jsonScanner struct {
	fieldPtr unsafe.Pointer
	typ      reflect.Type
}

func (s *jsonScanner) Scan(src interface{}) error {
	if src == nil {
		destValue := reflect.NewAt(s.typ, s.fieldPtr).Elem()
		kind := s.typ.Kind()

		if kind == reflect.Slice {
			destValue.Set(reflect.MakeSlice(s.typ, 0, 0))
		} else if kind == reflect.Map {
			destValue.Set(reflect.MakeMap(s.typ))
		} else if kind == reflect.Struct {
			destValue.Set(reflect.Zero(s.typ))
		}
		return nil
	}

	var bytes []byte
	switch v := src.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan type %T into JSON field", src)
	}

	destValue := reflect.NewAt(s.typ, s.fieldPtr).Elem()
	if err := json.Unmarshal(bytes, destValue.Addr().Interface()); err != nil {
		return fmt.Errorf("failed to unmarshal into %v: %w", s.typ, err)
	}

	return nil
}

type FallbackHandler struct{}

func (h *FallbackHandler) CanHandle(typ reflect.Type) bool {
	return true
}

func (h *FallbackHandler) ExtractValue(fieldPtr unsafe.Pointer, typ reflect.Type) (driver.Value, error) {
	value := reflect.NewAt(typ, fieldPtr).Elem()
	if value.Kind() == reflect.Slice && value.IsNil() {
		return nil, nil
	}
	return value.Interface(), nil
}

func (h *FallbackHandler) ScanTarget(fieldPtr unsafe.Pointer, typ reflect.Type) interface{} {
	return reflect.NewAt(typ, fieldPtr).Interface()
}
