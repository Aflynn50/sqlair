package expr

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/canonical/sqlair/internal/typeinfo"
)

// TypedExprs represents a SQLair query bound to concrete Go types. It contains
// all the type information needed by SQLair.
type TypedExprs []typedExpression

// BindInputs takes the SQLair input arguments and returns the PrimedQuery ready
// for use with the database.
func (tes *TypedExprs) BindInputs(args ...any) (pq *PrimedQuery, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("invalid input parameter: %s", err)
		}
	}()

	var typeToValue = make(map[reflect.Type]reflect.Value)
	for _, arg := range args {
		v := reflect.ValueOf(arg)
		if v.Kind() == reflect.Invalid || (v.Kind() == reflect.Pointer && v.IsNil()) {
			return nil, fmt.Errorf("need struct or map, got nil")
		}
		v = reflect.Indirect(v)
		t := v.Type()
		if v.Kind() != reflect.Struct && v.Kind() != reflect.Map {
			return nil, fmt.Errorf("need struct or map, got %s", t.Kind())
		}
		if _, ok := typeToValue[t]; ok {
			return nil, fmt.Errorf("type %q provided more than once", t.Name())
		}
		typeToValue[t] = v
	}

	// Generate SQL and query parameters.
	params := []any{}
	outputs := []typeinfo.Member{}
	argTypeUsed := map[reflect.Type]bool{}
	inCount := 0
	outCount := 0
	sqlStr := bytes.Buffer{}
	for _, te := range *tes {
		switch te := te.(type) {
		case *typedInputExpr:
			typeMember := te.input
			outerType := typeMember.OuterType()
			v, ok := typeToValue[outerType]
			if !ok {
				return nil, inputMissingError(outerType, typeToValue)
			}
			argTypeUsed[outerType] = true

			val, err := typeMember.ValueFromOuter(v)
			if err != nil {
				return nil, err
			}
			params = append(params, sql.Named("sqlair_"+strconv.Itoa(inCount), val.Interface()))

			sqlStr.WriteString("@sqlair_" + strconv.Itoa(inCount))
			inCount++
		case *typedOutputExpr:
			for i, oc := range te.outputColumns {
				sqlStr.WriteString(oc.sql)
				sqlStr.WriteString(" AS ")
				sqlStr.WriteString(markerName(outCount))
				if i != len(te.outputColumns)-1 {
					sqlStr.WriteString(", ")
				}
				outCount++
				outputs = append(outputs, oc.tm)
			}
		case *bypass:
			sqlStr.WriteString(te.chunk)
		}
	}

	for argType := range typeToValue {
		if !argTypeUsed[argType] {
			return nil, fmt.Errorf("%s not referenced in query", argType.Name())
		}
	}

	return &PrimedQuery{outputs: outputs, sql: sqlStr.String(), params: params}, nil
}

// typedExpression represents a expression bound to a type. It contains
// information to generate the SQL for the part and to access Go types
// referenced in the part.
type typedExpression interface {
	// typedExpr is a marker method.
	typedExpr()
}

const markerPrefix = "_sqlair_"

func markerName(n int) string {
	return markerPrefix + strconv.Itoa(n)
}

// markerIndex returns the int X from the string "_sqlair_X".
func markerIndex(s string) (int, bool) {
	if strings.HasPrefix(s, markerPrefix) {
		n, err := strconv.Atoi(s[len(markerPrefix):])
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

// inputMissingError returns an error message for a missing input type.
func inputMissingError(missingType reflect.Type, typeToValue map[reflect.Type]reflect.Value) error {
	// check if the missing type and some argument type have the same name but
	// are from different packages.
	typeNames := []string{}
	for argType := range typeToValue {
		if argType.Name() == missingType.Name() {
			return fmt.Errorf("parameter with type %q missing, have type with same name: %q", missingType.String(), argType.String())
		}
		typeNames = append(typeNames, argType.Name())
	}
	return typeMissingError(missingType.Name(), typeNames)
}

func typeMissingError(missingType string, existingTypes []string) error {
	if len(existingTypes) == 0 {
		return fmt.Errorf(`parameter with type %q missing`, missingType)
	}
	// "%s" is used instead of %q to correctly print double quotes within the joined string.
	return fmt.Errorf(`parameter with type %q missing (have "%s")`, missingType, strings.Join(existingTypes, `", "`))
}
