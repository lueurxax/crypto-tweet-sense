package fdb

import (
	"context"
	"encoding/binary"
)

type version interface {
	GetVersion(ctx context.Context) (uint32, error)
	WriteVersion(ctx context.Context, version uint32) error
}

func (d *db) WriteVersion(ctx context.Context, version uint32) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	data := make([]byte, binary.Size(version))
	binary.BigEndian.PutUint32(data, version)

	if err = tr.Set(d.keyBuilder.Version(), data); err != nil {
		return err
	}

	return tr.Commit()
}

func (d *db) GetVersion(ctx context.Context) (uint32, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return 0, err
	}

	data, err := tr.Get(d.keyBuilder.Version())
	if err != nil {
		return 0, err
	}

	if data == nil {
		return 0, nil
	}

	return binary.BigEndian.Uint32(data), nil
}
