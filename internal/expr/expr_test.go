package expr_test

import (
	"testing"

	"github.com/canonical/sqlair/internal/expr"
	. "gopkg.in/check.v1"
)

// Hook up gocheck into the "go test" runner.
func TestExpr(t *testing.T) { TestingT(t) }

type ExprSuite struct{}

var _ = Suite(&ExprSuite{})

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

var tests = []struct {
	summary        string
	input          string
	expectedParsed string
}{{
	"star table as output",
	"SELECT p.* AS &Person.*",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]]]",
}, {
	"multiple-quoted bypass expression",
	`SELECT '''' AS &Person.*`,
	`[Bypass[SELECT '''' AS ] Output[[] [Person.*]]]`,
}, {
	"single quote in double quotes",
	`SELECT "'" AS &Person.*`,
	`[Bypass[SELECT "'" AS ] Output[[] [Person.*]]]`,
}, {
	"spaces and tabs",
	"SELECT p.* 	AS 		   &Person.*",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]]]",
}, {
	"new lines",
	`SELECT
		p.* AS &Person.*,
		foo
	 FROM t
	 WHERE
		foo = bar
		and
		x = y`,
	`[Bypass[SELECT
		] Output[[p.*] [Person.*]] Bypass[,
		foo
	 FROM t
	 WHERE
		foo = bar
		and
		x = y]]`,
}, {
	"quoted output expression",
	"SELECT p.* AS &Person.*, '&notAnOutputExpresion.*' AS literal FROM t",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]] Bypass[, '&notAnOutputExpresion.*' AS literal FROM t]]",
}, {
	"quoted input expression",
	"SELECT foo FROM t WHERE bar = '$NotAn.input'",
	"[Bypass[SELECT foo FROM t WHERE bar = '$NotAn.input']]",
}, {
	"star as output",
	"SELECT * AS &Person.* FROM t",
	"[Bypass[SELECT ] Output[[*] [Person.*]] Bypass[ FROM t]]",
}, {
	"input v1",
	"SELECT foo, bar FROM table WHERE foo = $Person.id",
	"[Bypass[SELECT foo, bar FROM table WHERE foo = ] Input[[] [Person.id]]]",
}, {
	"input v2",
	"SELECT p FROM person WHERE p.name = $Person.name",
	"[Bypass[SELECT p FROM person WHERE p.name = ] Input[[] [Person.name]]]",
}, {
	"input v3",
	"SELECT p.*, a.district FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name = $Person.name",
	"[Bypass[SELECT p.*, a.district FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name = ] Input[[] [Person.name]]]",
}, {
	"output and input",
	"SELECT &Person.* FROM table WHERE foo = $Address.id",
	"[Bypass[SELECT ] Output[[] [Person.*]] Bypass[ FROM table WHERE foo = ] Input[[] [Address.id]]]",
}, {
	"star output and input",
	"SELECT &Person.* FROM table WHERE foo = $Address.id",
	"[Bypass[SELECT ] Output[[] [Person.*]] Bypass[ FROM table WHERE foo = ] Input[[] [Address.id]]]",
}, {
	"output and quote",
	"SELECT foo, bar, &Person.id FROM table WHERE foo = 'xx'",
	"[Bypass[SELECT foo, bar, ] Output[[] [Person.id]] Bypass[ FROM table WHERE foo = 'xx']]",
}, {
	"two outputs and quote",
	"SELECT foo, &Person.id, bar, baz, &Manager.name FROM table WHERE foo = 'xx'",
	"[Bypass[SELECT foo, ] Output[[] [Person.id]] Bypass[, bar, baz, ] Output[[] [Manager.name]] Bypass[ FROM table WHERE foo = 'xx']]",
}, {
	"star as output and quote",
	"SELECT * AS &Person.* FROM person WHERE name = 'Fred'",
	"[Bypass[SELECT ] Output[[*] [Person.*]] Bypass[ FROM person WHERE name = 'Fred']]",
}, {
	"star output and quote",
	"SELECT &Person.* FROM person WHERE name = 'Fred'",
	"[Bypass[SELECT ] Output[[] [Person.*]] Bypass[ FROM person WHERE name = 'Fred']]",
}, {
	"two star as outputs and quote",
	"SELECT * AS &Person.*, a.* AS &Address.* FROM person, address a WHERE name = 'Fred'",
	"[Bypass[SELECT ] Output[[*] [Person.*]] Bypass[, ] Output[[a.*] [Address.*]] Bypass[ FROM person, address a WHERE name = 'Fred']]",
}, {
	"multicolumn output",
	"SELECT (a.district, a.street) AS (&Address.district, &Address.street) FROM address AS a",
	"[Bypass[SELECT ] Output[[a.district a.street] [Address.district Address.street]] Bypass[ FROM address AS a]]",
}, {
	"multicolumn output and output",
	"SELECT (a.district, a.street) AS (&Address.district, &Address.street), a.id AS &Person.id FROM address AS a",
	"[Bypass[SELECT ] Output[[a.district a.street] [Address.district Address.street]] Bypass[, ] Output[[a.id] [Person.id]] Bypass[ FROM address AS a]]",
}, {
	"multicolumn output and star",
	"SELECT (a.district, a.street) AS (&Address.district, &Address.street), &Person.* FROM address AS a",
	"[Bypass[SELECT ] Output[[a.district a.street] [Address.district Address.street]] Bypass[, ] Output[[] [Person.*]] Bypass[ FROM address AS a]]",
}, {
	"multicolumn output and quote",
	"SELECT (a.district, a.street) AS &Address.* FROM address AS a WHERE p.name = 'Fred'",
	"[Bypass[SELECT ] Output[[a.district a.street] [Address.*]] Bypass[ FROM address AS a WHERE p.name = 'Fred']]",
}, {
	"quote",
	"SELECT 1 FROM person WHERE p.name = 'Fred'",
	"[Bypass[SELECT 1 FROM person WHERE p.name = 'Fred']]",
}, {
	"complex query v1",
	"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.*, (5+7), (col1 * col2) AS calculated_value FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name = 'Fred'",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]] Bypass[, ] Output[[a.district a.street] [Address.*]] Bypass[, (5+7), (col1 * col2) AS calculated_value FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name = 'Fred']]",
}, {
	"complex query v2",
	"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* FROM person AS p JOIN address AS a ON p .address_id = a.id WHERE p.name = 'Fred'",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]] Bypass[, ] Output[[a.district a.street] [Address.*]] Bypass[ FROM person AS p JOIN address AS a ON p .address_id = a.id WHERE p.name = 'Fred']]",
}, {
	"complex query v3",
	"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name IN (SELECT name FROM table WHERE table.n = $Person.name)",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]] Bypass[, ] Output[[a.district a.street] [Address.*]] Bypass[ FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name IN (SELECT name FROM table WHERE table.n = ] Input[[] [Person.name]] Bypass[)]]",
}, {
	"complex query v4",
	"SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* FROM person WHERE p.name IN (SELECT name FROM table WHERE table.n = $Person.name) UNION SELECT p.* AS &Person.*, (a.district, a.street) AS &Address.* FROM person WHERE p.name IN (SELECT name FROM table WHERE table.n = $Person.name)",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]] Bypass[, ] Output[[a.district a.street] [Address.*]] Bypass[ FROM person WHERE p.name IN (SELECT name FROM table WHERE table.n = ] Input[[] [Person.name]] Bypass[) UNION SELECT ] Output[[p.*] [Person.*]] Bypass[, ] Output[[a.district a.street] [Address.*]] Bypass[ FROM person WHERE p.name IN (SELECT name FROM table WHERE table.n = ] Input[[] [Person.name]] Bypass[)]]",
}, {
	"complex query v5",
	"SELECT p.* AS &Person.*, a.district AS &District.* FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name = $Person.name AND p.address_id = $Person.address_id",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]] Bypass[, ] Output[[a.district] [District.*]] Bypass[ FROM person AS p JOIN address AS a ON p.address_id = a.id WHERE p.name = ] Input[[] [Person.name]] Bypass[ AND p.address_id = ] Input[[] [Person.address_id]]]",
}, {
	"complex query v6",
	"SELECT p.* AS &Person.*, a.district AS &District.* FROM person AS p INNER JOIN address AS a ON p.address_id = $Address.id WHERE p.name = $Person.name AND p.address_id = $Person.address_id",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]] Bypass[, ] Output[[a.district] [District.*]] Bypass[ FROM person AS p INNER JOIN address AS a ON p.address_id = ] Input[[] [Address.id]] Bypass[ WHERE p.name = ] Input[[] [Person.name]] Bypass[ AND p.address_id = ] Input[[] [Person.address_id]]]",
}, {
	"join v1",
	"SELECT p.* AS &Person.*, m.* AS &Manager.* FROM person AS p JOIN person AS m ON p.manager_id = m.id WHERE p.name = 'Fred'",
	"[Bypass[SELECT ] Output[[p.*] [Person.*]] Bypass[, ] Output[[m.*] [Manager.*]] Bypass[ FROM person AS p JOIN person AS m ON p.manager_id = m.id WHERE p.name = 'Fred']]",
}, {
	"join v2",
	"SELECT person.*, address.district FROM person JOIN address ON person.address_id = address.id WHERE person.name = 'Fred'",
	"[Bypass[SELECT person.*, address.district FROM person JOIN address ON person.address_id = address.id WHERE person.name = 'Fred']]",
}, {
	"insert v1",
	"INSERT INTO person (*) VALUES ($Person.*)",
	"[Bypass[INSERT INTO person ] Input[[*] [Person.*]]]",
}, {
	"insert v2",
	"INSERT INTO person (name, id) VALUES ($Person.*)",
	"[Bypass[INSERT INTO person ] Input[[name id] [Person.*]]]",
}, {
	"insert v3",
	"INSERT INTO person (name, postalcode) VALUES ($Person.name, $Address.id)",
	"[Bypass[INSERT INTO person ] Input[[name postalcode] [Person.name Address.id]]]",
}, {
	"ignore dollar v1",
	"SELECT $ FROM moneytable",
	"[Bypass[SELECT $ FROM moneytable]]",
}, {
	"ignore dollar v2",
	"SELECT foo FROM data$",
	"[Bypass[SELECT foo FROM data$]]",
}, {
	"ignore dollar v3",
	"SELECT dollerrow$ FROM moneytable",
	"[Bypass[SELECT dollerrow$ FROM moneytable]]",
}, {
	"input with no space",
	"SELECT p.*, a.district FROM person AS p WHERE p.name=$Person.name",
	"[Bypass[SELECT p.*, a.district FROM person AS p WHERE p.name=] Input[[] [Person.name]]]",
}, {
	"escaped double quote",
	`SELECT foo FROM t WHERE t.p = "Jimmy ""Quickfingers"" Jones"`,
	`[Bypass[SELECT foo FROM t WHERE t.p = "Jimmy ""Quickfingers"" Jones"]]`,
}, {
	"escaped single quote",
	`SELECT foo FROM t WHERE t.p = 'Olly O''Flanagan'`,
	`[Bypass[SELECT foo FROM t WHERE t.p = 'Olly O''Flanagan']]`,
}, {
	"complex escaped quotes",
	`SELECT * AS &Person.* FROM person WHERE name IN ('Lorn', 'Onos T''oolan', '', ''' ''');`,
	`[Bypass[SELECT ] Output[[*] [Person.*]] Bypass[ FROM person WHERE name IN ('Lorn', 'Onos T''oolan', '', ''' ''');]]`,
}, {
	"update",
	"UPDATE person SET person.address_id = $Address.id WHERE person.id = $Person.id",
	"[Bypass[UPDATE person SET person.address_id = ] Input[[] [Address.id]] Bypass[ WHERE person.id = ] Input[[] [Person.id]]]",
}}

func (s *ExprSuite) TestExpr(c *C) {
	parser := expr.NewParser()
	for i, test := range tests {
		var parsedExpr *expr.ParsedExpr
		var err error
		if parsedExpr, err = parser.Parse(test.input); err != nil {
			c.Errorf("test %d failed (Parse):\nsummary: %s\ninput: %s\nexpected: %s\nerr: %s\n", i, test.summary, test.input, test.expectedParsed, err)
		} else if parsedExpr.String() != test.expectedParsed {
			c.Errorf("test %d failed (Parse):\nsummary: %s\ninput: %s\nexpected: %s\nactual:   %s\n", i, test.summary, test.input, test.expectedParsed, parsedExpr.String())
		}
	}
}

func (s *ExprSuite) TestValidInput(c *C) {
	testList := []struct {
		input          string
		expectedParsed string
	}{{
		"SELECT street FROM t WHERE x = $Address.street",
		"[Bypass[SELECT street FROM t WHERE x = ] Input[[] [Address.street]]]",
	}, {
		"SELECT p FROM t WHERE x = $Person.id",
		"[Bypass[SELECT p FROM t WHERE x = ] Input[[] [Person.id]]]",
	}}
	for _, test := range testList {
		parser := expr.NewParser()
		parsedExpr, err := parser.Parse(test.input)
		if err != nil {
			c.Fatal(err)
		}
		c.Assert(parsedExpr.String(), Equals, test.expectedParsed)
	}
}

// We return a proper error when we find an unbound string literal
func (s *ExprSuite) TestParseUnfinishedStringLiteral(c *C) {
	testList := []string{
		"SELECT foo FROM t WHERE x = 'dddd",
		"SELECT foo FROM t WHERE x = \"dddd",
		"SELECT foo FROM t WHERE x = \"dddd'",
		"SELECT foo FROM t WHERE x = '''",
		`SELECT foo FROM t WHERE x = '''""`,
		`SELECT foo FROM t WHERE x = """`,
		`SELECT foo FROM t WHERE x = """''`,
		`SELECT foo FROM t WHERE x = 'O'Donnell'`,
	}

	for _, sql := range testList {
		parser := expr.NewParser()
		expr, err := parser.Parse(sql)
		c.Assert(err, ErrorMatches, "cannot parse expression: column [0-9]+: missing closing quote in string literal")
		c.Assert(expr, IsNil)
	}
}

// Properly parsing empty string literal
func (s *ExprSuite) TestParseEmptyStringLiteral(c *C) {
	sql := "SELECT foo FROM t WHERE x = ''"
	parser := expr.NewParser()
	_, err := parser.Parse(sql)
	c.Assert(err, IsNil)
}

// Detect well escaped string literals
func (s *ExprSuite) TestWellEscaped(c *C) {
	sqls := []string{
		`SELECT foo FROM t WHERE x = 'O''Donnell'`,
		`SELECT foo FROM t WHERE x = "O""Donnell"`,
		`SELECT foo FROM t WHERE x = 'O''Do''nnell'`,
		`SELECT foo FROM t WHERE x = "O""Do""nnell"`,
	}

	for _, sql := range sqls {
		parser := expr.NewParser()
		_, err := parser.Parse(sql)
		c.Assert(err, IsNil)
	}
}

func (s *ExprSuite) TestParseBadFormatInput(c *C) {
	testList := []string{
		"SELECT foo FROM t WHERE x = $Address.",
		"SELECT foo FROM t WHERE x = $Address.&d",
		"SELECT foo FROM t WHERE x = $Address.-",
	}

	for _, sql := range testList {
		parser := expr.NewParser()
		expr, err := parser.Parse(sql)
		c.Assert(err, ErrorMatches, "cannot parse expression: column [0-9]+: invalid identifier")
		c.Assert(expr, IsNil)
	}

	sql := "SELECT foo FROM t WHERE x = $Address"
	parser := expr.NewParser()
	_, err := parser.Parse(sql)
	c.Assert(err, ErrorMatches, "cannot parse expression: column [0-9]+: type not qualified")
}

func FuzzParser(f *testing.F) {
	// Add some values to the corpus
	for _, test := range tests {
		f.Add(test.input)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// Loop forever or until it crashes
		parser := expr.NewParser()
		parser.Parse(s)
	})
}

func (s *ExprSuite) TestValidPrepare(c *C) {
	testList := []struct {
		input            string
		prepareArgs      []any
		expectedPrepared string
	}{{
		"SELECT street FROM t WHERE x = $Address.street",
		[]any{Address{}},
		"SELECT street FROM t WHERE x = @[a-zA-Z_0-9]+",
	}, {
		"SELECT street FROM t WHERE x, y = ($Address.street, $Person.id)",
		[]any{Address{}, Person{}},
		`SELECT street FROM t WHERE x, y = \(@[a-zA-Z_0-9]+, @[a-zA-Z_0-9]+\)`,
	}, {
		"SELECT p FROM t WHERE x = $Person.id",
		[]any{Person{}},
		"SELECT p FROM t WHERE x = @[a-zA-Z_0-9]+",
	}, {
		"INSERT INTO person (*) VALUES ($Person.*)",
		[]any{Person{}},
		`INSERT INTO person \(address_id, id, name\) VALUES \(@[a-zA-Z_0-9]+, @[a-zA-Z_0-9]+, @[a-zA-Z_0-9]+\)`,
	}, {
		"INSERT INTO person (name, id) VALUES ($Person.*)",
		[]any{Person{}},
		`INSERT INTO person \(name, id\) VALUES \(@[a-zA-Z_0-9]+, @[a-zA-Z_0-9]+\)`,
	}, {
		"INSERT INTO person (name, postalcode) VALUES ($Person.name, $Address.id)",
		[]any{Person{}, Address{}},
		`INSERT INTO person \(name, postalcode\) VALUES \(@[a-zA-Z_0-9]+, @[a-zA-Z_0-9]+\)`,
	}}
	for _, test := range testList {
		parser := expr.NewParser()
		parsedExpr, _ := parser.Parse(test.input)
		preparedExpr, err := parsedExpr.Prepare(test.prepareArgs...)

		if err != nil {
			c.Fatal(err)
		}
		c.Assert(preparedExpr.SQL, Matches, test.expectedPrepared)
	}
}

func (s *ExprSuite) TestMismatchedInputStructName(c *C) {
	sql := "SELECT street FROM t WHERE x = $Address.street"
	parser := expr.NewParser()
	parsedExpr, err := parser.Parse(sql)
	_, err = parsedExpr.Prepare(Person{ID: 1}, Manager{})
	c.Assert(err, ErrorMatches, `cannot prepare expression: type Address unknown, have: Manager, Person`)
}

func (s *ExprSuite) TestMissingTagInput(c *C) {
	sql := "SELECT street FROM t WHERE x = $Address.number"
	parser := expr.NewParser()
	parsedExpr, err := parser.Parse(sql)
	_, err = parsedExpr.Prepare(Address{ID: 1})
	c.Assert(err, ErrorMatches, `cannot prepare expression: type Address has no "number" db tag`)
}

func (s *ExprSuite) TestMismatchedColNum(c *C) {
	sql := "INSERT INTO person (postalcode) VALUES ($Person.name, $Address.id)"
	parser := expr.NewParser()
	parsedExpr, err := parser.Parse(sql)
	_, err = parsedExpr.Prepare(Address{ID: 1}, Person{Fullname: "jim"})
	c.Assert(err, ErrorMatches, `cannot prepare expression: mismatched number of inputs and cols in input expression: .*`)
}

func (s *ExprSuite) TestPrepareInvalidAsteriskPlacement(c *C) {
	testList := []struct {
		sql     string
		structs []any
	}{{
		sql:     "INSERT INTO person (*, postalcode) VALUES ($Person.name, $Address.id)",
		structs: []any{Address{}, Person{}},
	}, {
		sql:     "INSERT INTO person (name, postalcode) VALUES ($Person.*, $Address.*)",
		structs: []any{Address{}, Person{}},
	}, {
		sql:     "INSERT INTO person (*) VALUES ($Person.id)",
		structs: []any{Person{}},
	}, {
		sql:     "INSERT INTO person (name, postalcode) VALUES ($Person.*, $Address.*)",
		structs: []any{Address{}, Person{}},
	}, {
		sql:     "SELECT street FROM t WHERE x = $Address.*",
		structs: []any{Address{}},
	}}

	for i, test := range testList {
		parser := expr.NewParser()
		parsedExpr, err := parser.Parse(test.sql)
		if err != nil {
			c.Fatal(err)
		}
		_, err = parsedExpr.Prepare(test.structs...)
		c.Assert(err, ErrorMatches, "cannot prepare expression: invalid asterisk in input expression: .*",
			Commentf("test %d failed:\nsql: '%s'\nstructs:'%+v'", i, test.sql, test.structs))
	}
}

func (s *ExprSuite) TestValidComplete(c *C) {
	testList := []struct {
		input          string
		prepareArgs    []any
		completeArgs   []any
		completeValues []any
	}{{
		"SELECT * AS &Address.* FROM t WHERE x = $Person.name",
		[]any{Address{}, Person{}},
		[]any{Person{Fullname: "Jimany Johnson"}},
		[]any{"Jimany Johnson"},
	}, {
		"SELECT foo FROM t WHERE x = $Address.street, y = $Person.id",
		[]any{Person{}, Address{}},
		[]any{Person{ID: 666}, Address{Street: "Highway to Hell"}},
		[]any{"Highway to Hell", 666},
	}}
	for _, test := range testList {
		parser := expr.NewParser()
		parsedExpr, err := parser.Parse(test.input)
		if err != nil {
			c.Fatal(err)
		}

		preparedExpr, err := parsedExpr.Prepare(test.prepareArgs...)
		if err != nil {
			c.Fatal(err)
		}

		inputVals, err := preparedExpr.Complete(test.completeArgs...)
		if err != nil {
			c.Fatal(err)
		}

		c.Assert(inputVals, DeepEquals, test.completeValues)
	}
}

func (s *ExprSuite) TestCompleteMissingParameter(c *C) {
	sql := "SELECT street FROM t WHERE x = $Address.street, y = $Person.name"
	parser := expr.NewParser()
	parsedExpr, err := parser.Parse(sql)
	if err != nil {
		c.Fatal(err)
	}

	preparedExpr, err := parsedExpr.Prepare(Address{}, Person{})
	if err != nil {
		c.Fatal(err)
	}
	_, err = preparedExpr.Complete(Address{Street: "Dead end road"})
	c.Assert(err, ErrorMatches, `type Person not passed as a parameter`)
}

func (s *ExprSuite) TestCompleteNilType(c *C) {
	sql := "SELECT street FROM t WHERE x = $Address.street, y = $Person.name"
	parser := expr.NewParser()
	parsedExpr, err := parser.Parse(sql)
	if err != nil {
		c.Fatal(err)
	}
	preparedExpr, err := parsedExpr.Prepare(Address{}, Person{})
	if err != nil {
		c.Fatal(err)
	}
	_, err = preparedExpr.Complete(nil, Person{Fullname: "Monty Bingles"})
	c.Assert(err, ErrorMatches, "nil parameter")
}

func (s *ExprSuite) TestCompleteDifferentType(c *C) {
	sql := "SELECT street FROM t WHERE y = $Person.name"
	outerP := Person{}
	//type Person struct{}
	type Person struct {
		ID         int    `db:"id"`
		Fullname   string `db:"name"`
		PostalCode int    `db:"address_id"`
	}
	shadowedP := Person{}
	parser := expr.NewParser()
	parsedExpr, err := parser.Parse(sql)
	if err != nil {
		c.Fatal(err)
	}
	preparedExpr, err := parsedExpr.Prepare(outerP)
	if err != nil {
		c.Fatal(err)
	}
	_, err = preparedExpr.Complete(shadowedP)
	c.Assert(err, ErrorMatches, `type Person not passed as a parameter`)
}
