package parse

import (
	"fmt"
	"reflect"
)

type PreparedExpr struct {
	parsed    *ParsedExpr
	inputArgs []any
	sql       string
}

// Gather the reflect information for an arguemnt, match it with the parameter
// in the query, and return the argument for passing to the db for execution
func (p *InputPart) matchWithArg(value any) (any, error) {

	v, err := GetReflectValue(value)
	if err != nil {
		return nil, err
	}

	switch v.Kind() {
	case reflect.Struct:
		typ := v.Type()
		if typ.Name() != p.Prefix {
			return nil, fmt.Errorf("name of parameter struct is %s but the argument struct has name %s", p.Prefix, typ.Name())
		}
		if p.Name == "" {
			return nil, fmt.Errorf("cannot use a struct as a parameter")
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			// Fields without a "db" tag are outside of Sqlair's remit.
			tag := field.Tag.Get("db")
			if tag == "" {
				continue
			}

			tag, omitEmpty, err := ParseTag(tag)
			if err != nil {
				return nil, err
			}
			// If the input parameters has a * use the first field with a tag
			if (p.Name == tag || p.Name == "*") && !omitEmpty {
				// Return the field as an interface of type any
				return v.Field(i).Interface(), nil
			}
		}
		return nil, fmt.Errorf("there is no tag with name %s in %s", p.Name, p.Prefix)
	case reflect.Map:
		// Also check that the argument is actually of our own type M
		if p.Name != "M" {
			return nil, fmt.Errorf("use sqlair type M use a map as an argument")
		}
		for _, key := range v.MapKeys() {
			// This has to be a string becuse M is a map: map[string]any
			if key.String() == p.Name {
				return v.MapIndex(key).Interface(), nil
			}
		}
		return nil, fmt.Errorf("key %s not in input map", p.Name)
	// default is its a variable
	default:
		return v.Interface(), nil
	}
}

// Prepare takes a parsed expression and checks that the input values provided
// by the user match the values specified in the query. It also gets the exact
// values needed for execution by going inside structs/maps and getting the
// relevant fields

// NOTE: we now have to access output struct fields by tag name
func (pe *ParsedExpr) Prepare(inputArgs ...any) (*PreparedExpr, error) {
	var i int
	var args []any

	// Match inputParts in SQL to arguments and generate args for exection
	for _, part := range pe.queryParts {
		if p, ok := part.(*InputPart); ok {
			if len(inputArgs) <= i {
				return nil, fmt.Errorf("not enough input values provided")
			}

			arg, err := p.matchWithArg(inputArgs[i])
			if err != nil {
				return nil, err
			}

			args = append(args, arg)
			i++
		}
	}
	if i < len(inputArgs) {
		return nil, fmt.Errorf("%v inputs in query but %v inputs provided", i, len(inputArgs))
	}

	// Generate SQL
	sql := ""
	for _, p := range pe.queryParts {
		sql = sql + p.ToSQL()
	}

	return &PreparedExpr{pe, args, sql}, nil
}
