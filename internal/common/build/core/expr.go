package core

import (
	"apm/internal/common/app"
	"apm/internal/common/version"
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/expr-lang/expr"
)

type MapModule struct {
	Name   string
	Type   string
	Id     string
	If     bool
	Output map[string]string
}

type ExprData struct {
	Modules map[string]MapModule
	Env     map[string]string
	Version version.Version
}

var placeholderRegexp = regexp.MustCompile(`\$\{\{\s*([A-Za-z0-9_\-.]+)\s*}}`)

func ResolveExpr(str string, exprData any) (string, error) {
	data, err := resolvePlaceholders([]byte(str), exprData)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ResolveExprSlice(strs []string, data any) ([]string, error) {
	result := make([]string, len(strs))
	for i, str := range strs {
		resolved, err := ResolveExpr(str, data)
		if err != nil {
			return nil, err
		}
		result[i] = resolved
	}
	return result, nil
}

func ResolveExprMap(strs map[string]string, data any) (map[string]string, error) {
	var result = map[string]string{}
	for key, value := range strs {
		resolved, err := ResolveExpr(value, data)
		if err != nil {
			return nil, err
		}
		result[key] = resolved
	}
	return result, nil
}

func ResolveStruct(v any, data ExprData) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("ResolveStruct requires a pointer to struct")
	}

	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("ResolveStruct requires a pointer to struct")
	}

	return resolveStructValue(val, data)
}

func resolveStructValue(val reflect.Value, data ExprData) error {
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
			resolved, err := ResolveExpr(original, data)
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

				resolved, err := ResolveExprSlice(original, data)
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

					resolvedValue, err := ResolveExpr(v, data)
					if err != nil {
						return fmt.Errorf("failed to resolve map field %s[%s]: %w", fieldType.Name, k, err)
					}

					newMap.SetMapIndex(iter.Key(), reflect.ValueOf(resolvedValue))
				}

				field.Set(newMap)
			}

		case reflect.Struct:
			if err := resolveStructValue(field, data); err != nil {
				return err
			}

		case reflect.Ptr:
			if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
				if err := resolveStructValue(field.Elem(), data); err != nil {
					return err
				}
			}

		default:
			// Остальные типы (int, bool, float и т.д.) пропускаем
		}
	}

	return nil
}

func resolvePlaceholders(data []byte, exprData any) ([]byte, error) {
	var firstErr error

	result := placeholderRegexp.ReplaceAllFunc(data, func(match []byte) []byte {
		if firstErr != nil {
			return match
		}

		submatches := placeholderRegexp.FindSubmatch(match)
		if len(submatches) != 2 {
			return match
		}

		expression := string(submatches[1])
		output, err := ExtractExprResult(expression, exprData)
		if err != nil {
			firstErr = err
			return match
		}

		return []byte(output)
	})

	if firstErr != nil {
		return nil, firstErr
	}

	return result, nil
}

func ExtractExprResult(raw string, data any) (string, error) {
	program, err := expr.Compile(raw, expr.Env(data))
	if err != nil {
		return "", err
	}

	output, err := expr.Run(program, data)
	if err != nil {
		return "", err
	}

	switch output.(type) {
	case int, int64, float32, float64, bool, string, []string:
		return fmt.Sprintf("%v", output), nil
	default:
		return "", errors.New(app.T_("unknown expr output type"))
	}
}

func ExtractExprResultBool(raw string, data ExprData) (bool, error) {
	program, err := expr.Compile(raw, expr.Env(data), expr.AsBool())
	if err != nil {
		return false, err
	}

	output, err := expr.Run(program, data)
	if err != nil {
		return false, err
	}

	return output.(bool), nil
}
