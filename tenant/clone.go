package tenant

import (
	"reflect"
	"regexp"
)

// Clone copies configuration defaults without sharing maps, slices or pointers.
// Synchronization primitives and private cache internals start empty.
func Clone[T any](value T) T {
	return cloneValue(reflect.ValueOf(&value).Elem()).Interface().(T)
}

func cloneValue(value reflect.Value) reflect.Value {
	if value.CanInterface() {
		if copier, ok := value.Interface().(interface{ CloneForTenant() any }); ok {
			return reflect.ValueOf(copier.CloneForTenant())
		}
	}
	// Compiled expressions are immutable and safe to share.
	if value.Type() == reflect.TypeFor[*regexp.Regexp]() {
		return value
	}
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copy := reflect.New(value.Type().Elem())
		copy.Elem().Set(cloneValue(value.Elem()))
		return copy
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copy := reflect.New(value.Type()).Elem()
		copy.Set(cloneValue(value.Elem()))
		return copy
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copy := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			copy.SetMapIndex(iter.Key(), cloneValue(iter.Value()))
		}
		return copy
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copy := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			copy.Index(i).Set(cloneValue(value.Index(i)))
		}
		return copy
	case reflect.Struct:
		copy := reflect.New(value.Type()).Elem()
		for i := range value.NumField() {
			if value.Type().Field(i).IsExported() {
				copy.Field(i).Set(cloneValue(value.Field(i)))
			}
		}
		return copy
	default:
		return value
	}
}
