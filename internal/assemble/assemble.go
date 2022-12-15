package assemble

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/sqlair/internal/parse"
	"github.com/canonical/sqlair/internal/typeinfo"
)

type AssembledExpr struct {
	Parsed *parse.ParsedExpr
	SQL    string
}

type typeNameToInfo map[string]*typeinfo.Info

func assembleInput(ti typeNameToInfo, p *parse.InputPart) error {

	if inf, ok := ti[p.Source.Prefix]; ok {
		if _, ok := inf.TagToField[p.Source.Name]; ok {
			return nil
		}
		return fmt.Errorf("there is no tag with name %s in %s",
			p.Source.Name, inf.Type.Name())
	}
	return fmt.Errorf("unknown type: %s", p.Source.Prefix)
}

type sourceField int

const (
	fromString sourceField = iota
	fromName
)

// createOutCols creates the list of output columns from the parameters passed
// in.
// The source and sf define the field to use from the FullName. If inf is not
// nil, then it is used to run some checks.
// Returns the array of columns and nil on success, nil and an error otherwise.
func createOutCols(source []parse.FullName, sf sourceField, inf *typeinfo.Info) ([]string, error) {
	var outCols []string
	for _, c := range source {
		if inf != nil {
			if _, ok := inf.TagToField[c.Name]; !ok {
				return nil, fmt.Errorf("there is no tag with name %s in %s",
					c.Name, inf.Type.Name())
			}
		}
		if sf == fromString {
			outCols = append(outCols, c.String())
		} else {
			outCols = append(outCols, c.Name)
		}
	}
	return outCols, nil
}

func assembleOutput(ti typeNameToInfo, p *parse.OutputPart) ([]string, error) {

	var outCols []string = make([]string, 0)

	// Check target type and tags are valid.
	for _, t := range p.Target {
		if inf, ok := ti[t.Prefix]; ok {
			if _, ok := inf.TagToField[t.Name]; !ok && t.Name != "*" {
				return nil, fmt.Errorf("there is no tag with name %s in %s",
					t.Name, inf.Type.Name())
			}
		} else {
			return nil, fmt.Errorf("unknown type: %s", t.Prefix)
		}
	}

	// Case 1: Star target cases e.g. "...&P.*".
	// In parse we ensure that if p.Target[0] is a * then len(p.Target) == 1
	if p.Target[0].Name == "*" {

		inf, _ := ti[p.Target[0].Prefix]

		// Case 1.1: Single star e.g. "t.* AS &P.*" or "&P.*"
		if (len(p.Source) > 0 && p.Source[0].Name == "*") ||
			len(p.Source) == 0 {
			pref := ""
			if len(p.Source) > 0 && p.Source[0].Prefix != "" {
				pref = p.Source[0].Prefix + "."
			}
			for tag := range inf.TagToField {
				outCols = append(outCols, pref+tag)
			}
			// The strings are sorted to give a deterministic order for
			// testing.
			sort.Strings(outCols)
			return outCols, nil
		}

		// Case 1.2: Explicit columns e.g. "(col1, t.col2) AS &P.*".
		if len(p.Source) > 0 {
			return createOutCols(p.Source, fromString, inf)
		}
	}

	// Case 2: None star target cases e.g. "...&(P.name, P.id)".

	// Case 2.1: Explicit columns e.g. "name_1 AS P.name".
	if len(p.Source) > 0 {
		return createOutCols(p.Source, fromString, nil)
	}

	// Case 2.2: No columns e.g. "&(P.name, P.id)".
	return createOutCols(p.Target, fromName, nil)
}

func Assemble(pe *parse.ParsedExpr, args ...any) (expr *AssembledExpr, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot assemble expression: %s", err)
		}
	}()

	var ti = make(typeNameToInfo)

	// Generate and save reflection info.
	for _, arg := range args {
		i, err := typeinfo.TypeInfo(arg)
		if err != nil {
			return nil, err
		}
		ti[i.Type.Name()] = i
	}

	var sql bytes.Buffer

	// Check and expand each query part.
	for _, part := range pe.QueryParts {
		if p, ok := part.(*parse.InputPart); ok {
			err := assembleInput(ti, p)
			if err != nil {
				return nil, err
			}
			sql.WriteString(p.ToSQL([]string{}))
			continue
		}

		if p, ok := part.(*parse.OutputPart); ok {
			outCols, err := assembleOutput(ti, p)
			if err != nil {
				return nil, err
			}
			sql.WriteString(p.ToSQL(outCols))
			continue
		}

		p := part.(*parse.BypassPart)
		sql.WriteString(p.ToSQL([]string{}))

	}

	sqlString := strings.TrimSpace(sql.String())
	// We will probably need to save the outcols and in cols.
	return &AssembledExpr{Parsed: pe, SQL: sqlString}, nil
}
