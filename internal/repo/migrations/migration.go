package migrations

import (
	"context"

	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type Migration interface {
	Up(ctx context.Context, tr fdbclient.Transaction) error
	Down(ctx context.Context, tr fdbclient.Transaction) error
	Version() uint64
}

func Migrations(version uint64) map[uint64]Migration {
	migrations := []Migration{
		&Init{},
	}

	result := map[uint64]Migration{}
	for i, m := range migrations {
		v := m.Version()
		if uint64(i) != v {
			panic("first migration must have version 0")
		}

		if v > version {
			result[m.Version()] = m
		}
	}

	return result
}
