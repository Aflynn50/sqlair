package complete_test

import (
	"fmt"
	"testing"

	"github.com/canonical/sqlair/internal/assemble"
	"github.com/canonical/sqlair/internal/complete"
	"github.com/canonical/sqlair/internal/parse"
	"github.com/stretchr/testify/assert"
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

func TestWrongNumberOfArgs(t *testing.T) {
	sql := "select street from t where x = $Address.street, y = $Person.name"
	parser := parse.NewParser()
	parsedExpr, err := parser.Parse(sql)
	assembledExpr, err := assemble.Assemble(parsedExpr, Address{}, Person{})
	_, err = complete.Complete(assembledExpr, Address{Street: "Dead end street"})
	assert.Equal(t, fmt.Errorf("type Person not passed as a parameter"), err)
}
