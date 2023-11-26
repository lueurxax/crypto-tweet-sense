package fdb

import "context"

func (d *db) LoadSession(ctx context.Context) ([]byte, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	data, err := tx.Get(d.keyBuilder.TelegramSessionStorage())
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, ErrTelegramSessionNotFound
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return data, nil
}

func (d *db) StoreSession(ctx context.Context, data []byte) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.TelegramSessionStorage(), data)

	return tx.Commit()
}
