package expr

import "fmt"

// A QueryPart represents a section of a parsed SQL statement, which forms
// a complete query when processed together with its surrounding parts, in
// their correct order.
type queryPart interface {
	// String returns the part's representation for debugging purposes.
	toString() string

	// ToSQL returns the SQL representation of the part.
	toSQL() string
}

// FullName represents a table column or a Go type identifier.
type fullName struct {
	prefix, name string
}

func (fn fullName) toString() string {
	if fn.prefix == "" {
		return fn.name
	} else if fn.name == "" {
		return fn.prefix
	}
	return fn.prefix + "." + fn.name
}

// InputPart represents a named parameter that will be sent to the database
// while performing the query.
type inputPart struct {
	source fullName
}

func (p *inputPart) toString() string {
	return fmt.Sprintf("InputPart[%+v]", p.source)
}

func (p *inputPart) toSQL() string {
	return ""
}

// OutputPart represents a named target output variable in the SQL expression,
// as well as the source table and column where it will be read from.
type outputPart struct {
	source []fullName
	target []fullName
}

func (p *outputPart) toString() string {
	return fmt.Sprintf("OutputPart[%+v %+v]", p.source, p.target)
}

func (p *outputPart) toSQL() string {
	return ""
}

// BypassPart represents a part of the expression that we want to pass to the
// backend database verbatim.
type bypassPart struct {
	chunk string
}

func (p *bypassPart) toString() string {
	return "BypassPart[" + p.chunk + "]"
}

func (p *bypassPart) toSQL() string {
	return p.chunk
}
