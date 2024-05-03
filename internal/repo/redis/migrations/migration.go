package migrations

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Migration interface {
	Up(ctx context.Context, tr redis.Pipeliner) error
	Down(ctx context.Context, tr redis.Pipeliner) error
	Version() uint32
}

func Migrations(version uint32) []Migration {
	migrations := []Migration{}

	result := make([]Migration, 0, len(migrations))

	for i, m := range migrations {
		v := m.Version()
		if uint32(i+1) != v {
			panic("migration version must be equal to its index")
		}

		if v > version {
			result = append(result, m)
		}
	}

	return result
}
