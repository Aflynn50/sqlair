package sqlair_test

import (
	"reflect"

	_ "github.com/mattn/go-sqlite3"

	"github.com/canonical/sqlair/internal/expr"

	"github.com/canonical/sqlair"
	. "gopkg.in/check.v1"
)

func (s *PackageSuite) TestDecodeMtype(c *C) {
	var tests = []struct {
		summary  string
		query    string
		types    []any
		inputs   []any
		outputs  [][]any
		expected [][]any
	}{{
		summary:  "double select with name clash",
		query:    "SELECT p.id AS &Person.*, a.id AS &M.id FROM person AS p, address AS a",
		types:    []any{Person{}},
		inputs:   []any{},
		outputs:  [][]any{{&Person{}, &sqlair.M{}}},
		expected: [][]any{{&Person{ID: 30}, &sqlair.M{"id": int64(25)}}},
	}, {
		summary:  "select multiple with extras",
		query:    "SELECT name, * AS (&Person.*, &Address.id, &Manager.*), id FROM person WHERE id = $M.id",
		types:    []any{Person{}, Address{}, Manager{}},
		inputs:   []any{sqlair.M{"id": 30}},
		outputs:  [][]any{{&Person{}, &Address{}, &Manager{}}},
		expected: [][]any{{&Person{30, "Fred", "1000"}, &Address{ID: 30}, &Manager{30, "Fred", "1000"}}},
	}, {
		summary:  "select with renaming",
		query:    "SELECT (name, postcode) AS (&Address.street, &M.district) FROM person WHERE id = $Manager.id",
		types:    []any{Address{}, Manager{}},
		inputs:   []any{Manager{ID: 30}},
		outputs:  [][]any{{&Address{}, &sqlair.M{}}},
		expected: [][]any{{&Address{Street: "Fred"}, &sqlair.M{"district": "1000"}}},
	}, {
		summary:  "select into star struct",
		query:    "SELECT (name, postcode) AS &M.* FROM person WHERE postcode IN ($Manager.postcode, $Address.district)",
		types:    []any{Person{}, Address{}, Manager{}},
		inputs:   []any{Manager{PostalCode: "1000"}, Address{District: "2000"}},
		outputs:  [][]any{{&sqlair.M{}}},
		expected: [][]any{{&sqlair.M{"name": "Fred", "postcode": "1000"}}},
	},
	}

	drop, db, err := sqlairPersonAndAddressDB()
	if err != nil {
		c.Fatal(err)
	}

	sqlairDB := sqlair.NewDB(db)

	for _, t := range tests {
		stmt, err := sqlair.Prepare(t.query, t.types...)
		if err != nil {
			c.Error(err)
			continue
		}
		q, err := sqlairDB.Query(stmt, t.inputs...)
		if err != nil {
			c.Error(err)
			continue
		}
		for i, os := range t.outputs {
			ok, err := q.Next()
			if err != nil {
				c.Fatal(err)
			} else if !ok {
				c.Fatal("no more rows in query")
			}
			err = q.Decode(os...)
			if err != nil {
				c.Fatal(err)
			}
			for j, o := range os {
				c.Assert(o, DeepEquals, t.expected[i][j])
			}
		}
		q.Close()
	}

	_, err = db.Exec(drop)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *PackageSuite) TestAllMtype(c *C) {
	var tests = []struct {
		summary  string
		query    string
		types    []any
		inputs   []any
		expected [][]any
	}{{
		summary:  "double select with name clash",
		query:    "SELECT p.id AS &Person.*, a.id AS &M.* FROM person AS p, address AS a",
		types:    []any{Person{}},
		inputs:   []any{},
		expected: [][]any{{Person{ID: int64(30)}, sqlair.M{"id": int64(25)}}, {Person{ID: int64(30)}, sqlair.M{"id": int64(30)}}, {Person{ID: int64(30)}, sqlair.M{"id": int64(10)}}, {Person{ID: int64(20)}, sqlair.M{"id": int64(25)}}},
	}, {
		summary:  "simple select person",
		query:    "SELECT (id, name) AS &M.* FROM person WHERE name = 'Fred'",
		types:    []any{},
		inputs:   []any{},
		expected: [][]any{{sqlair.M{"id": int64(30), "name": "Fred"}}},
	}, {
		summary:  "simple select people",
		query:    "SELECT (id, name) AS &M.* FROM person WHERE postcode = '1000'",
		types:    []any{},
		inputs:   []any{},
		expected: [][]any{{sqlair.M{"id": int64(30), "name": "Fred"}}, {sqlair.M{"id": int64(32), "name": "Sam"}}},
	},
	}

	drop, db, err := personAndAddressDB()
	if err != nil {
		c.Fatal(err)
	}

	sqlairDB := sqlair.NewDB(db)

	for _, t := range tests {
		stmt, err := sqlair.Prepare(t.query, t.types...)
		if err != nil {
			c.Error(err)
			continue
		}
		q, err := sqlairDB.Query(stmt)
		if err != nil {
			c.Error(err)
			continue
		}
		res, err := q.All()
		if err != nil {
			c.Error(err)
			continue
		}

		for i, es := range t.expected {
			for j, e := range es {
				// Process M-type: validity check and clone.
				if IsValidMType(reflect.TypeOf(res[i][j])) {
					tm, ok := res[i][j].(expr.M)
					if !ok {
						c.Errorf("invalid map type")
					}
					res[i][j] = CloneM(tm)
				}

				c.Assert(res[i][j], DeepEquals, e)
			}
		}
	}

	_, err = db.Exec(drop)
	if err != nil {
		c.Fatal(err)
	}
}

// IsValidMapType takes a reflect type and checks whether it is a map, the type name is M, and the key type of the map is string.
func IsValidMType(mt reflect.Type) bool {
	var s string
	return mt.Kind() == reflect.Map && mt.Name() == reflect.TypeOf(sqlair.M{}).Name() && mt.Key() == reflect.TypeOf(s)
}

// CloneM copies the contents of an input expr.M type map into an sqlair_test.M type map, preparing it for DeepEqual asserting check.
func CloneM(m expr.M) sqlair.M {
	tm := sqlair.M{}
	for k, v := range m {
		tm[k] = v
	}
	return tm
}
