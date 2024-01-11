package expr

import (
	"bytes"
	"fmt"

	"github.com/canonical/sqlair/internal/typeinfo"
)

// ParsedExpr is the AST representation of SQLair query. It contains only
// information encoded in the SQLair query string.
type ParsedExpr struct {
	exprs []expression
}

// String returns a textual representation of the AST contained in the
// ParsedExpr for debugging and testing purposes.
func (pe *ParsedExpr) String() string {
	var out bytes.Buffer
	out.WriteString("[")
	for i, p := range pe.exprs {
		if i > 0 {
			out.WriteString(" ")
		}
		out.WriteString(p.String())
	}
	out.WriteString("]")
	return out.String()
}

// BindTypes takes samples of all types mentioned in the SQLair expressions of
// the query. The expressions are checked for validity and required information
// is generated from the types.
func (pe *ParsedExpr) BindTypes(args ...any) (tbe *TypeBoundExpr, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot prepare statement: %s", err)
		}
	}()

	argInfo, err := typeinfo.GenerateArgInfo(args)
	if err != nil {
		return nil, err
	}

	// Bind types to each expression.
	var typedExprs TypeBoundExpr
	outputUsed := map[typeinfo.Output]bool{}
	var te any
	for _, expr := range pe.exprs {
		switch e := expr.(type) {
		case *inputExpr:
			te, err = bindInputTypes(e, argInfo)
			if err != nil {
				return nil, err
			}
		case *outputExpr:
			toe, err := bindOutputTypes(e, argInfo)
			if err != nil {
				return nil, err
			}

			for _, oc := range toe.outputColumns {
				if ok := outputUsed[oc.output]; ok {
					return nil, fmt.Errorf("%s appears more than once in output expressions", oc.output.String())
				}
				outputUsed[oc.output] = true
			}
			te = toe
		case *bypass:
			te = e
		default:
			return nil, fmt.Errorf("internal error: unknown query expr type %T", expr)
		}
		typedExprs = append(typedExprs, te)
	}

	return &typedExprs, nil
}

// bindInputTypes binds the input expression to a query type and returns a typed
// input expression.
func bindInputTypes(e *inputExpr, argInfo typeinfo.ArgInfo) (te *typedInputExpr, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("input expression: %s: %s", err, e.raw)
		}
	}()

	var input typeinfo.Input
	switch a := e.sourceType.(type) {
	case *memberAccessor:
		input, err = argInfo.InputMember(a.typeName, a.memberName)
		if err != nil {
			return nil, err
		}
	case *sliceAccessor:
		return nil, fmt.Errorf("slice support not implemented")
	}
	return &typedInputExpr{input}, nil
}

// bindOutputTypes binds the output expression to concrete types. It then checks the
// expression valid with respect to its bound types and generates a typed output
// expression.
func bindOutputTypes(e *outputExpr, argInfo typeinfo.ArgInfo) (te *typedOutputExpr, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("output expression: %s: %s", err, e.raw)
		}
	}()

	// Prepend table name. E.g. "t" in "t.* AS &P.*".
	toe := &typedOutputExpr{}
	switch ca := e.sourceColumns.(type) {
	// Case 1: Generated columns e.g. "* AS (&P.*, &A.id)" or "&P.*".
	case nil, *wildcardColumn:
		tablePrefix := ""
		if wc, ok := ca.(*wildcardColumn); ok {
			tablePrefix = wc.tableName
		}
		for _, va := range e.targetTypes {
			switch va := va.(type) {
			case *allMembersAccessor:
				// Generate asterisk columns.
				members, memberNames, err := argInfo.AllStructOutputs(va.typeName)
				if err != nil {
					return nil, err
				}
				for i, member := range members {
					oc := newOutputColumn(tablePrefix, memberNames[i], member)
					toe.outputColumns = append(toe.outputColumns, oc)
				}
			case *memberAccessor:
				// Generate explicit columns.
				member, err := argInfo.OutputMember(va.typeName, va.memberName)
				if err != nil {
					return nil, err
				}
				oc := newOutputColumn(tablePrefix, va.memberName, member)
				toe.outputColumns = append(toe.outputColumns, oc)
			default:
				return nil, fmt.Errorf("internal error: unknown type %T", va)
			}
		}
		return toe, nil
	case standardColumns:
		for i, va := range e.targetTypes {
			switch va := va.(type) {
			case *allMembersAccessor:
				// Case 2: Explicit columns, single asterisk type
				// e.g. "(col1, t.col2) AS &P.*".
				if len(e.targetTypes) != 1 {
					return nil, fmt.Errorf("invalid asterisk in types")
				}
				for _, c := range ca {
					member, err := argInfo.OutputMember(va.typeName, c.columnName)
					if err != nil {
						return nil, err
					}
					oc := newOutputColumn(c.tableName, c.columnName, member)
					toe.outputColumns = append(toe.outputColumns, oc)
				}
				return toe, nil
			case *memberAccessor:
				// Case 3: Explicit columns and types
				// e.g. "(col1, col2) AS (&P.name, &P.id)".
				if len(ca) != len(e.targetTypes) {
					return nil, fmt.Errorf("mismatched number of columns and target types")
				}
				member, err := argInfo.OutputMember(va.typeName, va.memberName)
				if err != nil {
					return nil, err
				}
				oc := newOutputColumn(ca[i].tableName, ca[i].columnName, member)
				toe.outputColumns = append(toe.outputColumns, oc)
			default:
				return nil, fmt.Errorf("internal error: unknown type %T", va)
			}
		}
		return toe, nil
	default:
		return nil, fmt.Errorf("internal error: unknown type %T", ca)
	}
	return nil, fmt.Errorf("internal error: unreachable")
}
