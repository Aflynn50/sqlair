package assemble

import (
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

// assembleInput checks that the type in input expression is one we have
// reflected on and that the tag exists.
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

// makeOutCols builds an array of columns out of an array of tags.
// It admits an optional parameter that will be prefixed to the column name.
func makeOutCols(tags []string, prefix string) []string {
	var outCols []string = make([]string, 0)
	for _, tag := range tags {
		outCols = append(outCols, prefix+tag)
	}
	return outCols
}

func assembleOutput(ti typeNameToInfo, p *parse.OutputPart) ([]string, error) {

	var outCols []string = make([]string, 0)

	// In parse we ensure that if p.Target[0] is a * then len(p.Target) == 1
	// Case 1: Star target cases e.g. ...&P.*
	if p.Target[0].Name == "*" {
		var tags []string

		inf, ok := ti[p.Target[0].Prefix]
		if !ok {
			return nil, fmt.Errorf("unknown type: %s", p.Target[0].Prefix)
		}
		for tag := range inf.TagToField {
			tags = append(tags, tag)
		}

		if len(p.Source) > 0 { // Star with AS
			if p.Source[0].Name == "*" { // Single star column e.g. t.* AS &P.*
				pref := ""
				if p.Source[0].Prefix != "" {
					pref = p.Source[0].Prefix + "."
				}
				outCols = makeOutCols(tags, pref)
			} else { // Explicit columns e.g. (col1, col2) AS &P.*
				for _, c := range p.Source {
					if _, ok := inf.TagToField[c.Name]; !ok {
						return nil, fmt.Errorf("there is no tag with name %s in %s",
							c.Name, inf.Type.Name())
					}
					outCols = append(outCols, c.String())
				}
			}
		} else { // This is the case for star but no columns e.g. &P.*
			outCols = makeOutCols(tags, "")
		}
		// The strings are sorted to give a deterministic order for
		// testing.
		sort.Strings(outCols)
		return outCols, nil
	}

	// Case 2: None star target cases e.g. ...&(P.name, P.id)
	for _, t := range p.Target {
		if inf, ok := ti[t.Prefix]; ok {
			if _, ok := inf.TagToField[t.Name]; !ok {
				return nil, fmt.Errorf("there is no tag with name %s in %s",
					t.Name, inf.Type.Name())
			}
		} else {
			return nil, fmt.Errorf("unknown type: %s", t.Prefix)
		}
	}
	if len(p.Source) > 0 { // Explicit columns e.g. name_1 AS P.name
		for _, c := range p.Source {
			outCols = append(outCols, c.String())
		}
	} else { // No columns e.g. &(P.name, P.id)
		for _, t := range p.Target {
			outCols = append(outCols, t.Name)
		}
	}
	return outCols, nil
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

	sql := ""

	// Check and expand each query part
	for _, part := range pe.QueryParts {
		if p, ok := part.(*parse.InputPart); ok {
			err := assembleInput(ti, p)
			if err != nil {
				return nil, err
			}
			sql = sql + p.ToSQL([]string{})
			continue
		}

		if p, ok := part.(*parse.OutputPart); ok {
			outCols, err := assembleOutput(ti, p)
			if err != nil {
				return nil, err
			}
			sql = sql + p.ToSQL(outCols)
			continue
		}

		p := part.(*parse.BypassPart)
		sql = sql + p.ToSQL([]string{})

	}

	sql = strings.TrimSpace(sql)
	// We will probably need to save the outcols and in cols
	return &AssembledExpr{Parsed: pe, SQL: sql}, nil
}
