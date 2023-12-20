package migrations

import (
	"context"

	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type Migration interface {
	Up(ctx context.Context, tr fdbclient.Transaction) error
	Down(ctx context.Context, tr fdbclient.Transaction) error
	Version() uint32
}

func Migrations(version uint32) []Migration {
	migrations := []Migration{
		&Init{},
	}

	result := make([]Migration, 0, len(migrations))

	for i, m := range migrations {
		v := m.Version()
		if uint32(i) != v {
			panic("migration version must be equal to its index")
		}

		if v > version {
			result = append(result, m)
		}
	}

	return result
}
