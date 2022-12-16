package complete

import (
	"fmt"
	"reflect"

	"github.com/canonical/sqlair/internal/assemble"
	"github.com/canonical/sqlair/internal/parse"
	"github.com/canonical/sqlair/internal/typeinfo"
)

type colInfo struct {
	columns    []string
	values     []any
	valuePtrs  []any
	colToIndex map[string]int
}

type CompletedExpr struct {
	Parsed      *parse.ParsedExpr
	InputValues []any
}

type typeNameToValue = map[string]any

func Complete(ae *assemble.AssembledExpr, args ...any) (*CompletedExpr, error) {
	var tv = make(typeNameToValue)
	for _, arg := range args {
		if arg == (any)(nil) {
			return nil, fmt.Errorf("cannot reflect nil value")
		}
		tv[reflect.TypeOf(arg).Name()] = arg
	}

	fvs := []any{}

	var ioparts int
	for _, part := range ae.Parsed.QueryParts {
		if p, ok := part.(*parse.InputPart); ok {
			if v, ok := tv[p.Source.Prefix]; ok {
				fv, err := typeinfo.FieldValue(v, p.Source)
				if err != nil {
					return nil, err
				}
				fvs = append(fvs, fv)
				ioparts++
			} else {
				return nil, fmt.Errorf("type %s not passed as a parameter", p.Source.Prefix)
			}
		}
	}

	if ioparts != len(args) {
		return nil, fmt.Errorf("parameters mismatch. expected %d, have %d", ioparts, len(args))
	}

	return &CompletedExpr{ae.Parsed, fvs}, nil
}
