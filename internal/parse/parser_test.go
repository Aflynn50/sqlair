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
		input             string
		expectedParsed    string
		prepArgs          []any
		completeArgs      []any
		expectedCompleted string
	}{
		{
			"select p.* as &Person.*",
			"ParsedExpr[BypassPart[select] OutputPart[Source:p.* Target:Person.*]]",
			[]any{&Person{}},
			[]any{&Person{}},
			"select p.*",
		},
		{
			"select p.* AS&Person.*",
			"ParsedExpr[BypassPart[select] OutputPart[Source:p.* Target:Person.*]]",
			[]any{&Person{}},
			[]any{&Person{}},
			"select p.*",
		},
		{
			"select p.* as &Person.*, '&notAnOutputExpresion.*' as literal from t",
			"ParsedExpr[BypassPart[select] " +
				"OutputPart[Source:p.* Target:Person.*] " +
				"BypassPart[,] " +
				"BypassPart[ '&notAnOutputExpresion.*'] " +
				"BypassPart[ as literal from t]]",
			[]any{&Person{}},
			[]any{&Person{}},
			"select p.* ,  '&notAnOutputExpresion.*'  as literal from t",
		},
		{
			"select * as &Person.* from t",
			"ParsedExpr[BypassPart[select] " +
				"OutputPart[Source:* Target:Person.*] " +
				"BypassPart[from t]]",
			[]any{&Person{}},
			[]any{&Person{}},
			"select *  from t",
		},
		{
			"select foo, bar from table where foo = $Person.ID",
			"ParsedExpr[BypassPart[select foo, bar from table where foo =] " +
				"InputPart[Person.ID]]",
			[]any{&Person{}},
			[]any{&Person{}},
			"select foo, bar from table where foo = ?",
		},
		{
			"select &Person from table where foo = $Address.ID",
			"ParsedExpr[BypassPart[select] OutputPart[Source: Target:Person] " +
				"BypassPart[from table where foo =] " +
				"InputPart[Address.ID]]",
			[]any{&Person{}, &Address{}},
			[]any{&Person{}, &Address{}},
			"select address_id, id, name  from table where foo = ?",
		},
		{
			"select &Person.* from table where foo = $Address.ID",
			"ParsedExpr[BypassPart[select] " +
				"OutputPart[Source: Target:Person.*] " +
				"BypassPart[from table where foo =] " +
				"InputPart[Address.ID]]",
			[]any{&Person{}, &Address{}},
			[]any{&Person{}, &Address{}},
			"select address_id, id, name  from table where foo = ?",
		},
		{
			"select foo, bar, &Person.ID from table where foo = 'xx'",
			"ParsedExpr[BypassPart[select foo, bar,] " +
				"OutputPart[Source: Target:Person.ID] " +
				"BypassPart[from table where foo =] " +
				"BypassPart[ 'xx']]",
			[]any{&Person{}},
			[]any{&Person{}},
			"select foo, bar, id  from table where foo =  'xx'",
		},
		{
			"select foo, &Person.ID, bar, baz, &Manager.Name from table where foo = 'xx'",
			"ParsedExpr[BypassPart[select foo,] " +
				"OutputPart[Source: Target:Person.ID] " +
				"BypassPart[, bar, baz,] " +
				"OutputPart[Source: Target:Manager.Name] " +
				"BypassPart[from table where foo =] " +
				"BypassPart[ 'xx']]",
			[]any{&Person{}, &Manager{}},
			[]any{&Person{}, &Manager{}},
			"select foo, id , bar, baz, manager_name  from table where foo =  'xx'",
		},
		{
			"SELECT * AS &Person.* FROM person WHERE name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:* Target:Person.*] " +
				"BypassPart[FROM person WHERE name =] " +
				"BypassPart[ 'Fred']]",
			[]any{&Person{}},
			[]any{&Person{}},
			"SELECT *  FROM person WHERE name =  'Fred'",
		},
		{
			"SELECT &Person.* FROM person WHERE name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source: Target:Person.*] " +
				"BypassPart[FROM person WHERE name =] " +
				"BypassPart[ 'Fred']]",
			[]any{&Person{}},
			[]any{&Person{}},
			"SELECT address_id, id, name  FROM person WHERE name =  'Fred'",
		},
		{
			"SELECT * AS &Person.*, a.* as &Address.* FROM person, address a WHERE name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:* Target:Person.*] " +
				"BypassPart[,] " +
				"OutputPart[Source:a.* Target:Address.*] " +
				"BypassPart[FROM person, address a WHERE name =] " +
				"BypassPart[ 'Fred']]",
			[]any{&Person{}, &Address{}},
			[]any{&Person{}, &Address{}},
			"SELECT * , a.*  FROM person, address a WHERE name =  'Fred'",
		},
		{
			"SELECT (a.district, a.street) AS &Address.* FROM address AS a WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:a.district a.street Target:Address.*] " +
				"BypassPart[FROM address AS a WHERE p.name =] BypassPart[ 'Fred']]",
			[]any{&Address{}},
			[]any{&Address{}},
			"SELECT a.district, a.street  FROM address AS a WHERE p.name =  'Fred'",
		},
		{
			"SELECT 1 FROM person WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT 1 FROM person WHERE p.name =] " +
				"BypassPart[ 'Fred']]",
			[]any{},
			[]any{},
			"SELECT 1 FROM person WHERE p.name =  'Fred'",
		},
		{
			"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.*, " +
				"(5+7), (col1 * col2) as calculated_value FROM person AS p " +
				"JOIN address AS a ON p.address_id = a.id WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:p.* Target:Person.*] " +
				"BypassPart[,] " +
				"OutputPart[Source:a.district a.street Target:Address.*] " +
				"BypassPart[, (5+7), (col1 * col2) as calculated_value FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name =] " +
				"BypassPart[ 'Fred']]",
			[]any{&Person{}, &Address{}},
			[]any{&Person{}, &Address{}},
			"SELECT p.* , a.district, a.street , (5+7), (col1 * col2) as calculated_value FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name =  'Fred'",
		},
		{
			"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* " +
				"FROM person AS p JOIN address AS a ON p .address_id = a.id " +
				"WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:p.* Target:Person.*] " +
				"BypassPart[,] " +
				"OutputPart[Source:a.district a.street Target:Address.*] " +
				"BypassPart[FROM person AS p JOIN address AS a ON p .address_id = a.id WHERE p.name =] " +
				"BypassPart[ 'Fred']]",
			[]any{&Person{}, &Address{}},
			[]any{&Person{}, &Address{}},
			"SELECT p.* , a.district, a.street  FROM person AS p JOIN address AS a ON p .address_id = a.id WHERE p.name =  'Fred'",
		},
		{
			"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* " +
				"FROM person AS p JOIN address AS a ON p.address_id = a.id " +
				"WHERE p.name in (select name from table where table.n = $Person.name)",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:p.* Target:Person.*] " +
				"BypassPart[,] " +
				"OutputPart[Source:a.district a.street Target:Address.*] " +
				"BypassPart[FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name in (select name from table where table.n =] " +
				"InputPart[Person.name] " +
				"BypassPart[)]]",
			[]any{&Person{}, &Address{}},
			[]any{&Person{}, &Address{}, &Person{}},
			"SELECT p.* , a.district, a.street  FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name in (select name from table where table.n = ? )",
		},
		{
			"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* " +
				"FROM person WHERE p.name in (select name from table " +
				"where table.n = $Person.name) UNION " +
				"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* " +
				"FROM person WHERE p.name in " +
				"(select name from table where table.n = $Person.name)",
			"ParsedExpr[BypassPart[SELECT] OutputPart[Source:p.* Target:Person.*] " +
				"BypassPart[,] OutputPart[Source:a.district a.street Target:Address.*] " +
				"BypassPart[FROM person WHERE p.name in (select name from table where table.n =] " +
				"InputPart[Person.name] " +
				"BypassPart[) UNION SELECT] " +
				"OutputPart[Source:p.* Target:Person.*] " +
				"BypassPart[,] " +
				"OutputPart[Source:a.district a.street Target:Address.*] " +
				"BypassPart[FROM person WHERE p.name in (select name from table where table.n =] " +
				"InputPart[Person.name] " +
				"BypassPart[)]]",
			[]any{&Person{}, &Address{}},
			[]any{&Person{}, &Address{}, &Person{}, &Person{}, &Address{}, &Person{}},
			"SELECT p.* , a.district, a.street  FROM person WHERE p.name in (select name from table where table.n = ? ) UNION SELECT p.* , a.district, a.street  FROM person WHERE p.name in (select name from table where table.n = ? )",
		},
		{
			"SELECT p.* AS &Person.*, m.* AS &Manager.* " +
				"FROM person AS p JOIN person AS m " +
				"ON p.manager_id = m.id WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:p.* Target:Person.*] " +
				"BypassPart[,] " +
				"OutputPart[Source:m.* Target:Manager.*] " +
				"BypassPart[FROM person AS p JOIN person AS m ON p.manager_id = m.id WHERE p.name =] " +
				"BypassPart[ 'Fred']]",
			[]any{&Person{}, &Manager{}},
			[]any{&Person{}, &Manager{}},
			"SELECT p.* , m.*  FROM person AS p JOIN person AS m ON p.manager_id = m.id WHERE p.name =  'Fred'",
		},
		{
			"SELECT person.*, address.district FROM person JOIN address " +
				"ON person.address_id = address.id WHERE person.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT person.*, address.district FROM person JOIN address ON person.address_id = address.id WHERE person.name =] " +
				"BypassPart[ 'Fred']]",
			[]any{},
			[]any{},
			"SELECT person.*, address.district FROM person JOIN address ON person.address_id = address.id WHERE person.name =  'Fred'",
		},
		{
			"SELECT p FROM person WHERE p.name = $Person.name",
			"ParsedExpr[BypassPart[SELECT p FROM person WHERE p.name =] InputPart[Person.name]]",
			[]any{&Person{}},
			[]any{&Person{}},
			"SELECT p FROM person WHERE p.name = ?",
		},
		{
			"SELECT p.* AS &Person, a.District AS &District " +
				"FROM person AS p JOIN address AS a ON p.address_id = a.id " +
				"WHERE p.name = $Person.name AND p.address_id = $Person.address_id",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:p.* Target:Person] " +
				"BypassPart[,] " +
				"OutputPart[Source:a.District Target:District] " +
				"BypassPart[FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name =] " +
				"InputPart[Person.name] " +
				"BypassPart[AND p.address_id =] " +
				"InputPart[Person.address_id]]",
			[]any{&Person{}, &District{}},
			[]any{&Person{}, &District{}, &Person{}, &Person{}},
			"SELECT p.* , a.District  FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name = ?  AND p.address_id = ?",
		},
		{
			"SELECT p.* AS &Person, a.District AS &District " +
				"FROM person AS p INNER JOIN address AS a " +
				"ON p.address_id = $Address.ID " +
				"WHERE p.name = $Person.name AND p.address_id = $Person.address_id",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:p.* Target:Person] " +
				"BypassPart[,] " +
				"OutputPart[Source:a.District Target:District] " +
				"BypassPart[FROM person AS p INNER JOIN address AS a ON p.address_id =] " +
				"InputPart[Address.ID] " +
				"BypassPart[WHERE p.name =] " +
				"InputPart[Person.name] " +
				"BypassPart[AND p.address_id =] " +
				"InputPart[Person.address_id]]",
			[]any{&Address{}, &Person{}, &District{}},
			[]any{&Person{}, &District{}, &Address{}, &Person{}, &Person{}},
			"SELECT p.* , a.District  FROM person AS p INNER JOIN address AS a ON p.address_id = ?  WHERE p.name = ?  AND p.address_id = ?",
		},
		{
			"SELECT p.*, a.district " +
				"FROM person AS p JOIN address AS a ON p.address_id = a.id " +
				"WHERE p.name = $Person.*",
			"ParsedExpr[BypassPart[SELECT p.*, a.district FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name =] " +
				"InputPart[Person.*]]",
			[]any{&Person{}},
			[]any{&Person{}},
			"SELECT p.*, a.district FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name = ?",
		},
		{
			"INSERT INTO person (name) VALUES $Person.name",
			"ParsedExpr[BypassPart[INSERT INTO person (name) VALUES] " +
				"InputPart[Person.name]]",
			[]any{&Person{}},
			[]any{&Person{}},
			"INSERT INTO person (name) VALUES ?",
		},
		{
			"INSERT INTO person VALUES $Person.*",
			"ParsedExpr[BypassPart[INSERT INTO person VALUES] " +
				"InputPart[Person.*]]",
			[]any{&Person{}},
			[]any{&Person{}},
			"INSERT INTO person VALUES ?",
		},
		{
			"UPDATE person SET person.address_id = $Address.ID " +
				"WHERE person.id = $Person.ID",
			"ParsedExpr[BypassPart[UPDATE person SET person.address_id =] " +
				"InputPart[Address.ID] " +
				"BypassPart[WHERE person.id =] " +
				"InputPart[Person.ID]]",
			[]any{&Address{}, &Person{}},
			[]any{&Address{}, &Person{}},
			"UPDATE person SET person.address_id = ?  WHERE person.id = ?",
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
	assert.Equal(t, fmt.Errorf("parser error: missing right quote in string literal"), err)
}

func TestUnfinishedStringLiteralV2(t *testing.T) {
	sql := "select foo from t where x = \"dddd"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: missing right quote in string literal"), err)
}

// We require to end the string literal with the proper quote depending
// on the opening one.
func TestUnfinishedStringLiteralV3(t *testing.T) {
	sql := "select foo from t where x = \"dddd'"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: missing right quote in string literal"), err)
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
	assert.Equal(t, fmt.Errorf("parser error: missing right quote in string literal"), err)
}

// Detect bad input DSL pieces
func TestBadFormatInput(t *testing.T) {
	sql := "select foo from t where x = $.id"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: malformed input type"), err)
}

// Detect bad input DSL pieces
func TestBadFormatInputV2(t *testing.T) {
	sql := "select foo from t where x = $Address."
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: malformed input type"), err)
}

// Detect bad input DSL pieces
func TestBadFormatInputV3(t *testing.T) {
	sql := "select foo from t where x = $"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: malformed input type"), err)
}

// Detect bad input DSL pieces
func TestBadFormatInputV4(t *testing.T) {
	sql := "select foo from t where x = $$Address"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: malformed input type"), err)
}

// Detect bad input DSL pieces
func TestBadFormatInputV5(t *testing.T) {
	sql := "select foo from t where x = $```"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: malformed input type"), err)
}

// Detect bad input DSL pieces
func TestBadFormatInputV6(t *testing.T) {
	sql := "select foo from t where x = $.."
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: malformed input type"), err)
}

// Detect bad input DSL pieces
func TestBadFormatInputV7(t *testing.T) {
	sql := "select foo from t where x = $."
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: malformed input type"), err)
}

// Detect bad output DSL pieces
func TestBadFormatOutput(t *testing.T) {
	sql := "select foo as && from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: malformed output expression"), err)
}

// Detect bad output DSL pieces
func TestBadFormatOutputV2(t *testing.T) {
	sql := "select foo as &.bar from t"
	parser := parse.NewParser()
	_, err := parser.Parse(sql)
	assert.Equal(t, fmt.Errorf("parser error: malformed output expression"), err)
}
