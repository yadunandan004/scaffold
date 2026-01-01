package singleton

import (
	"reflect"
	"sync"
)

type Builder[U any] interface {
	Build() U
}

// SingletonEntry holds a builder and ensures Build() is called only once
type SingletonEntry struct {
	builder  Builder[any]
	instance any
	once     sync.Once
}

var (
	singletons sync.Map
)

func Inject[B Builder[T], T any]() T {
	builderRef := new(B)
	builder := *builderRef
	builderType := reflect.TypeOf(builder)

	entryInterface, _ := singletons.LoadOrStore(builderType, &SingletonEntry{
		builder: &genericBuilderWrapper[T]{builder: builder},
	})
	entry := entryInterface.(*SingletonEntry)
	entry.once.Do(func() {
		entry.instance = entry.builder.Build()
	})
	return entry.instance.(T)
}

// GetInstance retrieves an already initialized singleton instance
func GetInstance[B Builder[T], T any]() T {
	builderRef := new(B)
	builder := *builderRef
	builderType := reflect.TypeOf(builder)

	entryInterface, ok := singletons.Load(builderType)
	if !ok {
		var zero T
		return zero
	}

	entry := entryInterface.(*SingletonEntry)
	if entry.instance == nil {
		var zero T
		return zero
	}

	return entry.instance.(T)
}

type genericBuilderWrapper[T any] struct {
	builder Builder[T]
}

func (w *genericBuilderWrapper[T]) Build() any {
	return w.builder.Build()
}
