package fdbclient

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
)

type Database interface {
	NewTransaction(ctx context.Context) (Transaction, error)
	Clear(ctx context.Context, key []byte) error
}

type database struct {
	db fdb.Database
}

func (d *database) Clear(ctx context.Context, key []byte) error {
	tr, err := d.NewTransaction(ctx)
	if err != nil {
		return err
	}

	tr.Clear(key)

	return tr.Commit()
}

func (d *database) NewTransaction(ctx context.Context) (Transaction, error) {
	return NewTransaction(ctx, d.db)
}

func NewDatabase(db fdb.Database) Database {
	return &database{db: db}
}
