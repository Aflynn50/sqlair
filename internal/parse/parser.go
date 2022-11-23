package parse

import (
	"bytes"
	"fmt"
	"strings"
)

// Parser keeps track of the current parsing state.
type Parser struct {
	input string
	pos   int
	// prevPart is the value of pos when we last finished parsing a part.
	prevPart int
	// partStart is the value of pos just before we started parsing the part
	// under pos. We maintain partStart >= prevPart.
	partStart int
	parts     []QueryPart
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
	p.parts = []QueryPart{}
}

// A checkpoint struct for saving parser state to restore later. We only use
// a checkpoint within an attempted parsing of an part, not at a higher level
// since we don't keep track of the parts in the checkpoint.
type checkpoint struct {
	parser    *Parser
	pos       int
	prevPart  int
	partStart int
	parts     []QueryPart
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

type starFlag int

const (
	allowStar starFlag = iota
	disallowStar
)

// ParsedExpr is the AST representation of an SQL expression.
// It has a representation of the original SQL statement in terms of QueryParts
// A SQL statement like this:
//
// Select p.* as &Person.* from person where p.name = $Boss.col_name
//
// would be represented as:
//
// [BypassPart OutputPart BypassPart InputPart]
type ParsedExpr struct {
	QueryParts []QueryPart
}

// String returns a textual representation of the AST contained in the
// ParsedExpr for debugging purposes.
func (pe *ParsedExpr) String() string {
	var out bytes.Buffer
	out.WriteString("ParsedExpr[")
	for i, p := range pe.QueryParts {
		if i > 0 {
			out.WriteString(" ")
		}
		out.WriteString(p.String())
	}
	out.WriteString("]")
	return out.String()
}

// add pushes the parsed part to the parsedExprBuilder along with the BypassPart
// that stretches from the end of the previous part to the beginning of this
// part.
func (p *Parser) add(part QueryPart) {
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
// a pointer to a ParsedExpr. If the parser encounters an error then ParsedExpr
// is nil.
func (p *Parser) Parse(input string) (expr *ParsedExpr, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot parse expression: %s", err)
		}
	}()
	p.init(input)
	var parsed bool
	for {
		p.partStart = p.pos
		parsed = false

		if op, ok, err := p.parseOutputExpression(); err != nil {
			return nil, err
		} else if ok {
			p.add(op)
			parsed = true
		}

		if ip, ok, err := p.parseInputExpression(); err != nil {
			return nil, err
		} else if ok {
			p.add(ip)
			parsed = true
		}

		if sp, ok, err := p.parseStringLiteral(); err != nil {
			return nil, err
		} else if ok {
			p.add(sp)
			parsed = true
		}

		if p.pos == len(p.input) {
			break
		} else if !parsed {
			p.advance()
		}
	}
	// Add any remaining unparsed string input to the parser.
	p.add(nil)
	return &ParsedExpr{p.parts}, nil
}

// advance increments p.pos until we reach a space, a $, a " or a '. If we find
// a space we advance to the last adjacent space so the p.pos points to the last
// space before the next non-space character.
func (p *Parser) advance() {
	noteableBytes := map[byte]bool{
		'$':  true,
		'"':  true,
		'\'': true,
		' ':  true,
	}
	p.pos++
	for p.pos < len(p.input) && !noteableBytes[p.input[p.pos]] {
		p.pos++
	}
	if p.peekByte(' ') {
		for p.pos+1 < len(p.input) && p.input[p.pos+1] == ' ' {
			p.pos++
		}
	}
	return
}

// peekByte returns true if the current byte equals the one passed as parameter.
func (p *Parser) peekByte(b byte) bool {
	return p.pos < len(p.input) && p.input[p.pos] == b
}

// Functions with the prefix 'skip' always return a single bool. If they return
// false the parser state is unchanged. If they return true they move p.pos to
// the char after the pattern they have skipped.

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

// skipName advances the parser until it is on the first non name byte and
// returns true. If the p.pos does not start on a name byte it returns false.
func (p *Parser) skipName() bool {
	if p.pos >= len(p.input) {
		return false
	}
	mark := p.pos
	for p.pos < len(p.input) && isNameByte(p.input[p.pos]) {
		p.pos++
	}
	return p.pos > mark
}

// Functions with the prefix 'parse' attempt to parse some construct. They return
// the construct, a 'ok' bool and optionally an error.
//
// Return cases:
//  - bool == true (err == nil)
//		The construct was successfully parsed.
//  - bool == false (err == nil)
//		The construct was not the one we are looking for.
//  - bool == false, err != nil
//		The construct was recognised but was not correctly formatted.

// parseIdentifier parses either a name made up only of nameBytes or an
// asterisk.
func (p *Parser) parseIdentifier(starF starFlag) (string, bool) {
	if starF == allowStar {
		if p.skipByte('*') {
			return "*", true
		}
	}

	idStart := p.pos
	if p.skipName() {
		return p.input[idStart:p.pos], true
	}
	return "", false
}

// parseColumn parses a column made up of name bytes, optionally dot-prefixed by
// its table name.
func (p *Parser) parseColumn() (FullName, bool) {
	cp := p.save()
	var fn FullName
	if id, ok := p.parseIdentifier(allowStar); ok {
		if p.skipByte('.') {
			if idCol, ok := p.parseIdentifier(allowStar); ok {
				return FullName{Prefix: id, Name: idCol}, true
			}
		} else {
			// A column name specified without a table prefix should be in Name.
			fn.Name = id
			return fn, true
		}
	}
	cp.restore()
	return fn, false
}

// parseGoObject parses a source or target go object of the form Prefix.Name
// where the type is the Prefix and the field is the Name (this applies to maps
// and structs).
func (p *Parser) parseGoObject() (FullName, bool, error) {
	cp := p.save()
	var fn FullName
	if id, ok := p.parseIdentifier(disallowStar); ok {
		if p.skipByte('.') {
			if idField, ok := p.parseIdentifier(allowStar); ok {
				return FullName{Prefix: id, Name: idField}, true, nil
			} else {
				return fn, false, fmt.Errorf("not a valid identifier for a go object field")
			}
		} else {
			return fn, false, fmt.Errorf("go objects need to be qualified")
		}
	}
	cp.restore()
	return fn, false, nil
}

// parseColumns parses text in the SQL query of the form "table.colname". If
// there is more than one column then the columns must be enclosed in brackets
// e.g.  "(col1, col2) AS &Person.*".
func (p *Parser) parseColumns() ([]FullName, bool) {
	cp := p.save()
	var cols []FullName
	// We skip a space here to keep consistent with parseTargets which also
	// consumes one space before the start of the expression.
	p.skipByte(' ')
	// Case 1: A single column e.g. p.name
	if col, ok := p.parseColumn(); ok {
		return []FullName{col}, true
	} else if p.skipByte('(') {
		// Case 2: Multiple columns e.g. (p.name, p.id, q.*)
		if col, ok := p.parseColumn(); ok {
			cols = append(cols, col)
			p.skipSpaces()
			for p.skipByte(',') {
				p.skipSpaces()
				if col, ok := p.parseColumn(); ok {
					cols = append(cols, col)
				} else {
					cp.restore()
					return cols, false
				}
				p.skipSpaces()
			}
			if p.skipByte(')') {
				return cols, true
			}
		}
	}
	cp.restore()
	return cols, false
}

// parseTargets parses the part of the output expression following the
// ampersand. This can be one or more Go objects. If the ampersand is not found
// or is not preceded by a space and succeeded by a name or opening bracket the
// targets are not parsed.
func (p *Parser) parseTargets() ([]FullName, bool, error) {
	cp := p.save()
	var targets []FullName

	// An '&' must be preceded by a space and succeeded by a name or opening
	// bracket.
	if p.skipString(" &") {
		// Case 1: A single target e.g. &Person.name
		if target, ok, err := p.parseGoObject(); ok {
			return []FullName{target}, true, nil
		} else if err != nil {
			return targets, false, err
			// Case 2: Multiple targets e.g. &(Person.name, Person.id)
		} else if p.skipByte('(') {
			if target, ok, err := p.parseGoObject(); ok {
				targets = append(targets, target)
				p.skipSpaces()
				for p.skipByte(',') {
					p.skipSpaces()
					if target, ok, err := p.parseGoObject(); ok {
						targets = append(targets, target)
						p.skipSpaces()
					} else if err != nil {
						return targets, false, err
					} else {
						return targets, false, fmt.Errorf("not a valid identifier " +
							"for a go object field")
					}
				}

				if starCount(targets) > 1 {
					return targets, false, fmt.Errorf("more than one asterisk")
				}
				if p.skipByte(')') {
					return targets, true, nil
				}
				return targets, false, fmt.Errorf("expected closing parentheses")
			} else if err != nil {
				return targets, false, err
			} else {
				return targets, false, fmt.Errorf("not a valid identifier " +
					"for a go object field")
			}
		}
	}
	cp.restore()
	return targets, false, nil
}

// starCount returns the number of FullNames in the argument with a asterisk in
// the Name field.
func starCount(fns []FullName) int {
	s := 0
	for _, fn := range fns {
		if fn.Name == "*" {
			s++
		}
	}
	return s
}

// parseOutputExpression parses all output expressions. The ampersand must be
// preceded by a space and followed by a name byte.
func (p *Parser) parseOutputExpression() (op *OutputPart, ok bool, err error) {
	cp := p.save()
	var cols []FullName
	var targets []FullName

	// Case 1: simple case with no columns e.g. &Person.*
	if targets, ok, err = p.parseTargets(); ok {
		return &OutputPart{cols, targets}, true, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("output expression: %s", err)
	} else if cols, ok = p.parseColumns(); ok {
		// Case 2: The expression contains an AS e.g. "p.col1 AS &Person.*".
		p.skipSpaces()
		if p.skipString("AS") {
			if targets, ok, err = p.parseTargets(); ok {
				// If the target is not * then check there are equal columns
				// and targets.
				if !(len(targets) == 1 && targets[0].Name == "*") {
					if len(cols) != len(targets) {
						return nil, false, fmt.Errorf("output expression: "+
							"number of cols = %d but number of targets = %d",
							len(cols), len(targets))
					}
				}

				// If the target is not M check that there are not mixed *
				// and regular columns.
				if targets[0].Prefix != "M" && len(cols) > 1 &&
					starCount(cols) >= 1 {
					return nil, false, fmt.Errorf("output expression: " +
						"cannot mix asterisk and explicit columns")
				}
				return &OutputPart{cols, targets}, true, nil
			}
			if err != nil {
				return nil, false, fmt.Errorf("output expression: %s",
					err)
			}
		}
	}
	cp.restore()
	return nil, false, nil
}

// parseInputExpression parses an input expression of the form $Type.name.
func (p *Parser) parseInputExpression() (*InputPart, bool, error) {
	cp := p.save()

	if p.skipByte('$') {
		if fn, ok, err := p.parseGoObject(); ok {
			return &InputPart{fn}, true, nil
		} else if err != nil {
			return nil, false, fmt.Errorf("input expression: %s", err)
		}
	}
	cp.restore()
	return nil, false, nil
}

// parseStringLiteral parses quoted expressions and ignores their content
// including escaped quotes.
func (p *Parser) parseStringLiteral() (*BypassPart, bool, error) {
	cp := p.save()

	if p.pos < len(p.input) {
		c := p.input[p.pos]
		if (c == '"' || c == '\'') && (p.pos == 0 || p.input[p.pos-1] != '\\') {
			p.skipByte(c)
			for p.skipByteFind(c) {
				if p.input[p.pos-2] != '\\' {
					return &BypassPart{p.input[cp.pos:p.pos]}, true, nil
				}
			}
			// Reached end of string and didn't find the closing quote
			return nil, false, fmt.Errorf("missing right quote of char %d in string literal", cp.pos)
		}
	}

	cp.restore()
	return nil, false, nil
}
