package redis

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// toHash converts a struct into a map suitable for Redis HSet.
// Struct fields must have the `redis:"fieldname"` tag to be included.
// Returns alternating field names and values: map[field1] = value1, etc.
func toHash(v interface{}) map[string]interface{} {
	val := reflect.Indirect(reflect.ValueOf(v))
	typ := val.Type()

	fields := make(map[string]interface{})
	for i := 0; i < val.NumField(); i++ {
		tagVal := typ.Field(i).Tag.Get("redis")
		if tagVal == "" || tagVal == "-" {
			continue
		}

		fieldValue := val.Field(i).Interface()
		fields[tagVal] = fmt.Sprintf("%v", fieldValue)
	}

	return fields
}

// fromHash converts a Redis hash (map[string]string) to a struct.
// Struct fields must have the `redis:"fieldname"` tag to be populated.
// If a field cannot be parsed, Go's zero value is used.
//
// Supported types: string, int, int32, int64, uint32, uint64, float32, float64, bool, time.Duration
func fromHash(m map[string]string, out interface{}) error {
	val := reflect.ValueOf(out).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if !field.CanSet() {
			continue
		}

		tagVal := typ.Field(i).Tag.Get("redis")
		if tagVal == "" || tagVal == "-" {
			continue
		}

		v, ok := m[tagVal]
		if !ok {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(v)

		case reflect.Int:
			val, err := strconv.ParseInt(v, 10, 0)
			if err != nil {
				val = 0
			}
			field.SetInt(val)

		case reflect.Int32:
			val, err := strconv.ParseInt(v, 10, 32)
			if err != nil {
				val = 0
			}
			field.SetInt(val)

		case reflect.Int64:
			// Duration is recognized as an int64
			if field.Type() == reflect.ValueOf(time.Duration(0)).Type() {
				val, err := time.ParseDuration(v)
				if err != nil {
					val = 0
				}
				field.Set(reflect.ValueOf(val))
			} else {
				val, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					val = 0
				}
				field.SetInt(val)
			}

		case reflect.Uint32:
			val, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				val = 0
			}
			field.SetUint(val)

		case reflect.Uint64:
			val, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				val = 0
			}
			field.SetUint(val)

		case reflect.Float32:
			val, err := strconv.ParseFloat(v, 32)
			if err != nil {
				val = 0
			}
			field.SetFloat(val)

		case reflect.Float64:
			val, err := strconv.ParseFloat(v, 64)
			if err != nil {
				val = 0
			}
			field.SetFloat(val)

		case reflect.Bool:
			val, err := strconv.ParseBool(v)
			if err != nil {
				val = false
			}
			field.SetBool(val)
		}
	}

	return nil
}
