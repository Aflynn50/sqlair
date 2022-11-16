package parse

import (
	"fmt"
	"strings"
)

type Parser struct {
	input string
	pos   int
	// prevPart is the value of pos when we last finished parsing a part.
	prevPart int
	// partStart is the value of pos just before we started parsing the part
	// under pos. We maintain partStart >= prevPart.
	partStart int
	parts     []queryPart
}

func NewParser() *Parser {
	return &Parser{}
}

// init resets the state of the parser and sets the input string.
func (p *Parser) init(input string) {
	p.input = input
	p.pos = 0
	p.prevPart = 0
	p.partStart = 0
	p.parts = []queryPart{}
}

// A checkpoint struct for saving parser state to restore later. We only use
// a checkpoint within an attempted parsing of an part, not at a higher level
// since we don't keep track of the parts in the checkpoint.
type checkpoint struct {
	parser    *Parser
	pos       int
	prevPart  int
	partStart int
	parts     []queryPart
}

// save takes a snapshot of the state of the parser and returns a pointer to a
// checkpoint that represents it.
func (p *Parser) save() *checkpoint {
	return &checkpoint{
		parser:    p,
		pos:       p.pos,
		prevPart:  p.prevPart,
		partStart: p.partStart,
		parts:     p.parts,
	}
}

// restore sets the internal state of the parser to the values stored in the
// checkpoint.
func (cp *checkpoint) restore() {
	cp.parser.pos = cp.pos
	cp.parser.prevPart = cp.prevPart
	cp.parser.partStart = cp.partStart
	cp.parser.parts = cp.parts
}

type idClass int

const (
	columnId idClass = iota
	typeId
)

type starFlag int

const (
	allowStar starFlag = iota
	disallowStar
)

// ParsedExpr is the AST representation of an SQL expression.
// It has a representation of the original SQL statement in terms of queryParts
// A SQL statement like this:
//
// Select p.* as &Person.* from person where p.name = $Boss.Name
//
// would be represented as:
//
// [BypassPart OutputPart BypassPart InputPart]
type ParsedExpr struct {
	queryParts []queryPart
}

// String returns a textual representation of the AST contained in the
// ParsedExpr for debugging purposes.
func (pe *ParsedExpr) String() string {
	out := "ParsedExpr["
	for i, p := range pe.queryParts {
		if i > 0 {
			out = out + " "
		}
		out = out + p.String()
	}
	out = out + "]"
	return out
}

// add pushes the parsed part to the parsedExprBuilder along with the BypassPart
// that stretches from the end of the previous part to the beginning of this
// part.
func (p *Parser) add(part queryPart) {
	// Add the string between the previous I/O part and the current part.
	if p.prevPart != p.partStart {
		p.parts = append(p.parts,
			&BypassPart{p.input[p.prevPart:p.partStart]})
	}

	if part != nil {
		p.parts = append(p.parts, part)
	}

	// Save this position at the end of the part.
	p.prevPart = p.pos
	// Ensure that partStart >= prevPart.
	p.partStart = p.pos
}

// Parse takes an input string and parses the input and output parts. It returns
// a pointer to a ParsedExpr.
func (p *Parser) Parse(input string) (*ParsedExpr, error) {
	p.init(input)

	for {
		p.partStart = p.pos

		if op, ok, err := p.parseOutputExpression(); err != nil {
			return nil, fmt.Errorf("parser error: %s", err)
		} else if ok {
			p.add(op)

		} else if ip, ok, err := p.parseInputExpression(); err != nil {
			return nil, fmt.Errorf("parser error: %s", err)
		} else if ok {
			p.add(ip)

		} else if sp, ok, err := p.parseStringLiteral(); err != nil {
			return nil, fmt.Errorf("parser error: %s", err)
		} else if ok {
			p.add(sp)

		} else if p.pos == len(p.input) {
			break
		} else {
			p.pos++
		}
	}
	// Add any remaining uparsed string input to the parser

	p.add(nil)
	return &ParsedExpr{p.parts}, nil
}

// peekByte returns true if the current byte equals the one passed as parameter.
func (p *Parser) peekByte(b byte) bool {
	return p.pos < len(p.input) && p.input[p.pos] == b
}

// skipByte jumps over the current byte if it matches the byte passed as a
// parameter. Returns true in that case, false otherwise.
func (p *Parser) skipByte(b byte) bool {
	if p.pos < len(p.input) && p.input[p.pos] == b {
		p.pos++
		return true
	}
	return false
}

// skipByteFind advances the parser until it finds a byte that matches the one
// passed as parameter and then jumps over it. In that case returns true. If the
// end of the string is reached and no matching byte was found, it returns
// false.
func (p *Parser) skipByteFind(b byte) bool {
	for i := p.pos; i < len(p.input); i++ {
		if p.input[i] == b {
			p.pos = i + 1
			return true
		}
	}
	return false
}

// skipSpaces advances the parser jumping over consecutive spaces. It stops when
// finding a non-space character. Returns true if the parser position was
// actually changed, false otherwise.
func (p *Parser) skipSpaces() bool {
	mark := p.pos
	for p.pos < len(p.input) {
		if p.input[p.pos] != ' ' {
			break
		}
		p.pos++
	}
	return p.pos != mark
}

// skipString advances the parser and jumps over the string passed as parameter.
// In that case returns true, false otherwise.
// This function is case insensitive.
func (p *Parser) skipString(s string) bool {
	if p.pos+len(s) <= len(p.input) &&
		strings.EqualFold(p.input[p.pos:p.pos+len(s)], s) {
		p.pos += len(s)
		return true
	}
	return false
}

// isNameByte returns true if the byte passed as parameter is considered to be
// one that can be part of a name. It returns false otherwise
func isNameByte(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' ||
		'0' <= c && c <= '9' || c == '_'
}

// These functions attempt to parse some construct, they return a bool and that
// construct, if they can't parse they return false, restore the parser and
// leave the default value in the other return type
func (p *Parser) parseIdentifier(starF starFlag) (string, bool) {
	if p.pos >= len(p.input) {
		return "", false
	}
	if starF == allowStar {
		if p.peekByte('*') {
			p.pos++
			return "*", true
		}
	}

	idStart := p.pos
	if !isNameByte(p.input[p.pos]) {
		return "", false
	}
	var i int
	for i = p.pos; i < len(p.input); i++ {
		if !isNameByte(p.input[i]) {
			break
		}
	}
	p.pos = i
	return p.input[idStart:i], true
}

// When parsing a column the table name (if extant) is in FullName.Prefix and
// the column name is in FullName.Name.
func (p *Parser) parseColumn() (FullName, bool) {
	cp := p.save()
	var fn FullName
	if id, ok := p.parseIdentifier(allowStar); ok {
		fn.Prefix = id
		if p.skipByte('.') {
			if id, ok := p.parseIdentifier(allowStar); ok {
				fn.Name = id
				return fn, true
			}
		} else {
			// A column name specified without a table prefix is a name not a
			// prefix
			fn.Name = fn.Prefix
			fn.Prefix = ""
			return fn, true
		}
	}
	cp.restore()
	return fn, false
}

// parseGoObject parses a source or target go object of the from Prefix.Name
// where the Type is the Prefix and the field is the Name (this applys to Maps
// and structs).
func (p *Parser) parseGoObject() (FullName, error) {
	var fn FullName
	if id, ok := p.parseIdentifier(disallowStar); ok {
		fn.Prefix = id
		if p.skipByte('.') {
			if id, ok := p.parseIdentifier(allowStar); ok {
				fn.Name = id
				return fn, nil
			} else {
				return fn, fmt.Errorf("not a valid identifier for a go object field")
			}
		} else {
			return fn, fmt.Errorf("go objects need to be qualified")
		}
	} else {
		return fn, fmt.Errorf("not a valid identifier for a go object")
	}
}

// parseColumns parses text in the SQL query of the form "table.colname". If
// there is more than one column then the columns must be bracketed together,
// e.g.  "(col1, col2) AS Person".
// We return:
//   - the list of columns
//   - if there is a star column
//   - whether columns were sucessfuly parsed
func (p *Parser) parseColumns() ([]FullName, bool, bool) {
	var cols []FullName
	var starPresent bool

	cp := p.save()
	// Case 1: A single column.
	if col, ok := p.parseColumn(); ok {
		if col.Name == "*" {
			starPresent = true
		}
		cols = append(cols, col)

		// Case 2: Multiple columns.
	} else if p.skipByte('(') {
		col, ok := p.parseColumn()
		// If the column names are not formated in a recognisable way then give
		// up trying to parse.
		if !ok {
			cp.restore()
			return cols, starPresent, false
		}
		if col.Name == "*" {
			starPresent = true
		}
		cols = append(cols, col)
		p.skipSpaces()
		for p.skipByte(',') {
			p.skipSpaces()
			col, ok := p.parseColumn()
			if !ok {
				cp.restore()
				return cols, starPresent, false
			}
			if col.Name == "*" {
				starPresent = true
			}
			cols = append(cols, col)
			p.skipSpaces()
		}
		p.skipSpaces()
		p.skipByte(')')
	}
	return cols, starPresent, true
}

// parseTargets has three return states
//  1. The input is not a list of targets (bool=false, err=nil)
//  2. We correctly parse the targets (bool=true, err=nil)
//  3. The input is a badly formed list of targets (bool=true, err=error)
func (p *Parser) parseTargets() ([]FullName, bool, error) {
	var targets []FullName
	cp := p.save()

	if p.skipByte('&') {
		if p.skipByte('(') {
			var starPresent = false
			target, err := p.parseGoObject()
			if err != nil {
				return targets, true, err
			}

			if target.Name == "*" {
				starPresent = true
			}
			targets = append(targets, target)
			p.skipSpaces()

			for p.skipByte(',') {
				p.skipSpaces()
				target, err := p.parseGoObject()
				if err != nil {
					return targets, true, err
				}
				if target.Name == "*" {
					if starPresent {
						return targets, true, fmt.Errorf("more than one asterisk")
					} else {
						starPresent = true
					}
				}
				targets = append(targets, target)
				p.skipSpaces()
			}
			if !p.skipByte(')') {
				return targets, true, fmt.Errorf("expected closing parentheses")
			}
		} else {
			target, err := p.parseGoObject()
			if err != nil {
				return targets, true, err
			}

			targets = append(targets, target)
		}
		return targets, true, nil
	}
	cp.restore()
	return targets, false, nil
}

// parseOutputExpression parses an SDL output holder to be filled with values
// from the executed query.
func (p *Parser) parseOutputExpression() (*OutputPart, bool, error) {
	cp := p.save()
	// Skip spaces to enforce correct spacing around IO
	p.skipSpaces()
	var cols []FullName

	if targets, ok, err := p.parseTargets(); ok {
		// Case 1: simple case with no columns e.g. &Person.*
		if err != nil {
			return nil, true, fmt.Errorf("output expression: %s", err)
		}
		p.skipSpaces()
		return &OutputPart{cols, targets}, true, nil

	} else if cols, starCol, ok := p.parseColumns(); ok {
		// Case 2: The expression contains an AS e.g. "p.col1 AS &Person.*".
		p.skipSpaces()
		if p.skipString("AS") {
			p.skipSpaces()
			if targets, ok, err := p.parseTargets(); ok {
				if err != nil {
					return nil, true, fmt.Errorf("output expression: %s", err)
				}

				// If the target is not * then check there are equal columns and
				// targets.
				if !(len(targets) == 1 && targets[0].Name == "*") {
					if len(cols) != len(targets) {
						return nil, true, fmt.Errorf("output expression: " +
							"number of cols != number of targets")
					}
				}

				// If the target is not M check that there are not mixed * and
				// regular columns.
				if targets[0].Prefix != "M" && len(cols) > 1 {
					if starCol {
						return nil, true, fmt.Errorf("output expression: " +
							"cannot mix asterisk and explicit columns")
					}
				}
				p.skipSpaces()
				return &OutputPart{cols, targets}, true, nil
			}
		}
	}
	cp.restore()
	return nil, false, nil
}

// parseInputExpression parses an SDL input go-defined type to be used as a
// query argument.
func (p *Parser) parseInputExpression() (*InputPart, bool, error) {
	cp := p.save()
	// Skip spaces to enforce correct spacing around IO
	p.skipSpaces()

	if p.skipByte('$') {
		fn, err := p.parseGoObject()
		if err != nil {
			return nil, true, fmt.Errorf("input expression: %s", err)
		}
		p.skipSpaces()
		return &InputPart{fn}, true, nil
	}
	cp.restore()
	return nil, false, nil
}

// parseInputExpression parses an SDL input go-defined type to be used as a
// query argument.
func (p *Parser) parseStringLiteral() (*BypassPart, bool, error) {
	cp := p.save()

	var err error

	if p.pos < len(p.input) {
		c := p.input[p.pos]
		if c == '"' || c == '\'' {
			p.skipByte(c)
			if !p.skipByteFind(c) {
				// Reached end of string and didn't find the closing quote
				err = fmt.Errorf("missing right quote in string literal")
			}
			return &BypassPart{p.input[cp.pos:p.pos]}, true, err
		}
	}

	cp.restore()
	return nil, false, err
}
