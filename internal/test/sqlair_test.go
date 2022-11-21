package test

import (
	"testing"

	"github.com/canonical/sqlair/internal/parse"
)

type Address struct {
	ID       int    `db:"id"`
	District string `db:"district"`
	Street   string `db:"street"`
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
		name           string
		input          string
		expectedParsed string
	}{
		{
			"simple select",
			"select p.* as &Person.*",
			"ParsedExpr[BypassPart[select] OutputPart[Source:[p.*] Target:[Person.*]]]",
		},
		{
			"quoted output expr",
			"select p.* as &Person.*, '&notAnOutputExpresion.*' as literal from t",
			"ParsedExpr[BypassPart[select] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[, ] " +
				"BypassPart['&notAnOutputExpresion.*'] " +
				"BypassPart[ as literal from t]]",
		},
		{
			"output",
			"select * as &Person.* from t",
			"ParsedExpr[BypassPart[select] " +
				"OutputPart[Source:[*] Target:[Person.*]] " +
				"BypassPart[from t]]",
		},
		{
			"input",
			"select foo, bar from table where foo = $Person.id",
			"ParsedExpr[BypassPart[select foo, bar from table where foo =] " +
				"InputPart[Person.id]]",
		},
		{
			"input and output",
			"select &Person.* from table where foo = $Address.id",
			"ParsedExpr[BypassPart[select] OutputPart[Source:[] Target:[Person.*]] " +
				"BypassPart[from table where foo =] " +
				"InputPart[Address.id]]",
		},
		{
			"input and star output",
			"select &Person.* from table where foo = $Address.id",
			"ParsedExpr[BypassPart[select] " +
				"OutputPart[Source:[] Target:[Person.*]] " +
				"BypassPart[from table where foo =] " +
				"InputPart[Address.id]]",
		},
		{
			"output and quote",
			"select foo, bar, &Person.id from table where foo = 'xx'",
			"ParsedExpr[BypassPart[select foo, bar,] " +
				"OutputPart[Source:[] Target:[Person.id]] " +
				"BypassPart[from table where foo = ] " +
				"BypassPart['xx']]",
		},
		{
			"two outputs and quote",
			"select foo, &Person.id, bar, baz, &Manager.name from table where foo = 'xx'",
			"ParsedExpr[BypassPart[select foo,] " +
				"OutputPart[Source:[] Target:[Person.id]] " +
				"BypassPart[, bar, baz,] " +
				"OutputPart[Source:[] Target:[Manager.name]] " +
				"BypassPart[from table where foo = ] " +
				"BypassPart['xx']]",
		},
		{
			"star as output and quote",
			"SELECT * AS &Person.* FROM person WHERE name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[*] Target:[Person.*]] " +
				"BypassPart[FROM person WHERE name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"star output and quote",
			"SELECT &Person.* FROM person WHERE name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[] Target:[Person.*]] " +
				"BypassPart[FROM person WHERE name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"two star as output and quote",
			"SELECT * AS &Person.*, a.* as &Address.* FROM person, address a WHERE name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.*] Target:[Address.*]] " +
				"BypassPart[FROM person, address a WHERE name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"multicolumn output and quote",
			"SELECT (a.district, a.street) AS &(Address.district, Address.street) FROM address AS a",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.district Address.street]] " +
				"BypassPart[FROM address AS a]]",
		},
		{
			"multiple targets v1",
			"SELECT (a.district, a.street) AS &(Address.district, Address.street), " +
				"a.id AS &Person.id FROM address AS a",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.district Address.street]] " +
				"BypassPart[,] OutputPart[Source:[a.id] Target:[Person.id]] " +
				"BypassPart[FROM address AS a]]",
		},
		{
			"multiple targets v2",
			"SELECT (a.district, a.street) AS &(Address.district, Address.street), " +
				"&Person.* FROM address AS a",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.district Address.street]] " +
				"BypassPart[,] OutputPart[Source:[] Target:[Person.*]] " +
				"BypassPart[FROM address AS a]]",
		},
		{
			"multiple targets v3",
			"SELECT (a.district, a.street) AS &Address.* FROM address AS a WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[a.district a.street] Target:[Address.*]] " +
				"BypassPart[FROM address AS a WHERE p.name = ] BypassPart['Fred']]",
		},
		{
			"quote v2",
			"SELECT 1 FROM person WHERE p.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT 1 FROM person WHERE p.name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"complex query",
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
			"complex query v2",
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
			"complex query v3",
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
			"complex query v4",
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
			"join",
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
			"join v2",
			"SELECT person.*, address.district FROM person JOIN address " +
				"ON person.address_id = address.id WHERE person.name = 'Fred'",
			"ParsedExpr[BypassPart[SELECT person.*, address.district FROM person JOIN address ON person.address_id = address.id WHERE person.name = ] " +
				"BypassPart['Fred']]",
		},
		{
			"simple input v2",
			"SELECT p FROM person WHERE p.name = $Person.name",
			"ParsedExpr[BypassPart[SELECT p FROM person WHERE p.name =] InputPart[Person.name]]",
		},
		{
			"complex query v5 with empty struct",
			"SELECT p.* AS &Person.*, a.district AS &District.* " +
				"FROM person AS p JOIN address AS a ON p.address_id = a.id " +
				"WHERE p.name = $Person.name AND p.address_id = $Person.address_id",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.district] Target:[District.*]] " +
				"BypassPart[FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name =] " +
				"InputPart[Person.name] " +
				"BypassPart[AND p.address_id =] " +
				"InputPart[Person.address_id]]",
		},
		{
			"complex query v7",
			"SELECT p.* AS &Person.*, a.district AS &District.* " +
				"FROM person AS p INNER JOIN address AS a " +
				"ON p.address_id = $Address.id " +
				"WHERE p.name = $Person.name AND p.address_id = $Person.address_id",
			"ParsedExpr[BypassPart[SELECT] " +
				"OutputPart[Source:[p.*] Target:[Person.*]] " +
				"BypassPart[,] " +
				"OutputPart[Source:[a.district] Target:[District.*]] " +
				"BypassPart[FROM person AS p INNER JOIN address AS a ON p.address_id =] " +
				"InputPart[Address.id] " +
				"BypassPart[WHERE p.name =] " +
				"InputPart[Person.name] " +
				"BypassPart[AND p.address_id =] " +
				"InputPart[Person.address_id]]",
		},
		{
			"input v2",
			"SELECT p.*, a.district " +
				"FROM person AS p JOIN address AS a ON p.address_id = a.id " +
				"WHERE p.name = $Person.name",
			"ParsedExpr[BypassPart[SELECT p.*, a.district FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name =] " +
				"InputPart[Person.name]]",
		},
		{
			"no space before input",
			"SELECT p.*, a.district " +
				"FROM person AS p WHERE p.name=$Person.name",
			"ParsedExpr[BypassPart[SELECT p.*, a.district FROM person AS p WHERE p.name=] " +
				"InputPart[Person.name]]",
		},
		{
			"function output",
			"SELECT FUNC() AS &Person.name " +
				"FROM person AS p",
			"ParsedExpr[BypassPart[SELECT FUNC() AS] OutputPart[Source:[] Target:[Person.name]] " +
				"BypassPart[FROM person AS p]]",
		},
		{
			"ignore ampersand",
			"SELECT Foo & Bar FROM person AS p",
			"ParsedExpr[BypassPart[SELECT Foo & Bar FROM person AS p]]",
		},
		{
			"ignore ampersand v2",
			"SELECT Foo && Bar FROM person AS p",
			"ParsedExpr[BypassPart[SELECT Foo && Bar FROM person AS p]]",
		},
		{
			"ignore doller",
			"SELECT $ FROM moneytable",
			"ParsedExpr[BypassPart[SELECT $ FROM moneytable]]",
		},
		{
			"ignore doller v2",
			"SELECT foo FROM data$",
			"ParsedExpr[BypassPart[SELECT foo FROM data$]]",
		},
		{
			"ignore doller v3",
			"SELECT dollerrow$ FROM moneytable",
			"ParsedExpr[BypassPart[SELECT dollerrow$ FROM moneytable]]",
		},
		{
			"insert",
			"INSERT INTO person (name) VALUES $Person.name",
			"ParsedExpr[BypassPart[INSERT INTO person (name) VALUES] " +
				"InputPart[Person.name]]",
		},
		{
			"insert v2",
			"INSERT INTO person VALUES $Person.*",
			"ParsedExpr[BypassPart[INSERT INTO person VALUES] " +
				"InputPart[Person.*]]",
		},
		{
			"update",
			"UPDATE person SET person.address_id = $Address.id " +
				"WHERE person.id = $Person.id",
			"ParsedExpr[BypassPart[UPDATE person SET person.address_id =] " +
				"InputPart[Address.id] " +
				"BypassPart[WHERE person.id =] " +
				"InputPart[Person.id]]",
		},
	}

	parser := parse.NewParser()
	for i, test := range tests {
		var parsedExpr *parse.ParsedExpr
		var err error
		if parsedExpr, err = parser.Parse(test.input); err != nil {
			t.Errorf("test %d failed (Parse): name %s\ninput: %s\nexpected: %s\nerr: %s\n", i, test.name, test.input, test.expectedParsed, err)
		} else if parsedExpr.String() != test.expectedParsed {
			t.Errorf("test %d failed (Parse): name %s\ninput: %s\nexpected: %s\nactual:   %s\n", i, test.name, test.input, test.expectedParsed, parsedExpr.String())
		}
	}
}
