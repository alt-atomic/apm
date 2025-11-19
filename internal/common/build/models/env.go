package models

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
)

var placeholderRegexp = regexp.MustCompile(`\$\{\{\s*([A-Za-z0-9_\-.]+)\s*}}`)

func ResolveEnv(str string) (string, error) {
	data, err := resolvePlaceholders([]byte(str))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ResolveEnvSlice(strs []string) ([]string, error) {
	result := make([]string, len(strs))
	for i, str := range strs {
		resolved, err := ResolveEnv(str)
		if err != nil {
			return nil, err
		}
		result[i] = resolved
	}
	return result, nil
}

func ResolveEnvMap(strs map[string]string) (map[string]string, error) {
	var result = map[string]string{}
	for key, value := range strs {
		resolved, err := ResolveEnv(value)
		if err != nil {
			return nil, err
		}
		result[key] = resolved
	}
	return result, nil
}

func ResolveStruct(v any) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("ResolveStruct requires a pointer to struct")
	}

	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("ResolveStruct requires a pointer to struct")
	}

	return resolveStructValue(val)
}

func resolveStructValue(val reflect.Value) error {
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		if !field.CanSet() {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			original := field.String()
			resolved, err := ResolveEnv(original)
			if err != nil {
				return fmt.Errorf("failed to resolve field %s: %w", fieldType.Name, err)
			}
			field.SetString(resolved)

		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				original := make([]string, field.Len())
				for j := 0; j < field.Len(); j++ {
					original[j] = field.Index(j).String()
				}

				resolved, err := ResolveEnvSlice(original)
				if err != nil {
					return fmt.Errorf("failed to resolve field %s: %w", fieldType.Name, err)
				}

				newSlice := reflect.MakeSlice(field.Type(), len(resolved), len(resolved))
				for j, s := range resolved {
					newSlice.Index(j).SetString(s)
				}
				field.Set(newSlice)
			}

		case reflect.Map:
			if field.Type().Key().Kind() == reflect.String && field.Type().Elem().Kind() == reflect.String {
				newMap := reflect.MakeMap(field.Type())
				iter := field.MapRange()

				for iter.Next() {
					k := iter.Key().String()
					v := iter.Value().String()

					resolvedValue, err := ResolveEnv(v)
					if err != nil {
						return fmt.Errorf("failed to resolve map field %s[%s]: %w", fieldType.Name, k, err)
					}

					newMap.SetMapIndex(iter.Key(), reflect.ValueOf(resolvedValue))
				}

				field.Set(newMap)
			}

		case reflect.Struct:
			if err := resolveStructValue(field); err != nil {
				return err
			}

		case reflect.Ptr:
			if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
				if err := resolveStructValue(field.Elem()); err != nil {
					return err
				}
			}

		default:
			// Остальные типы (int, bool, float и т.д.) пропускаем
		}
	}

	return nil
}

func resolvePlaceholders(data []byte) ([]byte, error) {
	var firstErr error

	result := placeholderRegexp.ReplaceAllFunc(data, func(match []byte) []byte {
		if firstErr != nil {
			return match
		}

		submatches := placeholderRegexp.FindSubmatch(match)
		if len(submatches) != 2 {
			return match
		}

		rawKey := string(submatches[1])
		envKey, ok := extractEnvKey(rawKey)
		if !ok {
			firstErr = fmt.Errorf("unsupported placeholder %q; expected format ${ { Env.VAR } }", rawKey)
			return match
		}

		value, found := os.LookupEnv(envKey)
		if !found {
			firstErr = fmt.Errorf("environment variable %q is not set", envKey)
			return match
		}

		return []byte(value)
	})

	if firstErr != nil {
		return nil, firstErr
	}

	return result, nil
}

func extractEnvKey(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	// Проверяем префикс без учета регистра
	if !strings.HasPrefix(strings.ToLower(raw), "env.") {
		return "", false
	}

	key := raw[4:]
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}

	return key, true
}
