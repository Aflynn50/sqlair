// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"database/sql"

	"github.com/canonical/sqlair"
)

type DQLiteDBModelProvider struct{}

type DQLiteTableModelProvider struct {
	db *sqlair.DB
}

func NewDQLiteDBModelProvider() ModelProvider {
	return &DQLiteDBModelProvider{}
}

func NewDQLiteTableModelProvider() ModelProvider {
	return &DQLiteTableModelProvider{}
}

func (d *DQLiteDBModelProvider) Init() error {
	return nil
}

func (d *DQLiteTableModelProvider) Init() error {
	sqldb, err := sql.Open("sqlite3", "file:test.db?cache=shared&mode=memory")
	if err != nil {
		return err
	}

	db := sqlair.NewDB(sqldb)

	tx, err := db.Begin(nil, nil)
	if err != nil {
		return err
	}
	for _, schema := range schemas {
		if err := tx.Query(nil, sqlair.MustPrepare(schema)).Run(); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	d.db = db
	return tx.Commit()
}

func (d *DQLiteDBModelProvider) NewModel(name string) (Model, error) {
	sqldb, err := sql.Open("sqlite3", "file:"+name+".db?cache=shared&mode=memory")
	if err != nil {
		return Model{}, err
	}

	db := sqlair.NewDB(sqldb)

	tx, err := db.Begin(nil, nil)
	if err != nil {
		return Model{}, err
	}

	for _, schema := range schemas {
		if err := tx.Query(nil, sqlair.MustPrepare(schema)).Run(); err != nil {
			_ = tx.Rollback()
			return Model{}, err
		}
	}

	return Model{
		DB:                  db,
		Name:                name,
		ModelTableName:      "agent",
		ModelEventTableName: "agent_events",
		TxRunner:            transactionRunner(db),
	}, tx.Commit()
}

func (d *DQLiteTableModelProvider) NewModel(name string) (Model, error) {
	return Model{
		DB:                  d.db,
		Name:                name,
		ModelTableName:      "agent",
		ModelEventTableName: "agent_events",
		TxRunner:            transactionRunner(d.db),
	}, nil
}
