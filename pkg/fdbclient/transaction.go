package fdbclient

import (
	"context"
	"errors"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
)

type Transaction interface {
	Get(key []byte) (value []byte, err error)
	Set(key []byte, value []byte) (err error)
	Clear(key []byte)
	Commit() (err error)
	GetRange(pr fdb.KeyRange, opts ...*RangeOptions) ([]fdb.KeyValue, error)
	GetIterator(pr fdb.KeyRange, opts ...*RangeOptions) *fdb.RangeIterator
}

type transaction struct {
	ctx   context.Context
	tr    fdb.Transaction
	calls []func()
}

func (t *transaction) GetRange(pr fdb.KeyRange, opts ...*RangeOptions) ([]fdb.KeyValue, error) {
	options := SplitRangeOptions(opts)
	return t.tr.GetRange(pr, options).GetSliceWithError()
}

func (t *transaction) GetIterator(pr fdb.KeyRange, opts ...*RangeOptions) *fdb.RangeIterator {
	options := SplitRangeOptions(opts)
	return t.tr.GetRange(pr, options).Iterator()
}

func (t *transaction) Clear(key []byte) {
	t.calls = append(t.calls, func() {
		t.tr.Clear(fdb.Key(key))
	})
}

func (t *transaction) Get(key []byte) ([]byte, error) {
	return t.tr.Get(fdb.Key(key)).Get()
}

func (t *transaction) Set(key []byte, value []byte) (err error) {
	t.calls = append(t.calls, func() {
		t.tr.Set(fdb.Key(key), value)
	})

	return
}

func (t *transaction) Commit() (err error) {
	wrapped := func() {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(fdb.Error)
				if ok {
					err = e
				} else {
					panic(r)
				}
			}
		}()

		for _, call := range t.calls {
			call()
		}

		err = t.tr.Commit().Get()
	}

	for {
		wrapped()

		select {
		case <-t.ctx.Done():
			return err
		default:
		}

		if err == nil {
			return
		}

		var fe fdb.Error
		if errors.As(err, &fe) {
			err = t.tr.OnError(fe).Get()
		}

		if err != nil {
			return
		}
	}
}

func NewTransaction(ctx context.Context, db fdb.Database) (Transaction, error) {
	tr, err := db.CreateTransaction()
	if err != nil {
		return nil, err
	}

	return &transaction{ctx: ctx, tr: tr}, nil
}
