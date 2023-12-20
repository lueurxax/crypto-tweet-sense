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
