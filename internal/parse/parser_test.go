package parse_test

import (
	"fmt"
	"testing"

	"github.com/canonical/sqlair/internal/parse"
	"github.com/stretchr/testify/assert"
)

// We return a proper error when we find an unbound string literal
func TestUnfinishedStringLiteral(t *testing.T) {
	sql := "SELECT foo FROM t WHERE x = 'dddd"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: missing right quote for char 28 in string literal"), err)
}

func TestUnfinishedStringLiteralV2(t *testing.T) {
	sql := "SELECT foo FROM t WHERE x = \"dddd"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: missing right quote for char 28 in string literal"), err)
}

// We require to end the string literal with the proper quote depending
// on the opening one.
func TestUnfinishedStringLiteralV3(t *testing.T) {
	sql := "SELECT foo FROM t WHERE x = \"dddd'"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: missing right quote for char 28 in string literal"), err)
}

// Properly parsing empty string literal
func TestEmptyStringLiteral(t *testing.T) {
	sql := "SELECT foo FROM t WHERE x = ''"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, nil, err)
}

// Detect bad escaped string literal
func TestBadEscaped(t *testing.T) {
	sql := "SELECT foo FROM t WHERE x = 'O'Donnell'"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: missing right quote for char 38 in string literal"), err)
}

// Detect bad input expressions
func TestBadFormatInput(t *testing.T) {
	sql := "SELECT foo FROM t WHERE x = $Address."
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: invalid identifier near char 37"), err)
}

// Detect bad input expressions
func TestBadFormatInputV2(t *testing.T) {
	sql := "SELECT foo FROM t WHERE x = $Address"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: go object near char 36 not qualified"), err)
}

// Detect bad output expressions
func TestBadFormatOutput(t *testing.T) {
	sql := "SELECT foo AS &bar. FROM t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: invalid identifier near char 19"), err)
}

// Detect bad output expressions
func TestBadFormatOutputV2(t *testing.T) {
	sql := "SELECT foo AS &Person FROM t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: go object near char 21 not qualified"), err)
}

// Detect bad output expressions
func TestBadFormatOutputV3(t *testing.T) {
	sql := "SELECT foo AS &(Person) FROM t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: go object near char 22 not qualified"), err)
}

// Detect bad output expressions
func TestBadFormatOutputV4(t *testing.T) {
	sql := "SELECT foo, bar AS &(Person.name, Person) FROM t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: go object near char 40 not qualified"), err)
}

// Detect mismatched columns and targets in output expression
func TestMismatchedOutput(t *testing.T) {
	sql := "SELECT (foo, bar) AS &P.name FROM t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: number of cols = 2 but number of targets = 1 in expression near 28"), err)
}

// Detect mismatched columns and targets in output expression
func TestMismatchedOutputV2(t *testing.T) {
	sql := "SELECT foo AS &(P.name, P.age) FROM t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: number of cols = 1 but number of targets = 2 in expression near 30"), err)
}

// Detect missing brackets
func TestMissingClosingParenthesesOutput(t *testing.T) {
	sql := "SELECT (foo, bar) AS &(P.name P.id FROM t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: missing closing parentheses for char 23"), err)
}

// Detect multiple asterisk targets
func TestMutipleTargetAsterisksOutput(t *testing.T) {
	sql := "SELECT (foo, bar) AS &(P.*, A.*) FROM t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: more than one asterisk in expression near char 32"), err)
}

// Detect multiple asterisk columns
func TestMutipleColumnAsterisksOutput(t *testing.T) {
	sql := "SELECT (foo, bar, t.*) AS &P.* FROM t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: cannot mix asterisk and explicit columns in expression near 30"), err)
}
