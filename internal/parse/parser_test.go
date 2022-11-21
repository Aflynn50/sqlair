package parse_test

import (
	"fmt"
	"testing"

	"github.com/canonical/sqlair/internal/parse"
	"github.com/stretchr/testify/assert"
)

type Address struct {
	ID int `db:"id"`
}

type Person struct {
	ID         int    `db:"id"`
	Fullname   string `db:"name"`
	PostalCode int    `db:"address_id"`
}

type Manager struct {
	Name string `db:"manager_name"`
}

type District struct {
}

type M map[string]any

func TestRound(t *testing.T) {
	var tests = []struct {
		input          string
		expectedParsed string
	}{
		{
			"select p.* as &Person.*",
			"ParsedExpr[BypassPart[select] OutputPart[Source:[p.*] Target:[Person.*]]]",
		},
		{
			"select p.* as &Person.*, '&notAnOutputExpresion.*' as literal from t",
			"ParsedExpr[BypassPart[select] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[, ] " +
				"BypassPart['&notAnOutputExpresion.*'] " +
				"BypassPart[ as literal from t]]",
		},
		{
			"select * as &Person.* from t",
			"ParsedExpr[BypassPart[select] " +
				"OutputPart[Source:[*] Target:[Person.*]] " +
				"BypassPart[from t]]",
		},
		{
			"select foo, bar from table where foo = $Person.id",
			"ParsedExpr[BypassPart[select foo, bar from table where foo =] " +
				"InputPart[Person.ID]]",
		},
		{
			"select &Person.* from table where foo = $Address.id",
			"ParsedExpr[BypassPart[select] OutputPart[Source:[] Target:[Person.*]] " +
				"BypassPart[from table where foo =] " +
				"InputPart[Address.ID]]",
		},
		{
			"select &Person.* from table where foo = $Address.id",
			"ParsedExpr[BypassPart[select] " +
				"OutputPart[Source:[] Target:[Person.*]] " +
				"BypassPart[from table where foo =] " +
				"InputPart[Address.ID]]",
		},
		{
			"select foo, bar, &Person.id from table where foo = 'xx'",
			"ParsedExpr[BypassPart[select foo, bar,] " +
				"OutputPart[Source:[] Target:[Person.ID]] " +
				"BypassPart[from table where foo = ] " +
				"BypassPart['xx']]",
		},
		{
			"select foo, &Person.id, bar, baz, &Manager.name from table where foo = 'xx'",
			"ParsedExpr[BypassPart[select foo,] " +
				"OutputPart[Source:[] Target:[Person.ID]] " +
				"BypassPart[, bar, baz,] " +
				"OutputPart[Source:[] Target:[Manager.Name]] " +
				"BypassPart[from table where foo = ] " +
				"BypassPart['xx']]",
		},
		{
			"SELECT * AS &Person.* FROM person WHERE name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[*] Target:[Person.*]] " +
				"BypassPart[FROM person WHERE name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"SELECT &Person.* FROM person WHERE name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[] Target:[Person.*]] " +
				"BypassPart[FROM person WHERE name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"SELECT * AS &Person.*, a.* as &Address.* FROM person, address a WHERE name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.*] Target:[Address.*]] " +
				"BypassPart[FROM person, address a WHERE name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"SELECT (a.district, a.street) AS &(Address.district, Address.street) FROM address AS a",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.district Address.street]] " +
				"BypassPart[FROM address AS a]]",
		},
		{
			"SELECT (a.district, a.street) AS &(Address.district, Address.street), " +
				"a.id AS &Person.id FROM address AS a",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.district Address.street]] " +
				"BypassPart[,] OutputPart[Source:[a.id] Target:[Person.id]] " +
				"BypassPart[FROM address AS a]]",
		},
		{
			"SELECT (a.district, a.street) AS &(Address.district, Address.street), " +
				"&Person.* FROM address AS a",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.district Address.street]] " +
				"BypassPart[,] OutputPart[Source:[] Target:[Person.*]] " +
				"BypassPart[FROM address AS a]]",
		},
		{
			"SELECT (a.district, a.street) AS &Address.* FROM address AS a WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.*]] " +
				"BypassPart[FROM address AS a WHERE p.name = ] BypassPart['Fred']]",
		},
		{
			"SELECT 1 FROM person WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT 1 FROM person WHERE p.name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.*, " +
				"(5+7), (col1 * col2) as calculated_value FROM person AS p " +
				"JOIN address AS a ON p.address_id = a.id WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.*]] " +
				"BypassPart[, (5+7), (col1 * col2) as calculated_value FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* " +
				"FROM person AS p JOIN address AS a ON p .address_id = a.id " +
				"WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.*]] " +
				"BypassPart[FROM person AS p JOIN address AS a ON p .address_id = a.id WHERE p.name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* " +
				"FROM person AS p JOIN address AS a ON p.address_id = a.id " +
				"WHERE p.name in (select name from table where table.n = $Person.name)",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.*]] " +
				"BypassPart[FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name in (select name from table where table.n =] " +
				"InputPart[Person.name] " +
				"BypassPart[)]]",
		},
		{
			"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* " +
				"FROM person WHERE p.name in (select name from table " +
				"where table.n = $Person.name) UNION " +
				"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* " +
				"FROM person WHERE p.name in " +
				"(select name from table where table.n = $Person.name)",
			"ParsedExpr[BypassPart[SELECT] OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] OutputPart[Source:[a.district a.street] Target:[Address.*]] " +
				"BypassPart[FROM person WHERE p.name in (select name from table where table.n =] " +
				"InputPart[Person.name] " +
				"BypassPart[) UNION SELECT] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.*]] " +
				"BypassPart[FROM person WHERE p.name in (select name from table where table.n =] " +
				"InputPart[Person.name] " +
				"BypassPart[)]]",
		},
		{
			"SELECT p.* AS &Person.*, m.* AS &Manager.* " +
				"FROM person AS p JOIN person AS m " +
				"ON p.manager_id = m.id WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[m.*] Target:[Manager.*]] " +
				"BypassPart[FROM person AS p JOIN person AS m ON p.manager_id = m.id WHERE p.name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"SELECT person.*, address.district FROM person JOIN address " +
				"ON person.address_id = address.id WHERE person.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT person.*, address.district FROM person JOIN address ON person.address_id = address.id WHERE person.name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"SELECT p FROM person WHERE p.name = $Person.name",
			"ParsedExpr[BypassPart[SELECT p FROM person WHERE p.name =] InputPart[Person.name]]",
		},
		{
			"SELECT p.* AS &Person.*, a.district AS &District.* " +
				"FROM person AS p JOIN address AS a ON p.address_id = a.id " +
				"WHERE p.name = $Person.name AND p.address_id = $Person.address_id",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.District] Target:[District.*]] " +
				"BypassPart[FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name =] " +
				"InputPart[Person.name] " +
				"BypassPart[AND p.address_id =] " +
				"InputPart[Person.address_id]]",
		},
		{
			"SELECT p.* AS &Person.*, a.district AS &District.* " +
				"FROM person AS p INNER JOIN address AS a " +
				"ON p.address_id = $Address.id " +
				"WHERE p.name = $Person.name AND p.address_id = $Person.address_id",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.District] Target:[District.*]] " +
				"BypassPart[FROM person AS p INNER JOIN address AS a ON p.address_id =] " +
				"InputPart[Address.ID] " +
				"BypassPart[WHERE p.name =] " +
				"InputPart[Person.name] " +
				"BypassPart[AND p.address_id =] " +
				"InputPart[Person.address_id]]",
		},
		{
			"SELECT p.*, a.district " +
				"FROM person AS p JOIN address AS a ON p.address_id = a.id " +
				"WHERE p.name = $Person.name",
			"ParsedExpr[BypassPart[SELECT p.*, a.district FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name =] " +
				"InputPart[Person.name]]",
		},
		{
			"SELECT p.*, a.district " +
				"FROM person AS p WHERE p.name=$Person.name",
			"ParsedExpr[BypassPart[SELECT p.*, a.district FROM person AS p WHERE p.name=] " +
				"InputPart[Person.name]]",
		},
		{
			"SELECT FUNC() AS &Person.name " +
				"FROM person AS p",
			"ParsedExpr[BypassPart[SELECT FUNC() AS] OutputPart[Source:[] Target:[Person.Name]] " +
				"BypassPart[FROM person AS p]]",
		},
		{
			"SELECT FUNC() AS &Person.name " +
				"FROM person AS p",
			"ParsedExpr[BypassPart[SELECT FUNC() AS] OutputPart[Source:[] Target:[Person.Name]] " +
				"BypassPart[FROM person AS p]]",
		},
		{
			"SELECT Foo & Bar FROM person AS p",
			"ParsedExpr[BypassPart[SELECT Foo & Bar FROM person AS p]]",
		},
		{
			"SELECT Foo && Bar FROM person AS p",
			"ParsedExpr[BypassPart[SELECT Foo && Bar FROM person AS p]]",
		},
		{
			"SELECT $ FROM moneytable",
			"ParsedExpr[BypassPart[SELECT $ FROM moneytable]]",
		},
		{
			"SELECT foo FROM data$",
			"ParsedExpr[BypassPart[SELECT foo FROM data$]]",
		},
		{
			"SELECT dollerrow$ FROM moneytable",
			"ParsedExpr[BypassPart[SELECT dollerrow$ FROM moneytable]]",
		},
		{
			"INSERT INTO person (name) VALUES $Person.name",
			"ParsedExpr[BypassPart[INSERT INTO person (name) VALUES] " +
				"InputPart[Person.name]]",
		},
		{
			"INSERT INTO person VALUES $Person.*",
			"ParsedExpr[BypassPart[INSERT INTO person VALUES] " +
				"InputPart[Person.*]]",
		},
		{
			"UPDATE person SET person.address_id = $Address.id " +
				"WHERE person.id = $Person.ID",
			"ParsedExpr[BypassPart[UPDATE person SET person.address_id =] " +
				"InputPart[Address.ID] " +
				"BypassPart[WHERE person.id =] " +
				"InputPart[Person.ID]]",
		},
	}

	parser := parse.NewParser()
	for i, test := range tests {
		var parsedExpr *parse.ParsedExpr
		var err error
		if parsedExpr, err = parser.Parse(test.input); err != nil {
			t.Errorf("test %d failed (Parse): input: %s\nexpected: %s\nerr: %s\n", i, test.input, test.expectedParsed, err)
		} else if parsedExpr.String() != test.expectedParsed {
			t.Errorf("test %d failed (Parse): input: %s\nexpected: %s\nactual:   %s\n", i, test.input, test.expectedParsed, parsedExpr.String())
		}
	}
}

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
