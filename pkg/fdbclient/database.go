package fdbclient

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
)

type Database interface {
	NewTransaction(ctx context.Context) (Transaction, error)
}

type database struct {
	db fdb.Database
}

func (d *database) NewTransaction(ctx context.Context) (Transaction, error) {
	return NewTransaction(ctx, d.db)
}

func NewDatabase(db fdb.Database) Database {
	return &database{db: db}
}
