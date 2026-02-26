/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package ybaeon

import (
	"reflect"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// sanitizePayload recursively walks an arbitrary value (struct, map, slice, etc.)
// and strips HTML tags from all string fields. Returns a sanitized copy without
// modifying the original. This prevents WAF rejections when sending payloads to
// YB-Aeon API endpoints that sit behind firewalls blocking HTML content.
func sanitizePayload(payload interface{}) interface{} {
	if payload == nil {
		return nil
	}
	return sanitizeValue(reflect.ValueOf(payload)).Interface()
}

func sanitizeValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.String:
		return reflect.ValueOf(utils.StripAnchorTags(v.String()))

	case reflect.Ptr:
		if v.IsNil() {
			return v
		}
		sanitized := sanitizeValue(v.Elem())
		ptr := reflect.New(sanitized.Type())
		ptr.Elem().Set(sanitized)
		return ptr

	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		return sanitizeValue(v.Elem())

	case reflect.Struct:
		result := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			resultField := result.Field(i)
			if !field.CanInterface() {
				// Unexported field -- skip; irrelevant for JSON serialization
				continue
			}
			resultField.Set(sanitizeValue(field))
		}
		return result

	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		result := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
		for i := 0; i < v.Len(); i++ {
			result.Index(i).Set(sanitizeValue(v.Index(i)))
		}
		return result

	case reflect.Map:
		if v.IsNil() {
			return v
		}
		result := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			key := sanitizeValue(iter.Key())
			val := sanitizeValue(iter.Value())
			result.SetMapIndex(key, val)
		}
		return result

	default:
		// int, bool, float, etc. -- return as-is
		return v
	}
}
