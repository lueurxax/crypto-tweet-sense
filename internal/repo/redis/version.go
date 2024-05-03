package redis

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/redis/go-redis/v9"
)

type version interface {
	GetVersion(ctx context.Context) (uint32, error)
	WriteVersion(ctx context.Context, version uint32) error
}

func (d *db) WriteVersion(ctx context.Context, version uint32) error {
	data := make([]byte, binary.Size(version))
	binary.BigEndian.PutUint32(data, version)

	return d.db.Set(ctx, string(d.keyBuilder.Version()), string(data), 0).Err()
}

func (d *db) GetVersion(ctx context.Context) (uint32, error) {
	data, err := d.db.Get(ctx, string(d.keyBuilder.Version())).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}

	return binary.BigEndian.Uint32([]byte(data)), nil
}
