package migrations

import (
	"context"

	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
	"github.com/redis/go-redis/v9"
)

type Migration interface {
	Up(ctx context.Context, ftr fdbclient.Transaction, rtr redis.Pipeliner) error
	Down(ctx context.Context, ftr fdbclient.Transaction, rtr redis.Pipeliner) error
	Version() uint32
}

func Migrations(version uint32) []Migration {
	migrations := []Migration{
		&Accounts{},
		&Cookies{},
	}

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
