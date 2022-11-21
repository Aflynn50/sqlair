package parse_test

import (
	"fmt"
	"testing"

	"github.com/canonical/sqlair/internal/parse"
	"github.com/stretchr/testify/assert"
)

// We return a proper error when we find an unbound string literal
func TestUnfinishedStringLiteral(t *testing.T) {
	sql := "select foo from t where x = 'dddd"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: missing right quote in string literal"), err)
}

func TestUnfinishedStringLiteralV2(t *testing.T) {
	sql := "select foo from t where x = \"dddd"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: missing right quote in string literal"), err)
}

// We require to end the string literal with the proper quote depending
// on the opening one.
func TestUnfinishedStringLiteralV3(t *testing.T) {
	sql := "select foo from t where x = \"dddd'"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: missing right quote in string literal"), err)
}

// Properly parsing empty string literal
func TestEmptyStringLiteral(t *testing.T) {
	sql := "select foo from t where x = ''"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, nil, err)
}

// Detect bad escaped string literal
func TestBadEscaped(t *testing.T) {
	sql := "select foo from t where x = 'O'Donnell'"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: missing right quote in string literal"), err)
}

// Detect bad input expressions
func TestBadFormatInput(t *testing.T) {
	sql := "select foo from t where x = $Address."
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: input expression: not a valid identifier for a go object field"), err)
}

// Detect bad input expressions
func TestBadFormatInputV2(t *testing.T) {
	sql := "select foo from t where x = $Address"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: input expression: go objects need to be qualified"), err)
}

// Detect bad output expressions
func TestBadFormatOutput(t *testing.T) {
	sql := "select foo as &bar. from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: output expression: not a valid identifier for a go object field"), err)
}

// Detect bad output expressions
func TestBadFormatOutputV2(t *testing.T) {
	sql := "select foo as &Person from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: output expression: go objects need to be qualified"), err)
}

// Detect bad output expressions
func TestBadFormatOutputV3(t *testing.T) {
	sql := "select foo as &(Person) from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: output expression: go objects need to be qualified"), err)
}

// Detect bad output expressions
func TestBadFormatOutputV4(t *testing.T) {
	sql := "select foo, bar as &(Person.name, Person) from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: output expression: go objects need to be qualified"), err)
}

// Detect mismatched columns and targets in output expression
func TestMismatchedOutput(t *testing.T) {
	sql := "select (foo, bar) as &P.name from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: output expression: number of cols = 2 but number of targets = 1"), err)
}

// Detect mismatched columns and targets in output expression
func TestMismatchedOutputV2(t *testing.T) {
	sql := "select foo as &(P.name, P.age) from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: output expression: number of cols = 1 but number of targets = 2"), err)
}

// Detect missing brackets
func TestMissingClosingParenthesesOutput(t *testing.T) {
	sql := "select (foo, bar) as &(P.name P.id from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: output expression: expected closing parentheses"), err)
}

// Detect multiple asterisk targets
func TestMutipleTargetAsterisksOutput(t *testing.T) {
	sql := "select (foo, bar) as &(P.*, A.*) from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: output expression: more than one asterisk"), err)
}

// Detect multiple asterisk columns
func TestMutipleColumnAsterisksOutput(t *testing.T) {
	sql := "select (foo, bar, t.*) as &P.* from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("cannot parse expression: output expression: cannot mix asterisk and explicit columns"), err)
}
