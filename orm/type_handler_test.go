package orm

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
)

type testStruct struct {
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

type testNestedStruct struct {
	Name   string     `json:"name"`
	Nested testStruct `json:"nested"`
}

func TestJSONHandler_SliceOfStructs(t *testing.T) {
	handler := &JSONHandler{}

	testData := []testStruct{
		{Field1: "test1", Field2: 1},
		{Field1: "test2", Field2: 2},
	}

	typ := reflect.TypeOf(testData)
	if !handler.CanHandle(typ) {
		t.Fatalf("JSONHandler should handle []testStruct")
	}

	value, err := handler.ExtractValue(unsafe.Pointer(&testData), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	bytes, ok := value.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", value)
	}

	var result []testStruct
	if err := json.Unmarshal(bytes, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !reflect.DeepEqual(testData, result) {
		t.Fatalf("expected %+v, got %+v", testData, result)
	}
}

func TestJSONHandler_MapStringInterface(t *testing.T) {
	handler := &JSONHandler{}

	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	typ := reflect.TypeOf(testData)
	if !handler.CanHandle(typ) {
		t.Fatalf("JSONHandler should handle map[string]interface{}")
	}

	value, err := handler.ExtractValue(unsafe.Pointer(&testData), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	bytes, ok := value.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", value)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(result) != len(testData) {
		t.Fatalf("expected length %d, got %d", len(testData), len(result))
	}
}

func TestJSONHandler_MapStringFloat64(t *testing.T) {
	handler := &JSONHandler{}

	testData := map[string]float64{
		"x": 1.5,
		"y": 2.5,
	}

	typ := reflect.TypeOf(testData)
	if !handler.CanHandle(typ) {
		t.Fatalf("JSONHandler should handle map[string]float64")
	}

	value, err := handler.ExtractValue(unsafe.Pointer(&testData), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	bytes, ok := value.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", value)
	}

	var result map[string]float64
	if err := json.Unmarshal(bytes, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !reflect.DeepEqual(testData, result) {
		t.Fatalf("expected %+v, got %+v", testData, result)
	}
}

func TestJSONHandler_NestedStruct(t *testing.T) {
	handler := &JSONHandler{}

	testData := testNestedStruct{
		Name: "parent",
		Nested: testStruct{
			Field1: "nested",
			Field2: 42,
		},
	}

	typ := reflect.TypeOf(testData)
	if !handler.CanHandle(typ) {
		t.Fatalf("JSONHandler should handle nested structs")
	}

	value, err := handler.ExtractValue(unsafe.Pointer(&testData), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	bytes, ok := value.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", value)
	}

	var result testNestedStruct
	if err := json.Unmarshal(bytes, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !reflect.DeepEqual(testData, result) {
		t.Fatalf("expected %+v, got %+v", testData, result)
	}
}

func TestJSONHandler_EmptySlice(t *testing.T) {
	handler := &JSONHandler{}

	testData := []testStruct{}

	typ := reflect.TypeOf(testData)
	value, err := handler.ExtractValue(unsafe.Pointer(&testData), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	bytes, ok := value.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", value)
	}

	expected := "[]"
	if string(bytes) != expected {
		t.Fatalf("expected %s, got %s", expected, string(bytes))
	}
}

func TestJSONHandler_EmptyMap(t *testing.T) {
	handler := &JSONHandler{}

	testData := make(map[string]interface{})

	typ := reflect.TypeOf(testData)
	value, err := handler.ExtractValue(unsafe.Pointer(&testData), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	bytes, ok := value.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", value)
	}

	expected := "{}"
	if string(bytes) != expected {
		t.Fatalf("expected %s, got %s", expected, string(bytes))
	}
}

func TestJSONHandler_DoesNotHandleByteSlice(t *testing.T) {
	handler := &JSONHandler{}

	testData := []byte{1, 2, 3}
	typ := reflect.TypeOf(testData)

	if handler.CanHandle(typ) {
		t.Fatalf("JSONHandler should NOT handle []byte")
	}
}

func TestJSONHandler_DoesNotHandleUUID(t *testing.T) {
	handler := &JSONHandler{}

	testData := uuid.New()
	typ := reflect.TypeOf(testData)

	if handler.CanHandle(typ) {
		t.Fatalf("JSONHandler should NOT handle uuid.UUID")
	}
}

func TestJSONHandler_DoesNotHandleTime(t *testing.T) {
	handler := &JSONHandler{}

	testData := time.Now()
	typ := reflect.TypeOf(testData)

	if handler.CanHandle(typ) {
		t.Fatalf("JSONHandler should NOT handle time.Time")
	}
}

func TestJSONScanner_ScanNil(t *testing.T) {
	var result []testStruct
	typ := reflect.TypeOf(result)
	scanner := &jsonScanner{
		fieldPtr: unsafe.Pointer(&result),
		typ:      typ,
	}

	if err := scanner.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) failed: %v", err)
	}

	if result == nil {
		t.Fatalf("expected non-nil slice, got nil")
	}

	if len(result) != 0 {
		t.Fatalf("expected empty slice, got length %d", len(result))
	}
}

func TestJSONScanner_ScanBytes(t *testing.T) {
	data := []testStruct{
		{Field1: "test", Field2: 123},
	}
	jsonBytes, _ := json.Marshal(data)

	var result []testStruct
	typ := reflect.TypeOf(result)
	scanner := &jsonScanner{
		fieldPtr: unsafe.Pointer(&result),
		typ:      typ,
	}

	if err := scanner.Scan(jsonBytes); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if !reflect.DeepEqual(data, result) {
		t.Fatalf("expected %+v, got %+v", data, result)
	}
}

func TestJSONScanner_ScanString(t *testing.T) {
	data := map[string]interface{}{
		"key": "value",
	}
	jsonBytes, _ := json.Marshal(data)
	jsonString := string(jsonBytes)

	var result map[string]interface{}
	typ := reflect.TypeOf(result)
	scanner := &jsonScanner{
		fieldPtr: unsafe.Pointer(&result),
		typ:      typ,
	}

	if err := scanner.Scan(jsonString); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result["key"] != "value" {
		t.Fatalf("expected 'value', got %v", result["key"])
	}
}

func TestPrimitiveHandler(t *testing.T) {
	handler := &PrimitiveHandler{}

	t.Run("string", func(t *testing.T) {
		value := "test"
		typ := reflect.TypeOf(value)
		if !handler.CanHandle(typ) {
			t.Fatalf("PrimitiveHandler should handle string")
		}

		val, err := handler.ExtractValue(unsafe.Pointer(&value), typ)
		if err != nil {
			t.Fatalf("ExtractValue failed: %v", err)
		}

		if val != "test" {
			t.Fatalf("expected 'test', got %v", val)
		}
	})

	t.Run("int", func(t *testing.T) {
		value := 42
		typ := reflect.TypeOf(value)
		val, err := handler.ExtractValue(unsafe.Pointer(&value), typ)
		if err != nil {
			t.Fatalf("ExtractValue failed: %v", err)
		}

		if val != int64(42) {
			t.Fatalf("expected int64(42), got %v", val)
		}
	})

	t.Run("bool", func(t *testing.T) {
		value := true
		typ := reflect.TypeOf(value)
		val, err := handler.ExtractValue(unsafe.Pointer(&value), typ)
		if err != nil {
			t.Fatalf("ExtractValue failed: %v", err)
		}

		if val != true {
			t.Fatalf("expected true, got %v", val)
		}
	})
}

func TestSpecialTypeHandler_UUID(t *testing.T) {
	handler := &SpecialTypeHandler{}

	testUUID := uuid.New()
	typ := reflect.TypeOf(testUUID)

	if !handler.CanHandle(typ) {
		t.Fatalf("SpecialTypeHandler should handle uuid.UUID")
	}

	val, err := handler.ExtractValue(unsafe.Pointer(&testUUID), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	resultUUID, ok := val.(uuid.UUID)
	if !ok {
		t.Fatalf("expected uuid.UUID, got %T", val)
	}

	if resultUUID != testUUID {
		t.Fatalf("expected %v, got %v", testUUID, resultUUID)
	}
}

func TestSpecialTypeHandler_Time(t *testing.T) {
	handler := &SpecialTypeHandler{}

	testTime := time.Now()
	typ := reflect.TypeOf(testTime)

	if !handler.CanHandle(typ) {
		t.Fatalf("SpecialTypeHandler should handle time.Time")
	}

	val, err := handler.ExtractValue(unsafe.Pointer(&testTime), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	resultTime, ok := val.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", val)
	}

	if !resultTime.Equal(testTime) {
		t.Fatalf("expected %v, got %v", testTime, resultTime)
	}
}

func TestPointerHandler_NilPointer(t *testing.T) {
	handler := &PointerHandler{}

	var nilPtr *string
	typ := reflect.TypeOf(nilPtr)

	if !handler.CanHandle(typ) {
		t.Fatalf("PointerHandler should handle *string")
	}

	val, err := handler.ExtractValue(unsafe.Pointer(&nilPtr), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	if val != nil {
		t.Fatalf("expected nil, got %v", val)
	}
}

func TestPointerHandler_NonNilPointer(t *testing.T) {
	handler := &PointerHandler{}

	str := "test value"
	ptr := &str
	typ := reflect.TypeOf(ptr)

	if !handler.CanHandle(typ) {
		t.Fatalf("PointerHandler should handle *string")
	}

	val, err := handler.ExtractValue(unsafe.Pointer(&ptr), typ)
	if err != nil {
		t.Fatalf("ExtractValue failed: %v", err)
	}

	result, ok := val.(string)
	if !ok {
		t.Fatalf("expected string (delegated to PrimitiveHandler), got %T", val)
	}

	if result != str {
		t.Fatalf("expected '%v', got '%v'", str, result)
	}
}
