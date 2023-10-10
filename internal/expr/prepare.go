package expr

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// PreparedExpr contains an SQL expression that is ready for execution.
type PreparedExpr struct {
	outputs    []typeMember
	inputs     []typeMember
	queryParts []queryPart
	outputCols map[*outputPart][]columnAccessor
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

// getKeys returns the keys of a string map in a deterministic order.
func getKeys[T any](m map[string]T) []string {
	i := 0
	keys := make([]string, len(m))
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

// starCountColumns counts the number of asterisks in a list of columns.
func starCountColumns(cs []columnAccessor) int {
	s := 0
	for _, c := range cs {
		if c.columnName == "*" {
			s++
		}
	}
	return s
}

// starCountTypes counts the number of asterisks in a list of types.
func starCountTypes(vs []valueAccessor) int {
	s := 0
	for _, v := range vs {
		if v.memberName == "*" {
			s++
		}
	}
	return s
}

func typeMissingError(missingType string, existingTypes []string) error {
	if len(existingTypes) == 0 {
		return fmt.Errorf(`parameter with type %q missing`, missingType)
	}
	// "%s" is used instead of %q to correctly print double quotes within the joined string.
	return fmt.Errorf(`parameter with type %q missing (have "%s")`, missingType, strings.Join(existingTypes, `", "`))
}

// prepareInput checks that the input expression corresponds to a known type.
func prepareInput(ti typeNameToInfo, p *inputPart) (tm typeMember, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("input expression: %s: %s", err, p.raw)
		}
	}()

	info, ok := ti[p.sourceType.typeName]
	if !ok {
		return nil, typeMissingError(p.sourceType.typeName, getKeys(ti))
	}
	if p.isSlice {
		switch info := info.(type) {
		case *structInfo, *mapInfo:
			return nil, fmt.Errorf(`cannot use slice syntax with %s`, info.typ().Kind())
		case *sliceInfo:
			tm = info.getSliceType()
		default:
			return nil, fmt.Errorf(`internal error: unknown type: %T`, info)
		}
	} else {
		tm, err = info.typeMember(p.sourceType.memberName)
		if err != nil {
			return nil, err
		}
	}
	return tm, nil
}

// prepareOutput checks that the output expressions correspond to known types.
// It then checks they are formatted correctly and finally generates the columns for the query.
func prepareOutput(ti typeNameToInfo, p *outputPart) (outCols []columnAccessor, typeMembers []typeMember, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("output expression: %s: %s", err, p.raw)
		}
	}()

	numTypes := len(p.targetTypes)
	numColumns := len(p.sourceColumns)
	starTypes := starCountTypes(p.targetTypes)
	starColumns := starCountColumns(p.sourceColumns)

	// Case 1: Generated columns e.g. "* AS (&P.*, &A.id)" or "&P.*".
	if numColumns == 0 || (numColumns == 1 && starColumns == 1) {
		pref := ""
		// Prepend table name. E.g. "t" in "t.* AS &P.*".
		if numColumns > 0 {
			pref = p.sourceColumns[0].tableName
		}

		for _, t := range p.targetTypes {
			info, ok := ti[t.typeName]
			if !ok {
				return nil, nil, typeMissingError(t.typeName, getKeys(ti))
			}
			if t.memberName == "*" {
				// Generate asterisk columns.
				allMembers, err := info.getAllMembers()
				if err != nil {
					return nil, nil, err
				}
				typeMembers = append(typeMembers, allMembers...)
				for _, tm := range allMembers {
					outCols = append(outCols, columnAccessor{pref, tm.memberName()})
				}
			} else {
				// Generate explicit columns.
				tm, err := info.typeMember(t.memberName)
				if err != nil {
					return nil, nil, err
				}
				typeMembers = append(typeMembers, tm)
				outCols = append(outCols, columnAccessor{pref, t.memberName})
			}
		}
		return outCols, typeMembers, nil
	} else if numColumns > 1 && starColumns > 0 {
		return nil, nil, fmt.Errorf("invalid asterisk in columns")
	}

	// Case 2: Explicit columns, single asterisk type e.g. "(col1, t.col2) AS &P.*".
	if starTypes == 1 && numTypes == 1 {
		info, ok := ti[p.targetTypes[0].typeName]
		if !ok {
			return nil, nil, typeMissingError(p.targetTypes[0].typeName, getKeys(ti))
		}
		for _, c := range p.sourceColumns {
			tm, err := info.typeMember(c.columnName)
			if err != nil {
				return nil, nil, err
			}
			typeMembers = append(typeMembers, tm)
			outCols = append(outCols, c)
		}
		return outCols, typeMembers, nil
	} else if starTypes > 0 && numTypes > 1 {
		return nil, nil, fmt.Errorf("invalid asterisk in types")
	}

	// Case 3: Explicit columns and types e.g. "(col1, col2) AS (&P.name, &P.id)".
	if numColumns == numTypes {
		for i, c := range p.sourceColumns {
			t := p.targetTypes[i]
			info, ok := ti[t.typeName]
			if !ok {
				return nil, nil, typeMissingError(t.typeName, getKeys(ti))
			}
			tm, err := info.typeMember(t.memberName)
			if err != nil {
				return nil, nil, err
			}
			typeMembers = append(typeMembers, tm)
			outCols = append(outCols, c)
		}
	} else {
		return nil, nil, fmt.Errorf("mismatched number of columns and target types")
	}

	return outCols, typeMembers, nil
}

type typeNameToInfo map[string]typeInfo

// Prepare takes a parsed expression and struct instantiations of all the types
// mentioned in it.
// The IO parts of the statement are checked for validity against the types
// and expanded if necessary.
func (pe *ParsedExpr) Prepare(args ...any) (expr *PreparedExpr, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot prepare statement: %s", err)
		}
	}()

	var ti = make(typeNameToInfo)

	// Generate and save reflection info.
	for _, arg := range args {
		if arg == nil {
			return nil, fmt.Errorf("need valid value, got nil")
		}
		t := reflect.TypeOf(arg)
		switch t.Kind() {
		case reflect.Struct, reflect.Map, reflect.Slice:
			if t.Name() == "" {
				return nil, fmt.Errorf("cannot use anonymous %s", t.Kind())
			}
			info, err := getTypeInfo(arg)
			if err != nil {
				return nil, err
			}
			if dupeInfo, ok := ti[t.Name()]; ok {
				if dupeInfo.typ() == t {
					return nil, fmt.Errorf("found multiple instances of type %q", t.Name())
				}
				return nil, fmt.Errorf("two types found with name %q: %q and %q", t.Name(), dupeInfo.typ().String(), t.String())
			}
			ti[t.Name()] = info
		case reflect.Pointer:
			return nil, fmt.Errorf("unsupported type: pointer to %s", t.Elem().Kind())
		default:
			return nil, fmt.Errorf("unsupported type: %s", t.Kind())
		}
	}

	var outputs = make([]typeMember, 0)
	var inputs = make([]typeMember, 0)
	var typeMemberPresent = make(map[typeMember]bool)
	var outputCols = make(map[*outputPart][]columnAccessor, 0)

	// Check and expand each query part.
	for _, part := range pe.queryParts {
		switch p := part.(type) {
		case *inputPart:
			tm, err := prepareInput(ti, p)
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, tm)
		case *outputPart:
			outCols, typeMembers, err := prepareOutput(ti, p)
			if err != nil {
				return nil, err
			}

			for _, tm := range typeMembers {
				if ok := typeMemberPresent[tm]; ok {
					return nil, fmt.Errorf("member %q of type %q appears more than once in output expressions", tm.memberName(), tm.outerType().Name())
				}
				typeMemberPresent[tm] = true
			}
			outputs = append(outputs, typeMembers...)
			outputCols[p] = outCols
		case *bypassPart:
		default:
			return nil, fmt.Errorf("internal error: unknown query part type %T", part)
		}
	}
	return &PreparedExpr{inputs: inputs, outputs: outputs, queryParts: pe.queryParts, outputCols: outputCols}, nil
}

// stmtCriterion contains information that differentiates the SQL strings that
// can be generated from a single SQLair prepared statement.
type stmtCriterion struct {
	sliceLens []int
}

func (pe *PreparedExpr) sql(sc *stmtCriterion) string {
	var sql bytes.Buffer
	var inCount int
	var outCount int
	var sliceCount int

	for _, part := range pe.queryParts {
		switch p := part.(type) {
		case *inputPart:
			if p.isSlice {
				if sliceCount >= len(sc.sliceLens) {
					panic("internal error: slice information not found")
				}
				for i := 0; i < sc.sliceLens[sliceCount]; i++ {
					sql.WriteString("@sqlair_")
					sql.WriteString(strconv.Itoa(inCount))
					if i < sc.sliceLens[sliceCount]-1 {
						sql.WriteString(", ")
					}
					inCount++
				}
				sliceCount++
			} else {
				sql.WriteString("@sqlair_")
				sql.WriteString(strconv.Itoa(inCount))
				inCount++
			}
		case *outputPart:
			for i, c := range pe.outputCols[p] {
				sql.WriteString(c.String())
				sql.WriteString(" AS ")
				sql.WriteString(markerName(outCount))
				if i != len(pe.outputCols[p])-1 {
					sql.WriteString(", ")
				}
				outCount++
			}
		case *bypassPart:
			sql.WriteString(p.chunk)
		default:
			panic(fmt.Sprintf("internal error: unknown query part type %T", part))
		}
	}
	return sql.String()
}
