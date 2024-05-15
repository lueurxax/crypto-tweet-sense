package redis

import (
	"context"
)

func (d *db) LoadSession(ctx context.Context) ([]byte, error) {
	data, err := d.db.Get(ctx, string(d.keyBuilder.TelegramSessionStorage())).Result()
	if err != nil {
		return nil, err
	}

	return []byte(data), nil
}

func (d *db) StoreSession(ctx context.Context, data []byte) error {
	return d.db.Set(ctx, string(d.keyBuilder.TelegramSessionStorage()), string(data), 0).Err()
}
